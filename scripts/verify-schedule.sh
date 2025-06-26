#!/bin/bash

# 验证 GitHub Actions 定时配置

echo "🕐 GitHub Actions 定时执行验证"
echo "================================"
echo ""

# 获取当前时间
echo "📅 当前时间:"
echo "   UTC时间: $(date -u '+%Y-%m-%d %H:%M:%S')"
echo "   中国时间: $(TZ='Asia/Shanghai' date '+%Y-%m-%d %H:%M:%S') (UTC+8)"
echo ""

echo "⏰ 定时执行时间:"
echo "   1. 中国时间 07:00 = UTC 23:00 (前一天)"
echo "   2. 中国时间 23:00 = UTC 15:00"
echo ""

echo "📋 Cron 配置验证:"
echo "   schedule:"
echo "     - cron: '0 23 * * *'  # 中国时间早上7:00"
echo "     - cron: '0 15 * * *'  # 中国时间晚上23:00"
echo ""

# 计算下次执行时间
current_hour=$(date -u +%H)
current_minute=$(date -u +%M)

echo "🔮 下次执行时间预测:"
if [ $current_hour -lt 15 ] || ([ $current_hour -eq 15 ] && [ $current_minute -eq 0 ]); then
    echo "   UTC 15:00 今天 → 中国时间 23:00 今天"
elif [ $current_hour -lt 23 ] || ([ $current_hour -eq 23 ] && [ $current_minute -eq 0 ]); then
    echo "   UTC 23:00 今天 → 中国时间 07:00 明天"
else
    echo "   UTC 15:00 明天 → 中国时间 23:00 明天"
fi
echo ""

echo "✅ 配置文件位置:"
echo "   .github/workflows/sitemap-monitor.yml"
echo ""

echo "🔧 必需的 GitHub Secrets:"
echo "   □ BACKEND_API_URL"
echo "   □ BACKEND_API_KEY"
echo "   □ TRENDS_API_URL"
echo "   □ SITEMAP_URLS (可选)"
echo ""

echo "💡 提示:"
echo "   1. 确保已经设置好所有必需的 Secrets"
echo "   2. 推送代码到 GitHub 后，定时任务会自动生效"
echo "   3. 可以在 Actions 页面手动触发测试"