#!/bin/bash

# GitHub Secrets 设置助手脚本

echo "🔐 GitHub Secrets 配置助手"
echo "=========================="
echo ""

echo "📋 请在GitHub仓库中配置以下Secrets："
echo "   Settings → Secrets and variables → Actions → New repository secret"
echo ""

echo "1️⃣ BACKEND_API_URL"
echo "   值: https://work.seokey.vip/api/v1/keyword-metrics/batch"
echo ""

echo "2️⃣ BACKEND_API_KEY"
echo "   值: sitemap-update-api-key-2025"
echo ""

echo "3️⃣ TRENDS_API_URL"
echo "   值: [请输入实际的Google Trends API地址]"
echo "   示例: https://trends.google.com/trends/api/dailytrends"
echo ""

echo "4️⃣ SITEMAP_URLS (可选 - 如不设置将使用默认列表)"
echo ""
echo "📊 推荐配置选项："
echo ""

echo "🎯 基础配置（适合测试）："
BASIC_URLS="https://poki.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.miniplay.com/sitemap-games-3.xml,https://kiz10.com/sitemap-games.xml"
echo "   $BASIC_URLS"
echo ""

echo "🚀 完整配置（适合生产）："
FULL_URLS="https://poki.com/sitemap.xml,https://www.y8.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.friv.com/sitemap.xml,https://www.silvergames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.freegames.com/sitemap/games_1.xml,https://www.miniplay.com/sitemap-games-3.xml,https://kiz10.com/sitemap-games.xml,https://www.snokido.com/sitemaps/games.xml"
echo "   $FULL_URLS"
echo ""

echo "⚡ 测试配置（开发用）："
TEST_URLS="https://poki.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml"
echo "   $TEST_URLS"
echo ""

echo "🔧 配置完成后的验证步骤："
echo "1. 进入GitHub仓库的Actions页面"
echo "2. 选择'Sitemap Monitor'工作流"
echo "3. 点击'Run workflow'进行手动测试"
echo "4. 查看执行日志确认配置正确"
echo ""

echo "⏰ 自动执行时间："
echo "- 中国时间每天早上 7:00"
echo "- 中国时间每天晚上 23:00"
echo ""

echo "📊 预期结果："
echo "- 处理多个游戏网站的sitemap"
echo "- 提取关键词并去重"
echo "- 查询Google Trends数据"
echo "- 批量提交到后端API"
echo "- 生成详细的执行报告"
echo ""

echo "💡 提示："
echo "- 建议先用测试配置验证系统正常工作"
echo "- 确认无误后再使用完整配置"
echo "- 定期检查GitHub Actions的执行情况"
echo ""

# 生成配置模板文件
echo "📝 生成配置模板..."
cat > github-secrets-template.txt << EOF
# GitHub Secrets 配置模板
# 请复制以下内容到GitHub仓库的Secrets配置中

BACKEND_API_URL=https://work.seokey.vip/api/v1/keyword-metrics/batch

BACKEND_API_KEY=sitemap-update-api-key-2025

TRENDS_API_URL=[请替换为实际的Google Trends API地址]

# 选择一个配置（去掉注释符号#）：

# 基础配置（推荐新用户）：
# SITEMAP_URLS=$BASIC_URLS

# 完整配置（生产环境）：
# SITEMAP_URLS=$FULL_URLS

# 测试配置（开发调试）：
# SITEMAP_URLS=$TEST_URLS
EOF

echo "✅ 配置模板已生成: github-secrets-template.txt"
echo ""
echo "🎉 配置完成后，系统将自动开始工作！"