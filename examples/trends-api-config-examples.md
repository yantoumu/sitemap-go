# Google Trends API 配置示例

## 🔧 TRENDS_API_URL 配置方法

### 📋 单个API端点配置
如果只有一个Google Trends API端点：

```
TRENDS_API_URL=https://trends.google.com/trends/api/dailytrends
```

### 🔄 多个API端点配置
如果有多个Google Trends API端点，使用**逗号分隔**：

```
TRENDS_API_URL=https://api1.example.com/trends,https://api2.example.com/trends,https://backup-api.example.com/trends
```

## 📝 常见的Google Trends API端点示例

### 官方Google Trends API
```
# 每日趋势API
https://trends.google.com/trends/api/dailytrends

# 实时搜索趋势
https://trends.google.com/trends/api/realtimetrends

# 关键词兴趣度API
https://trends.google.com/trends/api/explore
```

### 第三方API服务示例
```
# SerpAPI Google Trends
https://serpapi.com/search.json?engine=google_trends

# RapidAPI Google Trends
https://google-trends8.p.rapidapi.com/trends

# 自建代理API
https://your-proxy-api.com/google-trends
```

## 🏗️ 多API配置的实际示例

### 示例1：主备API配置
```
TRENDS_API_URL=https://primary-trends-api.com/v1/trends,https://backup-trends-api.com/v1/trends
```

### 示例2：多服务商配置
```
TRENDS_API_URL=https://serpapi.com/trends,https://rapidapi.com/google-trends,https://custom-api.com/trends
```

### 示例3：地域分布配置
```
TRENDS_API_URL=https://us-trends-api.com/trends,https://eu-trends-api.com/trends,https://asia-trends-api.com/trends
```

## ⚙️ 系统如何处理多个API

当配置多个API端点时，系统会：

1. **轮询使用**：依次尝试每个API端点
2. **故障转移**：如果一个API失败，自动切换到下一个
3. **负载均衡**：分散请求到不同的API端点
4. **超时重试**：每个API都有独立的超时和重试机制

## 📊 配置格式验证

### ✅ 正确格式
```bash
# 单个API
TRENDS_API_URL=https://api.example.com/trends

# 多个API（逗号分隔，无空格）
TRENDS_API_URL=https://api1.com/trends,https://api2.com/trends,https://api3.com/trends

# 多个API（带协议）
TRENDS_API_URL=https://api1.example.com/v1/trends,http://internal-api.local:8080/trends,https://backup.example.com/api/trends
```

### ❌ 错误格式
```bash
# 错误：包含空格
TRENDS_API_URL=https://api1.com/trends, https://api2.com/trends

# 错误：使用分号
TRENDS_API_URL=https://api1.com/trends;https://api2.com/trends

# 错误：缺少协议
TRENDS_API_URL=api1.com/trends,api2.com/trends
```

## 🔐 带认证的API配置示例

### API Key认证
```
TRENDS_API_URL=https://api.example.com/trends?key=YOUR_API_KEY
```

### 多个带认证的API
```
TRENDS_API_URL=https://api1.com/trends?key=KEY1,https://api2.com/trends?token=TOKEN2,https://api3.com/trends?auth=AUTH3
```

## 🌍 地域特定配置示例

### 全球配置
```
TRENDS_API_URL=https://trends-global.com/api,https://worldwide-trends.com/v1,https://global-search-api.com/trends
```

### 中国地区优化
```
TRENDS_API_URL=https://cn-trends-api.com/api,https://asia-trends.com/v1,https://backup-global.com/trends
```

## 📋 推荐的生产配置

### 基础配置（推荐）
```
TRENDS_API_URL=https://primary-trends-api.com/v1/trends,https://backup-trends-api.com/v1/trends
```

### 高可用配置
```
TRENDS_API_URL=https://api1.trends-service.com/v1,https://api2.trends-service.com/v1,https://api3.trends-service.com/v1,https://fallback.trends-service.com/api
```

### 开发测试配置
```
TRENDS_API_URL=https://test-trends-api.com/v1/trends,http://localhost:8080/mock-trends
```

## 🔧 GitHub Secrets 配置示例

在GitHub Secrets中，选择适合你的配置：

### Secret Name: `TRENDS_API_URL`
### Secret Value（选择其中一种）:

**单API配置**:
```
https://trends.google.com/trends/api/dailytrends
```

**双API备份配置**:
```
https://primary-api.com/trends,https://backup-api.com/trends
```

**多API负载均衡配置**:
```
https://api1.example.com/trends,https://api2.example.com/trends,https://api3.example.com/trends
```

## 🚨 重要注意事项

1. **URL格式**：确保每个URL都是完整的，包含协议(http/https)
2. **分隔符**：只使用逗号(,)分隔，不要添加空格
3. **API限制**：注意每个API的调用频率限制
4. **认证信息**：如果URL包含API密钥，确保妥善保护
5. **测试验证**：配置后进行手动测试确认所有API都可访问

## 💡 最佳实践

1. **至少配置2个API**：保证高可用性
2. **混合服务商**：避免单点故障
3. **定期检查**：确保所有API端点都正常工作
4. **监控日志**：关注API调用成功率和响应时间