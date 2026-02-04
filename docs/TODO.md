# Zero Balance Loss - 项目实施计划

## ✅ 最新更新：并发时序泳道图可视化（2026-02-02）

### 🎉 新功能：微观甘特图（Swim Lane Visualization）

已成功将平面日志升级为**并发时序泳道图**，实现了以下功能：

#### 核心特性
- ✅ **纳秒级时间戳**：精确记录每个操作的开始和结束时间
- ✅ **三阶段可视化**：读取（蓝色）、计算（黄色）、写入（绿色）
- ✅ **竞态窗口检测**：自动识别并标记时间重叠区域（红色）
- ✅ **实时刷新**：每2秒自动更新泳道图数据
- ✅ **交互式图表**：鼠标悬停查看详细信息

#### 技术实现
- 后端：Go + 纳秒级埋点 + Timeline数据结构
- 前端：ECharts 5.4.3 + 自定义甘特图渲染
- API：`GET /api/swimlane` 和 `POST /api/swimlane/clear`

#### 快速体验
```bash
# 启动服务
go run main.go

# 访问
http://localhost:8080

# 操作步骤
1. 设置初始余额 1000元
2. 点击"发起并发攻击"（确保无锁模式）
3. 观察泳道图中的红色竞态窗口
4. 切换到加锁模式对比效果
```

#### 相关文档
- [详细实现说明](./SWIMLANE_VISUALIZATION.md)
- [快速上手指南](./QUICK_START_SWIMLANE.md)

---

## Plan: Zero Balance Loss - Level 1 并发余额丢失

构建一个 Go 项目来演示单机并发场景下的余额更新丢失问题。项目将包含一个故意不加锁的余额扣减接口，以及一个并发攻击脚本来证明数据不一致。使用 Gin + Gorm + MySQL + Redis 技术栈。

### Steps

1. **初始化项目结构** - 创建 go.mod，规划目录：config/, model/, service/, api/, cmd/attack/ 和 main.go
2. **设计数据库层** - 在 model/ 创建 accounts 表模型 (id, user_id, balance)，在 config/ 创建数据库连接配置
3. **实现核心服务** - 在 service/ 实现余额扣减逻辑（故意不加锁），在 api/ 使用 Gin 暴露 POST /deduct 接口
4. **编写攻击脚本** - 在 cmd/attack/ 创建并发测试程序，发起多个并发请求验证余额丢失问题
5. **添加配置和启动** - 创建配置文件（数据库连接、服务端口），完善 main.go 启动逻辑
6. **准备测试数据** - 创建 SQL 初始化脚本，插入测试账户（如 user_id=1, balance=1000）

### Further Considerations

1. **数据库字段类型** - balance 使用 DECIMAL(20,2) 还是 INT（存储分）？建议 DECIMAL 更贴近实际场景
2. **并发攻击强度** - 攻击脚本并发数设置多少合适？建议 100 goroutines × 10 次扣减，容易复现问题
3. **Redis 使用时机** - Level 1 仅演示单机并发问题，Redis 是否先保留接口暂不使用，待 Level 2 分布式锁时再启用？

---

## 项目概述
开发一个名为 "Zero Balance Loss" 的 Go 语言实战项目，用于模拟互金场景下的分布式并发问题。

**技术栈：** Go (Gin), Gorm (MySQL), Redis (Go-Redis)

**当前目标：** Level 1 - 单机并发下的余额更新丢失

---

## 项目结构设计

```
DDIA/
├── main.go                 # 程序入口
├── go.mod                  # Go 模块定义
├── config/                 # 配置相关
│   ├── config.go          # 配置结构体和加载
│   └── database.go        # 数据库连接初始化
├── model/                  # 数据模型
│   └── account.go         # accounts 表模型
├── service/                # 业务逻辑层
│   └── account_service.go # 余额扣减逻辑（不加锁）
├── api/                    # API 路由和处理器
│   └── handler.go         # HTTP 请求处理
├── cmd/                    # 命令行工具
│   └── attack/            # 并发攻击脚本
│       └── main.go        # 并发测试程序
└── init.sql               # 数据库初始化脚本
```

---

## 数据库设计

### accounts 表结构

