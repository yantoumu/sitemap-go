#!/bin/bash

# Generate a secure encryption key for sitemap-go
# This script helps generate a strong encryption key for securing stored data

echo "🔐 Generating secure encryption key for sitemap-go..."
echo ""

# Generate a 32-byte key and encode it in base64
ENCRYPTION_KEY=$(openssl rand -base64 32)

echo "Your encryption key is:"
echo "========================"
echo "$ENCRYPTION_KEY"
echo "========================"
echo ""
echo "📋 To set this in GitHub Secrets:"
echo "1. Go to your repository settings"
echo "2. Navigate to Secrets and variables > Actions"
echo "3. Click 'New repository secret'"
echo "4. Name: ENCRYPTION_KEY"
echo "5. Value: $ENCRYPTION_KEY"
echo ""
echo "⚠️  IMPORTANT: Save this key securely! You'll need it to decrypt stored data."
echo ""
echo "🔧 Required GitHub Secrets for sitemap-go:"
echo "  - BACKEND_API_URL: Your backend API endpoint"
echo "  - BACKEND_API_KEY: Your backend API authentication key"
echo "  - TRENDS_API_URL: Google Trends API endpoints (comma-separated)"
echo "  - ENCRYPTION_KEY: The key generated above"
echo "  - SITEMAP_URLS: (Optional) Comma-separated list of sitemap URLs to monitor"