// src/worker.js – Production version with correct request flow
// -----------------------------------------------------------------------------
//  Google Ads Keyword API glue for Cloudflare Workers
// -----------------------------------------------------------------------------

// Global request deduplication map
const pendingRequests = new Map();

/* -------------------------------------------------------------------------- */
/*  Rate Limiter Class                                                        */
/* -------------------------------------------------------------------------- */
class RateLimiter {
    constructor(env, maxConcurrent = 20) {
        this.env = env;
        this.maxConcurrent = maxConcurrent;
    }

    async acquire() {
        const key = 'concurrent_requests';
        let retries = 0;

        while (retries < 50) { // Max 50 retries (~5-10 seconds)
            try {
                const current = parseInt(await this.env.TOKEN_CACHE.get(key) || '0');

                if (current < this.maxConcurrent) {
                    // Double-check pattern to reduce race conditions
                    const beforeWrite = await this.env.TOKEN_CACHE.get(key);
                    const beforeInt = parseInt(beforeWrite || '0');

                    if (beforeInt === current && current < this.maxConcurrent) {
                        await this.env.TOKEN_CACHE.put(key, String(current + 1), {
                            expirationTtl: 60 // 1 minute TTL to prevent leaks
                        });
                        return true;
                    }
                }
            } catch (e) {
                console.error('[RateLimiter acquire error]', e);
            }

            // Wait and retry with jitter
            await new Promise(resolve =>
                setTimeout(resolve, 100 + Math.random() * 100)
            );
            retries++;
        }

        throw new Error('Failed to acquire rate limit after timeout');
    }

    async release() {
        const key = 'concurrent_requests';
        try {
            const current = parseInt(await this.env.TOKEN_CACHE.get(key) || '0');
            if (current > 0) {
                await this.env.TOKEN_CACHE.put(key, String(current - 1), {
                    expirationTtl: 60
                });
            }
        } catch (e) {
            console.error('[RateLimiter release error]', e);
        }
    }
}

/* -------------------------------------------------------------------------- */
/*  Main Worker                                                               */
/* -------------------------------------------------------------------------- */
const worker = {
    async fetch (request, env) {
        const { pathname, searchParams } = new URL(request.url);

        if (!/^\/api\/keywords\/?$/i.test(pathname)) {
            return json({ error: 'Use /api/keywords' }, 404);
        }

        /* ---- 1. Parameter validation ---------------------------------------- */
        const keywords = splitCsv(searchParams.get('keyword'));
        if (!keywords.length)          return json({ error: 'keyword param required' }, 400);
        if (keywords.length > 10)      return json({ error: 'max 10 keywords' }, 400);

        const seedUrl = searchParams.get('url') || undefined;
        const geo     = (searchParams.get('geo') || 'GLOBAL').toUpperCase();
        if (geo !== 'GLOBAL')          return json({ error: `unsupported geo ${geo}` }, 400);

        const cid     = searchParams.get('customerId') || '1961763003';
        const refresh = searchParams.get('refresh') === 'true';
        const limit   = Math.min(parseInt(searchParams.get('limit') || '250', 10), 10_000);

        /* ---- 2. Build cache key --------------------------------------------- */
        const cacheKey = buildCacheKey({ cid, keywords, seedUrl, geo });

        /* ---- 3. Check cache first (NO rate limiting) ------------------------ */
        if (!refresh) {
            try {
                const cached = await env.RESULTS_CACHE.get(cacheKey, { type: 'json' });
                if (cached) {
                    debug(env, 'RESULTS_CACHE hit - returning immediately');
                    return json(cached);
                }
            } catch (e) {
                console.error('[Cache read error]', e);
                // Continue to API call if cache read fails
            }
        } else {
            debug(env, 'refresh=true, skipping cache');
        }

        /* ---- 4. Check for pending identical requests (NO rate limiting) ----- */
        if (pendingRequests.has(cacheKey)) {
            debug(env, 'Identical request in progress - waiting without acquiring rate limit');
            try {
                const result = await pendingRequests.get(cacheKey);
                return json(result);
            } catch (e) {
                console.error('[Pending request error]', e);
                // If the pending request failed, we'll try again
            }
        }

        /* ---- 5. Need new API call - NOW we need rate limiting --------------- */
        const limiter = new RateLimiter(env, parseInt(env.MAX_CONCURRENT || '20'));
        let rateLimitAcquired = false;

        try {
            // Create the API call promise BEFORE acquiring rate limit
            const apiCallPromise = (async () => {
                // Acquire rate limit for actual API call
                await limiter.acquire();
                rateLimitAcquired = true;
                debug(env, 'Rate limit acquired for new API call');

                try {
                    const token = await getAccessToken(env);
                    const api = await fetchKeywordIdeas({
                        env, token, cid, keywords, seedUrl, pageSize: limit
                    });

                    const data = transform(api, keywords);

                    const payload = {
                        status: 'success',
                        geo_target: geo,
                        total_results: data.length,
                        data
                    };

                    // Cache the results
                    await maybeCacheResults(env, cacheKey, payload);

                    return payload;
                } finally {
                    // Release rate limit immediately after API call
                    if (rateLimitAcquired) {
                        await limiter.release();
                        rateLimitAcquired = false;
                        debug(env, 'Rate limit released after API call');
                    }
                }
            })();

            // Register this promise for deduplication
            pendingRequests.set(cacheKey, apiCallPromise);

            // Clean up after completion
            apiCallPromise.finally(() => {
                setTimeout(() => {
                    pendingRequests.delete(cacheKey);
                    debug(env, 'Cleaned up pending request');
                }, 100);
            }).catch(() => {}); // Prevent unhandled rejection

            // Wait for the result
            const result = await apiCallPromise;
            return json(result);

        } catch (e) {
            console.error('[Worker error]', e);

            // Ensure rate limit is released on error
            if (rateLimitAcquired) {
                await limiter.release();
            }

            // Return appropriate error response
            if (e.message && e.message.includes('rate limit')) {
                return json({
                    error: 'Service temporarily unavailable - too many concurrent requests',
                    retry_after: 5
                }, 503);
            }

            return json({ error: `${e.name || 'Error'}: ${e.message || 'Unknown error'}` }, 500);
        }
    }
};

