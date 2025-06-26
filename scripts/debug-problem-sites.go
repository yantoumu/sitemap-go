package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func main() {
	fmt.Println("=== Debugging Problem Sites ===\n")

	// Test lagged.com with different approaches
	fmt.Println("1. Testing lagged.com/sitemap.txt")
	testLaggedSitemap()

	fmt.Println("\n2. Testing www.playgame24.com/sitemap.xml")
	testPlaygame24Sitemap()
}

func testLaggedSitemap() {
	url := "https://lagged.com/sitemap.txt"

	// Method 1: Standard HTTP with custom headers
	fmt.Println("Method 1: Standard HTTP request")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/plain,text/html,application/xml,*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("   Status: %s\n", resp.Status)
		fmt.Printf("   Content-Type: %s\n", resp.Header.Get("Content-Type"))
		
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		fmt.Printf("   First 1KB:\n%s\n", string(body))
	}

	// Method 2: FastHTTP with different headers
	fmt.Println("\nMethod 2: FastHTTP request")
	req2 := fasthttp.AcquireRequest()
	resp2 := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req2)
	defer fasthttp.ReleaseResponse(resp2)

	req2.SetRequestURI(url)
	req2.Header.SetMethod(fasthttp.MethodGet)
	req2.Header.SetUserAgent("sitemap-parser/1.0")
	req2.Header.Set("Accept", "*/*")

	client2 := &fasthttp.Client{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	err = client2.Do(req2, resp2)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Status: %d\n", resp2.StatusCode())
		fmt.Printf("   Content-Type: %s\n", string(resp2.Header.Peek("Content-Type")))
		
		body := resp2.Body()
		if len(body) > 1024 {
			body = body[:1024]
		}
		fmt.Printf("   First 1KB:\n%s\n", string(body))
	}

	// Method 3: Try without HTTPS
	fmt.Println("\nMethod 3: HTTP (non-SSL) request")
	httpURL := strings.Replace(url, "https://", "http://", 1)
	resp3, err := http.Get(httpURL)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		defer resp3.Body.Close()
		fmt.Printf("   Status: %s\n", resp3.Status)
	}
}

func testPlaygame24Sitemap() {
	url := "https://www.playgame24.com/sitemap.xml"

	// Method 1: Direct request
	fmt.Println("Method 1: Direct request")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("   Status: %s\n", resp.Status)
	fmt.Printf("   Content-Type: %s\n", resp.Header.Get("Content-Type"))
	fmt.Printf("   Content-Length: %s\n", resp.Header.Get("Content-Length"))

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	fmt.Printf("   First 2KB:\n%s\n", string(body))

	// Check if it's valid XML
	content := string(body)
	if strings.Contains(content, "<?xml") {
		fmt.Println("   ✓ Has XML declaration")
	} else {
		fmt.Println("   ✗ Missing XML declaration")
	}

	if strings.Contains(content, "<urlset") || strings.Contains(content, "<sitemapindex") {
		fmt.Println("   ✓ Has sitemap root element")
	} else {
		fmt.Println("   ✗ Missing sitemap root element")
	}

	// Check for encoding issues
	for i, b := range body {
		if b > 127 && b < 160 {
			fmt.Printf("   ⚠️  Found problematic byte at position %d: 0x%02X\n", i, b)
		}
	}

	// Method 2: Try alternate URLs
	fmt.Println("\nMethod 2: Testing alternate URLs")
	alternates := []string{
		"https://www.playgame24.com/sitemap_index.xml",
		"https://www.playgame24.com/sitemap-index.xml",
		"https://www.playgame24.com/sitemap",
		"https://playgame24.com/sitemap.xml",
	}

	for _, altURL := range alternates {
		resp2, err := http.Head(altURL)
		if err != nil {
			fmt.Printf("   %s - Error: %v\n", altURL, err)
		} else {
			fmt.Printf("   %s - Status: %s\n", altURL, resp2.Status)
			resp2.Body.Close()
		}
	}
}