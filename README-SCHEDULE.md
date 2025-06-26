# GitHub Actions 定时执行说明

## 执行时间配置

本项目配置了每天执行 **2次** 的定时任务：

- **中国时间早上 7:00** (UTC 23:00 前一天)
- **中国时间晚上 23:00** (UTC 15:00)

## 时区转换说明

```
中国时间 (UTC+8) | UTC时间 | Cron表达式
----------------|---------|-------------
早上 7:00       | 23:00   | 0 23 * * *
晚上 23:00      | 15:00   | 0 15 * * *
```

## GitHub Actions 配置

在 `.github/workflows/sitemap-monitor.yml` 中配置：

```yaml
schedule:
  - cron: '0 23 * * *'  # 中国时间早上7:00
  - cron: '0 15 * * *'  # 中国时间晚上23:00
```

## 必需的 Secrets 配置

请在 GitHub 仓库的 Settings → Secrets and variables → Actions 中配置以下变量：

1. **BACKEND_API_URL** - 后端API地址
2. **BACKEND_API_KEY** - 后端API密钥
3. **TRENDS_API_URL** - Google Trends API地址
4. **SITEMAP_URLS** - 要监控的sitemap URLs (可选，逗号分隔)

## 手动触发

除了定时执行外，还可以通过以下方式手动触发：

1. 进入 GitHub 仓库的 Actions 页面
2. 选择 "Sitemap Monitor" workflow
3. 点击 "Run workflow" 按钮
4. 可选配置参数：
   - `sitemap_workers`: Sitemap并发数 (默认15)
   - `api_workers`: API并发数 (默认2)
   - `debug`: 是否启用调试日志 (默认false)

## 监控和日志

- 每次执行会生成摘要报告，包含中国时间
- 失败时会保存日志文件作为 artifacts (保留7天)
- 可在 Actions 页面查看历史执行记录

## 执行频率调整

如需调整执行频率，修改 cron 表达式即可。常用示例：

```yaml
# 每天执行4次 (0点、6点、12点、18点)
- cron: '0 16 * * *'  # 中国时间 0:00
- cron: '0 22 * * *'  # 中国时间 6:00  
- cron: '0 4 * * *'   # 中国时间 12:00
- cron: '0 10 * * *'  # 中国时间 18:00

# 每小时执行一次
- cron: '0 * * * *'

# 工作日执行 (周一到周五)
- cron: '0 23 * * 1-5'  # 中国时间早上7:00，仅工作日
```

## 注意事项

1. GitHub Actions 使用 UTC 时间，配置时需要考虑时差
2. 定时任务可能有几分钟的延迟
3. 免费账户每月有 2000 分钟的执行时间限制
4. 建议监控执行日志，确保任务正常运行