```sql
CREATE TABLE accounts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE,
    balance DECIMAL(20,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**字段说明：**
- `id`: 主键，自增
- `user_id`: 用户 ID，唯一索引
- `balance`: 账户余额，使用 DECIMAL(20,2) 精确存储金额
- `created_at/updated_at`: 时间戳

---

## 核心接口定义

### POST /deduct - 余额扣减接口

**请求参数：**
```json
{
    "user_id": 1,
    "amount": 10.00
}
```

**响应示例（成功）：**
```json
{
    "code": 200,
    "message": "success",
    "data": {
        "user_id": 1,
        "balance": 990.00
    }
}
```

**响应示例（余额不足）：**
```json
{
    "code": 400,
    "message": "insufficient balance"
}
```

---

## Level 1 任务描述

### 目标
演示单机并发场景下，**不加锁**的余额扣减操作会导致余额更新丢失。

### 实现要点

#### 1. 错误代码实现（故意不加锁）
```go
// service/account_service.go
func DeductBalance(userID int64, amount float64) error {
    // 1. 查询当前余额
    account := GetAccount(userID)
    
    // 2. 检查余额是否充足
    if account.Balance < amount {
        return errors.New("insufficient balance")
    }
    
    // 3. 计算新余额
    newBalance := account.Balance - amount
    
    // 4. 更新数据库（问题所在：读写之间没有任何保护）
    db.Model(&account).Update("balance", newBalance)
    
    return nil
}
```

**问题分析：**
- 多个请求并发执行时，可能读取到相同的余额
- 导致后续的更新操作基于过期数据，造成余额丢失

#### 2. 并发攻击脚本
```go
// cmd/attack/main.go
// 启动 100 个 goroutine，每个发起 10 次扣减请求
// 预期扣减：100 * 10 * 10.00 = 10000.00
// 实际余额：由于并发丢失，会远大于 -9000.00
```

### 测试流程
1. 初始化账户：user_id=1, balance=1000.00
2. 运行攻击脚本：100 并发 × 10 次请求，每次扣 10.00
3. 理论结果：1000 - (100×10×10) = -9000.00
4. 实际结果：由于并发丢失，余额会明显偏高（如 -8000, -7500 等）

---

## 实施步骤

- [ ] **Step 1: 初始化项目结构**
  - [ ] 创建 go.mod
  - [ ] 创建目录结构：config/, model/, service/, api/, cmd/attack/
  - [ ] 创建 main.go 骨架

- [ ] **Step 2: 数据库层设计**
  - [ ] model/account.go - 定义 Account 模型
  - [ ] config/database.go - 实现数据库连接
  - [ ] 创建 init.sql - 建表和初始化数据

- [ ] **Step 3: 核心服务实现**
  - [ ] service/account_service.go - 实现不加锁的余额扣减逻辑
  - [ ] api/handler.go - 实现 POST /deduct 接口

- [ ] **Step 4: 攻击脚本开发**
  - [ ] cmd/attack/main.go - 创建并发测试程序
  - [ ] 设置并发数：100 goroutines
  - [ ] 设置每个 goroutine 的请求次数：10 次
  - [ ] 每次扣减金额：10.00

- [ ] **Step 5: 配置和启动**
  - [ ] config/config.go - 定义配置结构体（数据库连接、端口等）
  - [ ] main.go - 完善启动逻辑（初始化 DB、启动 Gin 服务器）
  - [ ] 创建示例配置文件（可选）

- [ ] **Step 6: 测试验证**
  - [ ] 初始化数据库表和测试数据
  - [ ] 启动服务
  - [ ] 运行攻击脚本
  - [ ] 验证余额丢失问题

---

## 待讨论问题

### 1. 数据库字段类型
- **选项 A**: balance DECIMAL(20,2) - 更贴近实际金融场景
- **选项 B**: balance BIGINT - 存储分（需要 × 100），性能更好
- **建议**: 使用 DECIMAL(20,2)，演示更直观

### 2. 并发攻击强度
- **当前设置**: 100 goroutines × 10 次 × 10.00 元
- **是否调整**: 可根据机器性能和演示效果调整

### 3. Redis 使用时机
- **Level 1**: 仅演示单机并发问题，暂不使用 Redis
- **Level 2**: 引入分布式锁（Redis SETNX）
- **Level 3**: 引入分布式场景（多实例）

---

## 预期成果

完成后将得到：
1. ✅ 一个可运行的 Go Web 服务（Gin + Gorm）
2. ✅ 一个演示并发问题的错误实现
3. ✅ 一个可复现余额丢失的攻击脚本
4. ✅ 清晰的问题现象和日志输出

---

**状态**: 等待确认执行

**创建时间**: 2026-01-31

---

## 🚀 生产级功能改进计划（2026-02-04）

### 📊 当前项目评估

**已完成（教学演示级）**：
- ✅ 基础并发问题演示（Lost Update）
- ✅ 单机互斥锁（sync.Mutex）
- ✅ 冲突可视化器（双边对决动画）
- ✅ 实时监控（WebSocket + Chart.js）

**局限性（玩具级 → 生产级的差距）**：
- ❌ 单机锁无法应对多实例部署
- ❌ 无缓存层，高并发直击数据库
- ❌ 无限流熔断，服务易被打垮
- ❌ 无监控告警，故障无感知
- ❌ 无降级策略，雪崩风险高

---

### 🎯 改进路线图（渐进式升级）

#### 🟢 **阶段1：数据库层优化**（基础必备，1-2天）

##### 1.1 乐观锁（Version版本号）⭐
```sql
-- 添加版本号字段
ALTER TABLE accounts ADD COLUMN version INT DEFAULT 0;