/* -------------------------------------------------------------------------- */
/*  OAuth helpers with distributed lock                                       */
/* -------------------------------------------------------------------------- */
async function getAccessToken (env) {
    // Check cached token
    try {
        const hit = await env.TOKEN_CACHE.get('google_token', { type: 'json' });
        if (hit && hit.expires > Date.now()) {
            return hit.token;
        }
    } catch (e) {
        console.error('[Token cache read error]', e);
    }

    // Need to refresh token - use distributed lock
    const lockKey = 'token_refresh_lock';
    const lockValue = crypto.randomUUID();
    const maxRetries = 50;

    for (let retry = 0; retry < maxRetries; retry++) {
        // Try to acquire lock
        const lockAcquired = await tryAcquireLock(env, lockKey, lockValue, 5);

        if (lockAcquired) {
            try {
                // Double-check token after acquiring lock
                const recheck = await env.TOKEN_CACHE.get('google_token', { type: 'json' });
                if (recheck && recheck.expires > Date.now()) {
                    return recheck.token;
                }

                // Perform token refresh
                debug(env, 'Refreshing Google OAuth token');
                return await refreshToken(env);
            } finally {
                // Always release lock
                await releaseLock(env, lockKey, lockValue);
            }
        }

        // Lock not acquired, wait and retry
        await new Promise(resolve =>
            setTimeout(resolve, 100 + Math.random() * 100)
        );

        // Check if token was refreshed by another request
        try {
            const checkAgain = await env.TOKEN_CACHE.get('google_token', { type: 'json' });
            if (checkAgain && checkAgain.expires > Date.now()) {
                return checkAgain.token;
            }
        } catch (e) {
            console.error('[Token recheck error]', e);
        }
    }

    throw new Error('Failed to acquire token refresh lock');
}

async function tryAcquireLock(env, key, value, ttl) {
    try {
        const existing = await env.TOKEN_CACHE.get(key);
        if (existing) return false;

        await env.TOKEN_CACHE.put(key, value, {
            expirationTtl: ttl
        });

        // Verify lock was acquired (handle race condition)
        const verify = await env.TOKEN_CACHE.get(key);
        return verify === value;
    } catch (e) {
        console.error('[tryAcquireLock error]', e);
        return false;
    }
}

async function releaseLock(env, key, value) {
    try {
        const current = await env.TOKEN_CACHE.get(key);
        if (current === value) {
            await env.TOKEN_CACHE.delete(key);
        }
    } catch (e) {
        console.error('[releaseLock error]', e);
    }
}

async function refreshToken (env) {
    const r = await fetch('https://oauth2.googleapis.com/token', {
        method : 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body   : new URLSearchParams({
            client_id    : env.GOOGLE_CLIENT_ID,
            client_secret: env.GOOGLE_CLIENT_SECRET,
            refresh_token: env.GOOGLE_REFRESH_TOKEN,
            grant_type   : 'refresh_token'
        })
    });

    if (!r.ok) {
        const detail = await r.text().catch(() => '');
        throw new Error(`OAuth ${r.status} ${detail}`);
    }

    const { access_token: token, expires_in } = await r.json();
    const ttl = expires_in - 60; // Refresh 1 minute before expiry

    await env.TOKEN_CACHE.put(
        'google_token',
        JSON.stringify({ token, expires: Date.now() + ttl * 1000 }),
        { expirationTtl: ttl }
    );

    debug(env, 'Google OAuth token refreshed successfully');
    return token;
}

