#!/bin/bash

echo "🚀 Starting Sitemap Keyword Deduplication Test"
echo "============================================="
echo ""

# 确保构建最新版本
echo "📦 Building test script..."
go build -o test_deduplication test_deduplication.go

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful!"
echo ""

# 创建测试结果目录
mkdir -p test_results
cd test_results

# 运行测试
echo "🔍 Running keyword deduplication test..."
echo "This may take a few minutes depending on network speed..."
echo ""

../test_deduplication

# 检查结果文件
echo ""
echo "📊 Test Results Summary:"
echo "========================"

if [ -f "unique_keywords.txt" ]; then
    unique_count=$(wc -l < unique_keywords.txt)
    echo "✅ Unique keywords: $unique_count"
else
    echo "❌ unique_keywords.txt not found"
fi

if [ -f "all_keywords.txt" ]; then
    total_count=$(wc -l < all_keywords.txt)
    echo "📝 Total keywords: $total_count"
else
    echo "❌ all_keywords.txt not found"
fi

if [ -f "keyword_analysis.txt" ]; then
    echo "📈 Analysis report: keyword_analysis.txt"
else
    echo "❌ keyword_analysis.txt not found"
fi

if [ -f "failed_sitemaps.txt" ]; then
    failed_count=$(wc -l < failed_sitemaps.txt)
    echo "⚠️  Failed sitemaps: $failed_count"
else
    echo "✅ No failed sitemaps"
fi

echo ""
echo "📁 All results saved in: ./test_results/"
echo "📋 Files generated:"
ls -la *.txt 2>/dev/null | awk '{print "   - " $9 " (" $5 " bytes)"}'

echo ""
echo "🎉 Test completed! Check the generated files for detailed results."