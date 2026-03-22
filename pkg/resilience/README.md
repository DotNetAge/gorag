# Resilience - 弹性包装器

为 Pipeline Step 提供限流和熔断能力，保护系统免受过载和级联故障的影响。

## 是什么

弹性包装器是横切关注点，通过装饰器模式为任意 Pipeline Step 添加速率限制和熔断能力。

### 核心组件

```
┌─────────────────────────────────────────────────────────┐
│                   弹性包装器                              │
├─────────────────────────┬───────────────────────────────┤
│   RateLimiterStepWrapper │   CircuitBreakerStepWrapper   │
│   (令牌桶限流器)          │   (熔断器)                     │
├─────────────────────────┴───────────────────────────────┤
│  防止过载                    处理故障恢复                  │
│  ⏱️ 控制请求速率              🔄 快速失败 + 回退            │
└─────────────────────────────────────────────────────────┘
```

### 熔断器状态机

```
        ┌──────────────────────────────────┐
        │                                  │
        ▼                                  │
   ┌─────────┐   失败 >= 阈值      ┌───────────┐
   │ Closed  │ ──────────────────▶ │   Open    │
   │ (正常)  │                     │  (熔断)   │
   └────┬────┘                     └─────┬─────┘
        │ 成功                         │  超时
        │                              │
        │ ◀─────────────────────────────┘
        │      进入 Half-Open 试探
        ▼
   ┌─────────┐
   │Half-Open│
   │ (半开)  │
   └────┬────┘
        │
        ├──成功──▶ ┌─────────┐
        │         │ Closed  │
        │         └─────────┘
        │
        └──失败──▶ ┌───────────┐
                  │   Open    │
                  └───────────┘
```

---

## 有什么用

### RateLimiterStepWrapper

1. **防止过载**：限制每秒请求数，避免资源耗尽
2. **成本控制**：配合 LLM API 限流，控制 token 消耗
3. **公平调度**：多用户/多租户场景下保证公平性

### CircuitBreakerStepWrapper

1. **快速失败**：故障时立即返回，避免长时间等待
2. **优雅降级**：配置 Fallback Step 提供降级服务
3. **故障隔离**：防止级联故障扩散
4. **自动恢复**：超时后自动试探恢复

---

## 怎么工作的

### 令牌桶限流

```
请求到达
    │
    ▼
┌───────────────┐
│  请求 Token?  │
└───────┬───────┘
        │
   有Token      无Token
    │              │
    ▼              ▼
  放行          等待/拒绝
    │              │
    ▼              ▼
 执行Step      等待Token
    │
    ▼
 消耗Token
```

### 熔断器逻辑

```
请求到达
    │
    ▼
 ┌────────────────────────┐
 │ 连续错误 >= 阈值?        │
 └───┬────────────────────┘
     │
  Yes │ No
   ┌──┴──┐
   ▼     ▼
 Open  执行Step
   │     │
   │    成功→重置计数器→Closed
   │
   ▼
 Fallback (如果有)
   │
   ▼
 返回错误/降级结果
```

---

## 我们怎么实现的

### 核心结构

```go
// 限流器包装器
type RateLimiterStepWrapper[T any] struct {
    BaseStep pipeline.Step[T]
    limiter  *rate.Limiter
}

// 熔断器包装器
type CircuitBreakerStepWrapper[T any] struct {
    BaseStep     pipeline.Step[T]
    FallbackStep pipeline.Step[T]
    options      BreakerOptions
    consecutiveErr atomic.Int32
    lastErrorTime  atomic.Int64
}

// 熔断器配置
type BreakerOptions struct {
    ErrorThreshold int           // 触发熔断的连续错误数
    Timeout        time.Duration // 熔断持续时间
}
```

### 配置选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `ErrorThreshold` | 3 | 连续错误达到此值触发熔断 |
| `Timeout` | 30s | 熔断持续时间 |

---

## 如何与项目集成

### 限流器示例

```go
import "golang.org/x/time/rate"

// 限制每秒 10 个请求，突发 20
limitedStep := resilience.WithRateLimiter(
    myStep,
    rate.Limit(10),
    20,
)

p.AddStep(limitedStep)
```

### 熔断器示例

```go
// 配置熔断器
breakerOpts := resilience.BreakerOptions{
    ErrorThreshold: 3,
    Timeout:        30 * time.Second,
}

// 创建带回退的熔断器
breakerStep := resilience.WithCircuitBreakerAndFallback(
    llmGenerationStep,
    fallbackStep,  // 降级步骤
    breakerOpts,
)

p.AddStep(breakerStep)
```

### 组合使用

```go
// 先限流，再熔断
p.AddStep(resilience.WithRateLimiter(step1, rate.Limit(10), 20))
p.AddStep(resilience.WithCircuitBreakerAndFallback(step2, fallback, opts))
```

### 与 Pipeline Step 配合

```go
p := pipeline.New[*core.GenerationContext]()

// 限流 + 熔断 + 回退
p.AddStep(resilience.WithRateLimiter(
    resilience.WithCircuitBreakerAndFallback(
        generationStep,
        simpleFallback,
        opts,
    ),
    rate.Limit(5),
    10,
))
```

---

## 适用于哪些场景

### ✅ 适合使用

- **LLM API 保护**：防止 API 过载和控制成本
- **外部服务调用**：处理外部 API 的不稳定
- **高并发场景**：保护下游系统
- **微服务架构**：故障隔离和优雅降级

### ❌ 不适合使用

- **低延迟要求**：限流有等待开销
- **简单场景**：单用户、无并发不需要
- **幂等操作**：失败可以安全重试

---

## API 参考

### `WithRateLimiter`

```go
func WithRateLimiter[T any](
    base pipeline.Step[T],
    limit rate.Limit,
    burst int,
) *RateLimiterStepWrapper[T]
```

创建带限流的 Step 包装器。

**参数**：
- `base`: 要包装的 Step
- `limit`: 每秒允许的请求数 (rate.Limit)
- `burst`: 允许的突发大小

### `WithCircuitBreakerAndFallback`

```go
func WithCircuitBreakerAndFallback[T any](
    base pipeline.Step[T],
    fallback pipeline.Step[T],
    opts BreakerOptions,
) *CircuitBreakerStepWrapper[T]
```

创建带熔断和回退的 Step 包装器。

**参数**：
- `base`: 要包装的 Step
- `fallback`: 降级时执行的 Step (可为 nil)
- `opts`: 熔断器配置

### `BreakerOptions`

```go
type BreakerOptions struct {
    ErrorThreshold int           // Default: 3
    Timeout        time.Duration // Default: 30s
}
```

熔断器配置选项。

---

## 测试

```bash
go test ./pkg/resilience/... -v
```

**测试覆盖**：
- `TestRateLimiterStepWrapper_Execute` - 限流执行
- `TestRateLimiterStepWrapper_Name` - 名称方法
- `TestCircuitBreakerStepWrapper_Execute_Success` - 成功执行
- `TestCircuitBreakerStepWrapper_Execute_Fallback` - 回退触发
- `TestCircuitBreakerStepWrapper_Execute_CircuitOpen` - 熔断开启
- `TestCircuitBreakerStepWrapper_Name` - 名称方法
- `TestBreakerOptions_Defaults` - 默认值
- `TestWithCircuitBreakerAndFallback_NilFallback` - nil 回退处理