-- 更新时校验版本
UPDATE accounts 
SET balance = ?, version = version + 1 
WHERE user_id = ? AND version = ?;
```

**任务清单**：
- [ ] model/account.go 添加 Version 字段
- [ ] service 层实现 CAS（Compare-And-Swap）逻辑
- [ ] 可视化器展示版本冲突检测
- [ ] 对比三种模式：无锁 vs 互斥锁 vs 乐观锁

**优点**：
- ✅ 无需外部依赖
- ✅ 适合读多写少场景
- ✅ 性能优于悲观锁

**实现难度**：⭐⭐

---

##### 1.2 悲观锁（SELECT FOR UPDATE）
```go
// 查询时加行锁
db.Raw("SELECT * FROM accounts WHERE user_id = ? FOR UPDATE", userID).Scan(&account)
```

**任务清单**：
- [ ] 实现 DeductWithPessimisticLock 方法
- [ ] 可视化器显示锁等待时间
- [ ] 性能压测对比（TPS、P99延迟）

**实现难度**：⭐

---

#### 🟡 **阶段2：分布式锁**（核心能力，2-3天）⭐⭐⭐

##### 2.1 Redis分布式锁（Redlock算法）
```go
// 1. 添加 Redis 客户端
import "github.com/go-redis/redis/v8"

// 2. 实现分布式锁
func (s *AccountService) DeductWithRedisLock(req DeductRequest) error {
    lockKey := fmt.Sprintf("lock:account:%d", req.UserID)
    lockValue := uuid.New().String()
    
    // 获取锁（SET NX EX 10）
    ok, _ := s.redis.SetNX(ctx, lockKey, lockValue, 10*time.Second).Result()
    if !ok {
        return errors.New("获取锁失败，请重试")
    }
    
    defer func() {
        // Lua脚本原子释放锁
        script := `
            if redis.call("get", KEYS[1]) == ARGV[1] then
                return redis.call("del", KEYS[1])
            else
                return 0
            end
        `
        s.redis.Eval(ctx, script, []string{lockKey}, lockValue)
    }()
    
    // 执行业务逻辑
    return s.deductBalance(req)
}
```

**任务清单**：
- [ ] docker-compose.yml 添加 Redis 服务
- [ ] 集成 go-redis 客户端
- [ ] 实现 Redlock（单节点版）
- [ ] 实现 Redlock（多节点版，可选）
- [ ] 可视化器显示分布式锁状态
- [ ] 部署多个服务实例验证效果
- [ ] 压测对比：单机锁 vs 分布式锁

**挑战**：
- ⚠️ 锁超时问题（业务执行超过10秒怎么办？）
- ⚠️ 锁误删问题（进程A的锁被进程B删除）
- ⚠️ 锁重入问题（同一个请求需要多次加锁）

**实现难度**：⭐⭐⭐

---

#### 🟠 **阶段3：缓存层**（性能优化，2-3天）

##### 3.1 Redis缓存余额
```go
// 1. 查询余额（Cache-Aside模式）
func (s *AccountService) GetBalance(userID int64) (int64, error) {
    cacheKey := fmt.Sprintf("balance:%d", userID)
    
    // 1️⃣ 查缓存
    val, err := s.redis.Get(ctx, cacheKey).Int64()
    if err == nil {
        return val, nil // 缓存命中
    }
    
    // 2️⃣ 查数据库
    var account model.Account
    s.db.Where("user_id = ?", userID).First(&account)
    
    // 3️⃣ 写缓存（TTL 5分钟 + 随机30秒）
    ttl := 5*time.Minute + time.Duration(rand.Intn(30))*time.Second
    s.redis.Set(ctx, cacheKey, account.Balance, ttl)
    
    return account.Balance, nil
}

