# Monitoring Data Branch

这个分支专门存储sitemap监控的数据文件，用于在GitHub Actions运行间保持数据持久化。

## 数据结构
- data/pr/processed_urls.enc - 已处理URL哈希
- data/fa/failed_keywords.enc - 失败关键词
- data/si/ - Sitemap缓存文件
- DATA_SUMMARY.txt - 运行摘要

⚠️ 警告：请勿将此分支合并到main分支！

