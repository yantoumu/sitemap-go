#!/bin/bash

# Full sitemap test with 60 sites
export BACKEND_URL="https://api.example.com"
export BACKEND_API_KEY="test-api-key"
export TRENDS_API_URL="https://k2.seokey.vip/api/keywords"
export TRENDS_API_URL_SECONDARY="https://ads.seokey.vip/api/keywords"
export ENCRYPTION_KEY="test-encryption-key-32-characters"
export SITEMAP_WORKERS=30
export API_WORKERS=8
export BATCH_SIZE=8
export DEBUG=false

# All sitemaps to test
SITEMAP_URLS="https://1games.io/sitemap.xml,https://azgames.io/sitemap.xml,https://baldigames.com/sitemap.xml,https://game-game.com/sitemap.xml,https://geometry-free.com/sitemap.xml,https://geometrydash.io/sitemap.xml,https://googledoodlegames.net/sitemap.xml,https://html5.gamedistribution.com/sitemap.xml,https://itch.io/feed/new.xml,https://kiz10.com/sitemap-games.xml,https://kizi.com/sitemaps/kizi/en/sitemap_games.xml.gz,https://lagged.com/sitemap.txt,https://nointernetgame.com/game-sitemap.xml,https://playgama.com/sitemap-2.xml,https://playtropolis.com/sitemap.games.xml,https://pokerogue.io/sitemap.xml,https://poki.com/en/sitemaps/index.xml,https://ssgames.site/sitemap.xml,https://wordle2.io/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.1001jeux.fr/sitemap-games.xml,https://www.freegames.com/sitemap/games_1.xml,https://www.gamearter.com/sitemap,https://www.minigiochi.com/sitemap-games-3.xml,https://www.onlinegames.io/sitemap.xml,https://www.play-games.com/sitemap.xml,https://www.playgame24.com/sitemap.xml,https://www.twoplayergames.org/sitemap-games.xml,https://keygames.com/games-sitemap.xml,https://www.snokido.com/sitemaps/games.xml,https://www.miniplay.com/sitemap-games-3.xml,https://sprunki.org/sitemap.xml,https://geometrygame.org/sitemap.xml,https://kiz10.com/sitemap-games-2.xml,https://sprunkigo.com/en/sitemap.xml,https://sprunki.com/sitemap.xml,https://www.sprunky.org/sitemap.xml,https://www.megaigry.ru/rss/,https://superkidgames.com/sitemap.xml,https://www.gamesgames.com/sitemaps/gamesgames/en/sitemap_games.xml.gz,https://www.spel.nl/sitemaps/agame/nl/sitemap_games.xml.gz,https://www.girlsgogames.it/sitemaps/girlsgogames/it/sitemap_games.xml.gz,https://www.games.co.id/sitemaps/agame/id/sitemap_games.xml.gz,https://www.newgrounds.com/sitemaps/art/sitemap.94.xml,https://www.topigre.net/sitemap.xml,https://geoguessr.io/sitemap.xml,https://startgamer.ru/sitemap.xml,https://doodle-jump.co/sitemap.xml,https://www.hoodamath.com/sitemap.xml,https://www.brightestgames.com/games-sitemap.xml,https://www.hahagames.com/sitemap.xml,https://www.puzzleplayground.com/sitemap.xml,https://www.mathplayground.com/sitemap_main.xml,https://geometrydashworld.net/sitemap.xml,https://zh.y8.com/sitemaps/y8/zh/sitemap.xml.gz,https://geometry-lite.io/sitemap.xml,https://geometrydashsubzero.net/sitemap.xml,https://kizgame.com/sitemap-en.xml,https://wordhurdle.co/sitemap.xml,https://chillguygame.io/sitemap.xml"

echo "🚀 Starting full sitemap test with 60 sites..."
echo "📊 Configuration:"
echo "   - Sitemap workers: $SITEMAP_WORKERS (30 concurrent)"
echo "   - API workers: $API_WORKERS (8 concurrent)"
echo "   - Batch size: $BATCH_SIZE"
echo "   - Primary API: $TRENDS_API_URL"
echo "   - Secondary API: $TRENDS_API_URL_SECONDARY"
echo "   - Dual API: Enabled with load balancing"
echo ""
echo "🔧 Performance optimizations:"
echo "   - Parallel keyword extraction using all CPU cores"
echo "   - Removed 5-second API delays (now 100ms)"
echo "   - True concurrent API execution"
echo "   - No rate limiting on local operations"
echo ""

# Run the sitemap monitor
time ./sitemap-go -sitemaps "$SITEMAP_URLS"