// 2. 更新余额（先删缓存，再更新DB）
func (s *AccountService) UpdateBalance(userID int64, newBalance int64) error {
    cacheKey := fmt.Sprintf("balance:%d", userID)
    
    // 1️⃣ 删除缓存
    s.redis.Del(ctx, cacheKey)
    
    // 2️⃣ 更新数据库
    s.db.Model(&model.Account{}).
        Where("user_id = ?", userID).
        Update("balance", newBalance)
    
    return nil
}
```

**任务清单**：
- [ ] 实现 Cache-Aside 模式
- [ ] 添加布隆过滤器防穿透
- [ ] 添加随机TTL防雪崩
- [ ] 监控缓存命中率
- [ ] 压测对比：无缓存 vs 有缓存

**挑战**：
- ⚠️ 缓存一致性（双写不一致）
- ⚠️ 缓存穿透（查询不存在的数据）
- ⚠️ 缓存雪崩（大量key同时过期）
- ⚠️ 缓存击穿（热点key过期）

**实现难度**：⭐⭐⭐⭐

---

#### 🔴 **阶段4：限流熔断**（服务保护，1-2天）

##### 4.1 令牌桶限流
```go
import "golang.org/x/time/rate"

// 全局限流器：每秒1000个请求，桶容量2000
var limiter = rate.NewLimiter(1000, 2000)

func DeductHandler(c *gin.Context) {
    // 限流检查
    if !limiter.Allow() {
        c.JSON(429, gin.H{
            "code":    429,
            "message": "请求过快，请稍后重试",
        })
        return
    }
    
    // 正常业务逻辑
    // ...
}
```

**任务清单**：
- [ ] 集成 golang.org/x/time/rate
- [ ] 实现全局限流（服务级）
- [ ] 实现用户级限流（按user_id）
- [ ] Web界面显示限流状态
- [ ] 压测触发限流并观察效果

---

##### 4.2 熔断器
```go
import "github.com/sony/gobreaker"

// MySQL连接熔断器
var dbBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "MySQL",
    MaxRequests: 3,              // 半开状态最多3个请求
    Interval:    10 * time.Second, // 统计周期
    Timeout:     60 * time.Second, // 熔断后60秒尝试恢复
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 3 && failureRatio >= 0.6 // 错误率60%熔断
    },
})

func QueryDB(userID int64) (*model.Account, error) {
    result, err := dbBreaker.Execute(func() (interface{}, error) {
        var account model.Account
        err := db.Where("user_id = ?", userID).First(&account).Error
        return &account, err
    })
    
    if err != nil {
        return nil, err
    }
    return result.(*model.Account), nil
}
```

**任务清单**：
- [ ] 集成 github.com/sony/gobreaker
- [ ] 为数据库连接添加熔断器
- [ ] 为Redis连接添加熔断器
- [ ] Web界面显示熔断器状态
- [ ] 模拟数据库故障触发熔断

**实现难度**：⭐⭐⭐

---

#### 🟣 **阶段5：可观测性**（生产必备，3-5天）

##### 5.1 Prometheus监控
```go
import "github.com/prometheus/client_golang/prometheus"

// 自定义指标
var (
    // 请求计数器
    requestCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "deduct_requests_total",
            Help: "Total number of deduct requests",
        },
        []string{"status", "lock_type"},
    )
    
    // 余额丢失金额
    balanceLossGauge = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "balance_loss_amount",
            Help: "Total amount of balance loss",
        },
    )
    
    // 请求延迟直方图
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "deduct_duration_seconds",
            Help:    "Deduct request duration",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"lock_type"},
    )
    
    // 锁等待时间
    lockWaitDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "lock_wait_duration_seconds",
            Help:    "Lock wait duration",
            Buckets: prometheus.DefBuckets,
        },
    )
)

func init() {
    prometheus.MustRegister(requestCounter, balanceLossGauge, requestDuration, lockWaitDuration)
}

// 在业务代码中埋点
func DeductHandler(c *gin.Context) {
    start := time.Now()
    
    // 业务逻辑
    err := accountService.Deduct(req)
    
    // 记录指标
    duration := time.Since(start).Seconds()
    if err != nil {
        requestCounter.WithLabelValues("error", req.LockType).Inc()
    } else {
        requestCounter.WithLabelValues("success", req.LockType).Inc()
    }
    requestDuration.WithLabelValues(req.LockType).Observe(duration)
}
```

**任务清单**：
- [ ] 集成 Prometheus Go客户端
- [ ] 添加 /metrics 端点
- [ ] docker-compose.yml 添加 Prometheus 服务
- [ ] 添加 Grafana 服务
- [ ] 创建 Grafana Dashboard
- [ ] 配置告警规则（余额丢失 > 100元）

---

##### 5.2 分布式追踪（Jaeger）
```go
import "github.com/opentracing/opentracing-go"

