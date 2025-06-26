# 队列机制实现规划

## 🔍 现状分析

### 当前架构问题
```go
// 当前：批处理串行模式 (sitemap_monitor.go:285-305)
Step 1: extractAllKeywords() → 等待所有站点地图完成
Step 2: deduplicateKeywords() → 单线程去重  
Step 3: queryAndSubmitKeywords() → 串行API查询
```

### 文件现状检查
- ✅ `sitemap_monitor.go`: 1000+行 (需要拆分)
- ✅ 无现有队列实现
- ✅ 符合SOLID拆分要求

## 🎯 队列实现规划

### Phase 1: 最小可行队列 (MVP)

#### 新增文件 (符合<300行限制)
```
pkg/
├── queue/
│   ├── keyword_queue.go      # 核心队列 (~150行)
│   ├── batch_processor.go    # 批次处理 (~200行)
│   └── pipeline_monitor.go   # 流水线监控 (~100行)
```

#### 核心设计原则
- **KISS**: 最简单的生产者-消费者模式
- **SOLID**: 每个组件单一职责
- **不过度设计**: 无高级特性，只解决当前问题

### 🏗️ 架构设计

#### 1. KeywordQueue (关键词队列)
```go
// pkg/queue/keyword_queue.go (~150行)
type KeywordQueue struct {
    items    chan KeywordBatch  // 队列缓冲
    closed   bool               // 关闭状态
    metrics  QueueMetrics       // 简单指标
}

type KeywordBatch struct {
    Keywords   []string
    SourceURL  string  
    SitemapURL string
}

// 核心方法 (8个，符合接口隔离)
func NewKeywordQueue(bufferSize int) *KeywordQueue
func (kq *KeywordQueue) Enqueue(batch KeywordBatch) error
func (kq *KeywordQueue) Dequeue() (KeywordBatch, bool)
func (kq *KeywordQueue) Close()
```

#### 2. BatchProcessor (批次处理器)
```go
// pkg/queue/batch_processor.go (~200行)
type BatchProcessor struct {
    queue       *KeywordQueue
    deduplicator map[string]bool  // 简单去重
    batchSize   int              // 批次大小=4
}

// 职责分离
func (bp *BatchProcessor) StartProducer(ctx context.Context) // 生产者
func (bp *BatchProcessor) StartConsumer(ctx context.Context) // 消费者
func (bp *BatchProcessor) ProcessSitemaps() // 处理站点地图
func (bp *BatchProcessor) QueryAPIs() // 查询API
```

#### 3. PipelineMonitor (流水线监控)
```go
// pkg/queue/pipeline_monitor.go (~100行)
type PipelineMonitor struct {
    produced  int64
    consumed  int64
    failed    int64
    startTime time.Time
}

// 简单监控指标
func (pm *PipelineMonitor) GetMetrics() PipelineMetrics
func (pm *PipelineMonitor) RecordProduced(count int)
func (pm *PipelineMonitor) RecordConsumed(count int)
```

### 🔄 集成方案

#### 修改 sitemap_monitor.go
```go
// 替换现有的三步串行处理
func (sm *SitemapMonitor) ProcessSitemaps(ctx context.Context, sitemapURLs []string, workers int) ([]MonitorResult, error) {
    // 创建队列系统
    queue := queue.NewKeywordQueue(1000)
    processor := queue.NewBatchProcessor(queue, 4) // 批次大小=4
    
    // 启动流水线
    go processor.StartProducer(ctx, sitemapURLs, workers)
    go processor.StartConsumer(ctx)
    
    // 等待完成
    return processor.WaitForCompletion(ctx)
}
```

## 📋 实施步骤

### Step 1: 创建队列组件 (1天)
- [ ] 创建 `pkg/queue/` 目录
- [ ] 实现 `keyword_queue.go` (核心队列)
- [ ] 单元测试覆盖

### Step 2: 实现批次处理器 (1天)  
- [ ] 实现 `batch_processor.go` (生产消费逻辑)
- [ ] 集成去重功能
- [ ] 测试验证

### Step 3: 流水线监控 (0.5天)
- [ ] 实现 `pipeline_monitor.go` (指标收集)
- [ ] 基础监控面板

### Step 4: 集成现有系统 (1天)
- [ ] 修改 `sitemap_monitor.go` 集成队列
- [ ] 保持向后兼容
- [ ] 端到端测试

### Step 5: 优化调试 (0.5天)
- [ ] 性能调优
- [ ] 错误处理完善
- [ ] 文档更新

## 🛡️ 风险控制

### 三不原则检查
1. **不改架构**: ✅ 仅添加队列层，保持现有接口
2. **不做计划外功能**: ✅ 只实现基础队列，无高级特性  
3. **不创建相似逻辑**: ✅ 复用现有组件，避免重复

### SOLID合规验证
- **S**: ✅ 每个类单一职责 (队列/处理/监控)
- **O**: ✅ 可扩展队列大小和处理逻辑
- **L**: ✅ 可替换不同队列实现
- **I**: ✅ 精简接口，职责明确
- **D**: ✅ 依赖抽象队列接口

### 文件大小控制
- ✅ 每个文件<300行
- ✅ 按功能模块拆分
- ✅ 符合Go包组织最佳实践

## 🎯 最终目标

### 性能提升预期
- **吞吐量**: +30% (流水线并行)
- **内存使用**: -80% (固定队列大小)
- **首次响应**: -90% (边产生边消费)

### 代码质量
- **可维护性**: 模块化设计，职责清晰
- **可测试性**: 每个组件独立测试
- **可扩展性**: 队列参数可配置

## ✅ 批准条件

满足以下条件后开始实施：
- [ ] 确认不影响现有功能
- [ ] 设计评审通过
- [ ] 测试策略确定
- [ ] 回滚方案制定

**总工期**: 4天
**风险等级**: 低 (增量式添加，保持向后兼容)
**优先级**: 高 (解决当前性能瓶颈)