/* -------------------------------------------------------------------------- */
/*  Google Ads API with retry logic                                           */
/* -------------------------------------------------------------------------- */
async function fetchKeywordIdeas ({ env, token, cid, keywords, seedUrl, pageSize }) {
    const url  = `https://googleads.googleapis.com/v18/customers/${cid}:generateKeywordIdeas`;
    const body = {
        language            : 'languageConstants/1000',
        keyword_plan_network: 'GOOGLE_SEARCH',
        page_size           : pageSize
    };

    if (keywords.length && seedUrl)      body.keyword_and_url_seed = { keywords, url: seedUrl };
    else if (keywords.length)            body.keyword_seed         = { keywords };
    else if (seedUrl)                    body.url_seed             = { url: seedUrl };

    const maxRetries = 3;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
        try {
            const r = await fetch(url, {
                method : 'POST',
                headers: {
                    Authorization       : `Bearer ${token}`,
                    'developer-token'   : env.GOOGLE_DEVELOPER_TOKEN,
                    'login-customer-id' : env.GOOGLE_LOGIN_CUSTOMER_ID,
                    'Content-Type'      : 'application/json'
                },
                body: JSON.stringify(body)
            });

            // Handle specific status codes
            if (r.status === 409) {
                // 409 Conflict - wait with exponential backoff
                const delay = Math.min(1000 * Math.pow(2, attempt), 5000);
                console.warn(`[Google Ads API] Got 409, retrying after ${delay}ms (attempt ${attempt + 1}/${maxRetries})`);
                await new Promise(resolve => setTimeout(resolve, delay));
                continue;
            }

            if (r.status === 429) {
                // Rate limit - wait longer
                const retryAfter = r.headers.get('Retry-After');
                const delay = retryAfter ? parseInt(retryAfter) * 1000 : 5000;
                console.warn(`[Google Ads API] Got 429, retrying after ${delay}ms`);
                await new Promise(resolve => setTimeout(resolve, delay));
                continue;
            }

            if (!r.ok) {
                const errorText = await r.text().catch(() => '');
                throw new Error(`Ads API ${r.status}: ${errorText}`);
            }

            return await r.json();

        } catch (e) {
            if (attempt === maxRetries - 1) throw e;

            // Network error - retry with backoff
            const delay = Math.min(1000 * Math.pow(2, attempt), 5000);
            console.error(`[Google Ads API] Network error, retrying after ${delay}ms:`, e.message);
            await new Promise(resolve => setTimeout(resolve, delay));
        }
    }

    throw new Error(`Failed after ${maxRetries} attempts`);
}

/* -------------------------------------------------------------------------- */
/*  KV write with atomic counter                                              */
/* -------------------------------------------------------------------------- */
async function maybeCacheResults (env, key, payload) {
    const bytes = new TextEncoder().encode(JSON.stringify(payload)).length;
    if (bytes > 10 * 1024) {
        debug(env, 'skip cache – object too big');
        return;
    }

    const cap = parseInt(env.KV_WRITE_CAP || '500000', 10);
    const day = new Date().toISOString().slice(0,10).replace(/-/g,'');
    const counterKey = `__wc__${day}`;

    // Use atomic increment
    try {
        const count = await atomicIncrement(env, counterKey, cap);
        if (count === null) {
            debug(env, 'skip cache – daily cap reached');
            return;
        }

        // Write to cache
        await env.RESULTS_CACHE.put(key, JSON.stringify(payload), {
            expirationTtl: 60*60*12 // 12 hours
        });

        debug(env, `Cached results successfully (daily count: ${count})`);
    } catch (e) {
        console.error('[Cache write error]', e);
        // Don't throw - caching is not critical
    }
}

async function atomicIncrement(env, key, maxValue) {
    const maxRetries = 5;

    for (let i = 0; i < maxRetries; i++) {
        try {
            const current = await env.RESULTS_CACHE.get(key);
            const currentInt = parseInt(current || '0');

            if (currentInt >= maxValue) {
                return null; // Cap reached
            }

            // Double-check right before write
            await new Promise(resolve => setTimeout(resolve, Math.random() * 10));
            const checkAgain = await env.RESULTS_CACHE.get(key);
            const checkInt = parseInt(checkAgain || '0');

            if (checkInt !== currentInt) {
                continue; // Value changed, retry
            }

            await env.RESULTS_CACHE.put(
                key,
                String(currentInt + 1),
                { expirationTtl: 60*60*24 } // 24 hours
            );

            return currentInt + 1;
        } catch (e) {
            if (i === maxRetries - 1) {
                console.error('[atomicIncrement error]', e);
                throw e;
            }

            // Exponential backoff
            await new Promise(resolve =>
                setTimeout(resolve, Math.min(100 * Math.pow(2, i), 1000))
            );
        }
    }

    throw new Error('Failed to increment counter');
}