func DeductWithTrace(c *gin.Context, req DeductRequest) error {
    // 创建Span
    span, ctx := opentracing.StartSpanFromContext(c.Request.Context(), "deduct")
    defer span.Finish()
    
    // 添加标签
    span.SetTag("user_id", req.UserID)
    span.SetTag("amount", req.Amount)
    
    // 子操作1：查询余额
    span1, _ := opentracing.StartSpanFromContext(ctx, "query_balance")
    balance := queryBalance(ctx, req.UserID)
    span1.Finish()
    
    // 子操作2：计算新余额
    span2, _ := opentracing.StartSpanFromContext(ctx, "calculate")
    newBalance := balance - req.Amount
    span2.Finish()
    
    // 子操作3：更新数据库
    span3, _ := opentracing.StartSpanFromContext(ctx, "update_db")
    updateDB(ctx, req.UserID, newBalance)
    span3.Finish()
    
    return nil
}
```

**任务清单**：
- [ ] docker-compose.yml 添加 Jaeger 服务
- [ ] 集成 OpenTracing
- [ ] 为关键路径添加追踪埋点
- [ ] Web界面添加TraceID显示
- [ ] Jaeger UI查看调用链

**实现难度**：⭐⭐⭐⭐

---

#### ⚫ **阶段6：异步解耦**（高级架构，3-5天）

##### 6.1 消息队列（RabbitMQ）
```
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│  Web请求  │ ───> │ 写入队列  │ ───> │ 消费者    │ ───> │   扣款    │
└──────────┘      └──────────┘      └──────────┘      └──────────┘
     │                                                       │
     │                                                       ↓
     └────── 立即返回 "处理中" ──────────────────> 异步通知结果
```

**任务清单**：
- [ ] docker-compose.yml 添加 RabbitMQ
- [ ] 实现生产者（接收HTTP请求 → 发MQ）
- [ ] 实现消费者（消费MQ → 执行扣款）
- [ ] 实现幂等性（防止重复消费）
- [ ] 实现重试机制（死信队列）
- [ ] Web界面显示异步任务状态

**挑战**：
- ⚠️ 最终一致性（用户看到延迟）
- ⚠️ 消息丢失（MQ宕机）
- ⚠️ 消息重复（消费者重启）
- ⚠️ 消息乱序（多个消费者）

**实现难度**：⭐⭐⭐⭐⭐

---

#### 🔵 **阶段7：终极挑战**（进阶方向）

##### 7.1 分库分表（用户量千万级）
- [ ] 集成 ShardingSphere-Go
- [ ] 按 user_id 哈希分表（256张表）
- [ ] 跨分片查询优化

##### 7.2 分布式事务（Seata/DTM）
- [ ] 扣款 + 积分 跨服务事务
- [ ] TCC（Try-Confirm-Cancel）模式
- [ ] Saga 长事务模式

##### 7.3 CQRS + Event Sourcing
- [ ] 读写分离架构
- [ ] 事件溯源（记录所有历史变更）
- [ ] 基于事件重建余额状态

**实现难度**：⭐⭐⭐⭐⭐

---

### 🎯 优先级推荐

#### 🔥 **第一优先级（最快看到效果）**：
1. **Redis分布式锁**（2-3天）- 多实例部署对比演示 ⭐⭐⭐
2. **乐观锁**（1天）- 展示version并发控制 ⭐⭐
3. **Prometheus监控**（2天）- 实时QPS、错误率、P99延迟 ⭐⭐⭐

#### 📊 **第二优先级（生产必备）**：
4. **限流熔断**（1-2天）- 服务保护 ⭐⭐⭐
5. **缓存层**（2-3天）- 性能优化 ⭐⭐⭐⭐

#### 🚀 **第三优先级（进阶能力）**：
6. **异步解耦**（3-5天）- 削峰填谷 ⭐⭐⭐⭐⭐
7. **分布式追踪**（2-3天）- 问题排查 ⭐⭐⭐⭐

---

### 📝 实施建议

1. **渐进式改进**：每个阶段完成后部署一个新版本
2. **保留旧代码**：通过配置开关切换不同实现
3. **性能对比**：每个阶段都做压测并记录数据
4. **文档同步**：更新 README 和使用指南
5. **可视化增强**：为每个新功能添加可视化展示

---

**更新时间**：2026-02-04
**下一步行动**：选择一个阶段开始实施

