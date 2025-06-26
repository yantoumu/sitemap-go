#!/bin/bash

# 部署脚本 - 编译并准备生产版本

set -e

echo "🚀 Starting deployment build..."

# 清理旧的构建文件
echo "🧹 Cleaning old builds..."
rm -f sitemap-go

# 编译优化的生产版本
echo "📦 Building production binary..."
go build -ldflags="-s -w" -o sitemap-go main.go

# 检查编译结果
if [ ! -f sitemap-go ]; then
    echo "❌ Build failed!"
    exit 1
fi

# 显示编译信息
echo "✅ Build completed successfully!"
echo "📊 Binary details:"
ls -lh sitemap-go
file sitemap-go

# 设置执行权限
chmod +x sitemap-go

echo ""
echo "🎉 Deployment build ready!"
echo ""
echo "📋 Next steps:"
echo "1. Test locally: ./sitemap-go -help"
echo "2. Commit changes: git add . && git commit -m 'chore: update production build'"
echo "3. Push to GitHub: git push origin main"
echo "4. GitHub Actions will run automatically at:"
echo "   - 中国时间早上 7:00"
echo "   - 中国时间晚上 23:00"
echo ""
echo "⚙️  Required GitHub Secrets:"
echo "   - BACKEND_API_URL"
echo "   - BACKEND_API_KEY"
echo "   - TRENDS_API_URL"
echo "   - SITEMAP_URLS (optional)"