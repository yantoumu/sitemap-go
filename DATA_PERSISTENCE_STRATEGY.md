# 数据持久化策略说明

## 为什么需要数据持久化？

1. **URL去重** - 避免重复处理已分析的URL
2. **失败重试** - 累积失败关键词，下次优先处理
3. **趋势分析** - 对比历史数据，发现变化趋势
4. **性能优化** - 跳过已处理内容，节省API调用

## 当前方案：专用数据分支

### 架构设计
```
main branch (代码)          monitoring-data branch (数据)
     |                              |
     ├── pkg/                       ├── data/
     ├── main.go                    ├── DATA_STATS.md
     └── .github/workflows/         └── README.md
```

### 优势对比

| 特性 | 直接写main | 专用数据分支 | 外部存储 |
|------|-----------|-------------|----------|
| 数据持久化 | ✅ | ✅ | ✅ |
| 历史追踪 | ✅ | ✅ | ❌ |
| 代码隔离 | ❌ | ✅ | ✅ |
| 免费使用 | ✅ | ✅ | ❌ |
| 简单实施 | ✅ | ✅ | ❌ |
| 防止冲突 | ❌ | ✅ | ✅ |

### 数据流程

1. **启动时恢复**
   ```bash
   git checkout origin/monitoring-data -- data/
   ```

2. **运行时对比**
   - SimpleTracker加载已处理URL哈希
   - 自动过滤重复URL
   - 累积新的失败关键词

3. **结束时保存**
   ```bash
   git checkout monitoring-data
   git add data/
   git commit -m "data: update"
   git push origin monitoring-data
   ```

### 安全措施

1. **分支保护**
   - 设置分支保护规则，防止误删
   - 限制只有Actions可以写入

2. **数据加密**
   - 所有数据文件使用AES加密
   - 密钥存储在GitHub Secrets

3. **自动清理**
   - 30天自动删除旧数据
   - 防止仓库无限增长

4. **审计追踪**
   - 每次提交包含运行ID和时间戳
   - 可追溯数据变更历史

### 最佳实践

1. **定期审查数据分支大小**
   ```bash
   git checkout monitoring-data
   du -sh data/
   ```

2. **必要时手动清理**
   ```bash
   git checkout monitoring-data
   find data/archive -mtime +60 -delete
   git add -u
   git commit -m "chore: cleanup old archives"
   ```

3. **数据分支不要合并到main**
   - 永远保持数据和代码分离
   - 使用 `[skip ci]` 避免触发其他工作流

### 迁移指南

如果已有数据在main分支：
1. 创建数据分支：`git checkout -b monitoring-data`
2. 移动数据文件：`git mv monitoring-data/* data/`
3. 从main删除：`git rm -r monitoring-data/`
4. 更新工作流使用新方案

## 总结

这个方案平衡了：
- ✅ 数据持久化需求
- ✅ 代码仓库清洁
- ✅ 实施简单性
- ✅ 成本效益（免费）
- ✅ 安全性

是目前最适合的解决方案。