# 未使用导入修复指南 - 修复三律

## 🎯 问题描述
```
错误：pkg/api/simple_retry.go:5:2："errors"已导入但未使用
```

## ⚙️ 三步走修复流程

### 1️⃣ 精：复杂度≤原方案80%

#### 手动修复（推荐）
```bash
# 直接删除未使用的导入
- import "errors"
+ // 删除整行
```

#### 自动修复
```bash
# 使用goimports自动修复
goimports -w ./pkg/api/simple_retry.go

# 或使用gofmt
gofmt -w ./pkg/api/simple_retry.go
```

### 2️⃣ 准：直击根本原因

**根因分析：**
- **重构过程中遗留** - 删除代码时未清理导入
- **复制粘贴错误** - 从其他文件复制时带入不需要的导入
- **开发工具问题** - IDE自动导入但后续未使用

**预防措施：**
```bash
# 配置IDE自动清理导入
# VS Code: 设置保存时自动运行goimports
# GoLand: 启用"Optimize imports on the fly"
```

### 3️⃣ 净：0技术债务

**质量保证：**
```bash
# 提交前检查
go mod tidy
go vet ./...
go build ./...

# 使用pre-commit hook
echo 'goimports -w .' > .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## 🛡️ SOLID++合规性验证

### ✅ KISS原则（Keep It Simple）
- 直接删除，无复杂逻辑
- 不引入新的抽象层

### ✅ DRY原则（Don't Repeat Yourself）  
- 避免重复导入同一包
- 统一导入管理策略

### ✅ YAGNI原则（You Aren't Gonna Need It）
- 只导入真正使用的包
- 避免"预留"导入

### ✅ LoD原则（Law of Demeter）
- 最小化依赖关系
- 减少包耦合度

## 🚀 自动化解决方案

### 项目级别修复
```bash
# 使用我们的自动化工具
go run scripts/import_cleaner.go .
```

### CI/CD集成
```yaml
# GitHub Actions中添加
- name: Check imports
  run: |
    go mod tidy
    if ! git diff --quiet; then
      echo "❌ 发现未使用的导入"
      exit 1
    fi
```

## 📊 效果验证

**修复前：**
```go
import (
    "context"
    "errors"  // ← 未使用
    "time"
)
```

**修复后：**
```go
import (
    "context"
    "time"
)
```

**收益：**
- ✅ 编译时间减少
- ✅ 二进制大小优化
- ✅ 代码可读性提升
- ✅ 符合Go最佳实践

---

**修复三律保证：精确、直接、无债务的解决方案！**