/* -------------------------------------------------------------------------- */
/*  Transform API response                                                    */
/* -------------------------------------------------------------------------- */
function transform (api, filter) {
    const want = new Set(filter.map(k => k.toLowerCase())); // 精确保留输入词

    const monthMap = {
        JANUARY:1, FEBRUARY:2, MARCH:3, APRIL:4, MAY:5, JUNE:6,
        JULY:7, AUGUST:8, SEPTEMBER:9, OCTOBER:10, NOVEMBER:11, DECEMBER:12
    };

    return (api.results || [])
        .filter(it => want.has(it.text.toLowerCase()))        // ← 不想过滤就删除本行
        .map(it => {
            const m    = it.keywordIdeaMetrics || {};
            const vols = m.monthlySearchVolumes || [];

            const arr = vols.map(v => ({
                year    : parseInt(v.year) || 0,
                month   : monthMap[v.month] || 0,
                searches: +v.monthlySearches || 0
            }));

            const latest = arr.length ? arr[arr.length - 1].searches : 0;
            const max    = arr.reduce((mx,v) => Math.max(mx,v.searches), 0);
            const quality= analyzeDataQuality(arr);

            return {
                keyword: it.text,
                metrics: {
                    avg_monthly_searches   : +m.avgMonthlySearches    || 0,
                    latest_searches        : latest,
                    max_monthly_searches   : max,
                    competition            : m.competition           ?? 'N/A',
                    competition_index      : m.competitionIndex      ? +m.competitionIndex      : null,
                    low_top_of_page_bid_micro : m.lowTopOfPageBidMicros  ? +m.lowTopOfPageBidMicros  : null,
                    high_top_of_page_bid_micro: m.highTopOfPageBidMicros ? +m.highTopOfPageBidMicros : null,
                    monthly_searches       : arr,
                    data_quality           : quality
                }
            };
        })
        .sort((a,b) => b.metrics.avg_monthly_searches - a.metrics.avg_monthly_searches);
}

/* -------------------------------------------------------------------------- */
function analyzeDataQuality (monthly) {
    if (!monthly.length) {
        return {
            status: 'no_data', complete:false,
            has_missing_months:false, only_last_month_has_data:false,
            total_months:0, available_months:0, missing_months_count:0,
            missing_months:[], warnings:['no_monthly_data']
        };
    }

    const sorted  = [...monthly].sort((a,b)=>a.year!==b.year? a.year-b.year : a.month-b.month);
    const nonZero = sorted.filter(d=>d.searches>0);
    const zeros   = sorted.filter(d=>d.searches===0);
    const onlyLast= nonZero.length===1 && nonZero[0]===sorted[sorted.length-1];

    const warnings=[];
    if (onlyLast) warnings.push('only_last_month_has_data');
    if (zeros.length) warnings.push('has_missing_months');

    return {
        status                : warnings.length?'incomplete':'complete',
        complete              : warnings.length===0,
        has_missing_months    : !!zeros.length,
        only_last_month_has_data: onlyLast,
        total_months          : sorted.length,
        available_months      : nonZero.length,
        missing_months_count  : zeros.length,
        missing_months        : zeros.map(d=>({year:d.year, month:d.month})),
        warnings
    };
}

/* -------------------------------------------------------------------------- */
/*  Utilities                                                                 */
/* -------------------------------------------------------------------------- */
function splitCsv (raw) {
    return raw ? raw.split(',').map(s=>s.trim()).filter(Boolean) : [];
}

function buildCacheKey ({ cid, keywords, seedUrl, geo }) {
    const kwPart = [...keywords].sort().join('|');
    return `${cid}:${geo}:${kwPart}:${seedUrl || ''}`.slice(0, 480);
}

function json (obj, status = 200) {
    return new Response(JSON.stringify(obj), {
        status,
        headers: {
            'Content-Type'             : 'application/json; charset=UTF-8',
            'Access-Control-Allow-Origin': '*',
            'Access-Control-Allow-Methods': 'GET, OPTIONS',
            'Access-Control-Allow-Headers': 'Content-Type',
            'Cache-Control'            : status === 200 ? 'public, max-age=300' : 'no-cache'
        }
    });
}

function debug (env, ...args) {
    if (
        env.DEBUG &&
        (env.DEBUG === true ||
            (typeof env.DEBUG === 'string' && env.DEBUG.toLowerCase() === 'true'))
    ) console.log('[debug]', ...args);
}

/* -------------------------------------------------------------------------- */
export default worker;