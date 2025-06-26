# GitHub Secrets 配置指南

## 🔧 必需的 Secrets 配置

请在 GitHub 仓库的 **Settings → Secrets and variables → Actions** 中添加以下配置：

### 1. BACKEND_API_URL
```
https://work.seokey.vip/api/v1/keyword-metrics/batch
```

### 2. BACKEND_API_KEY
```
sitemap-update-api-key-2025
```

### 3. TRENDS_API_URL
```
https://trends.google.com/trends/api/dailytrends
```
> 注意：请根据实际的Google Trends API端点进行调整

### 4. SITEMAP_URLS (可选)
```
https://poki.com/sitemap.xml,https://www.y8.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.friv.com/sitemap.xml,https://www.silvergames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.freegames.com/sitemap/games_1.xml,https://www.miniplay.com/sitemap-games-3.xml,https://kiz10.com/sitemap-games.xml,https://www.snokido.com/sitemaps/games.xml
```

## 📋 完整的游戏网站 Sitemap 示例

如果你想监控更多游戏网站，可以使用以下完整列表：

```
https://poki.com/sitemap.xml,https://www.y8.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.friv.com/sitemap.xml,https://www.silvergames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.freegames.com/sitemap/games_1.xml,https://www.miniplay.com/sitemap-games-3.xml,https://kiz10.com/sitemap-games.xml,https://www.snokido.com/sitemaps/games.xml,https://www.gamesgames.com/sitemaps/gamesgames/en/sitemap_games.xml.gz,https://www.spel.nl/sitemaps/agame/nl/sitemap_games.xml.gz,https://www.girlsgogames.it/sitemaps/girlsgogames/it/sitemap_games.xml.gz,https://html5.gamedistribution.com/sitemap.xml,https://itch.io/feed/new.xml,https://lagged.com/sitemap.txt,https://www.onlinegames.io/sitemap.xml,https://www.play-games.com/sitemap.xml,https://www.twoplayergames.org/sitemap-games.xml,https://geometrydash.io/sitemap.xml,https://sprunki.org/sitemap.xml,https://www.hoodamath.com/sitemap.xml,https://www.mathplayground.com/sitemap_main.xml
```

## 🎯 推荐的基础配置

如果你刚开始，建议使用这个精简的高质量网站列表：

```
https://poki.com/sitemap.xml,https://www.crazygames.com/sitemap.xml,https://www.1001games.com/sitemap-games.xml,https://www.miniplay.com/sitemap-games-3.xml,https://kiz10.com/sitemap-games.xml
```

## ⚙️ 配置步骤

1. **进入 GitHub 仓库**
2. **点击 Settings 标签**
3. **在左侧菜单选择 "Secrets and variables" → "Actions"**
4. **点击 "New repository secret"**
5. **逐一添加上述 4 个 secrets**

## 🔍 验证配置

配置完成后，可以：

1. **手动触发测试**：
   - 进入 Actions 页面
   - 选择 "Sitemap Monitor" workflow
   - 点击 "Run workflow"

2. **查看执行日志**：
   - 确保所有 secrets 正确加载
   - 检查 API 连接是否成功

## 📊 网站分类说明

### 主流游戏平台
- **Poki**: 大型HTML5游戏平台
- **CrazyGames**: 热门在线游戏
- **Y8**: 经典游戏网站
- **Friv**: 休闲游戏集合

### 专业游戏网站
- **1001Games**: 综合游戏门户
- **MiniPlay**: 小游戏专业平台
- **Kiz10**: 儿童游戏网站
- **Snokido**: 多语言游戏平台

### 特殊格式网站
- **Lagged**: TXT格式sitemap
- **Itch.io**: RSS Feed格式
- **HTML5 Game Distribution**: 开发者平台

## 💡 使用建议

1. **测试环境**：先用3-5个网站测试
2. **生产环境**：确认稳定后添加更多网站
3. **监控频率**：根据需要调整执行时间
4. **资源限制**：注意GitHub Actions的使用配额

## 🚨 重要提醒

- **不要在代码中硬编码敏感信息**
- **定期轮换 API 密钥**
- **监控 API 调用频率，避免被限流**
- **关注 GitHub Actions 的执行时间和资源使用**