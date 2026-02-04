# 🚀 Zero Balance Loss - 生产级升级实施计划

## 📋 升级路线图

### 🎯 核心目标
将教学演示系统升级为**接近生产环境**的并发控制系统，展示真实场景下的各种解决方案。

---

## 🔥 **阶段1：分布式锁（Redis）**
**优先级：P0 | 时间：1-2天 | 难度：⭐⭐⭐**

### 为什么优先？
- ✅ 单机锁无法应对多实例部署
- ✅ 这是最常见的生产解决方案
- ✅ Redis基础设施已就绪

### 实施步骤

#### 1.1 集成 Redis 客户端
```bash
# 已经在 go.mod 中
go get github.com/go-redis/redis/v8
```

#### 1.2 实现分布式锁服务
创建 `service/redis_lock.go`：

```go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"
    
    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
)

type RedisLock struct {
    client *redis.Client
    key    string
    value  string
    ttl    time.Duration
}

// AcquireLock 获取分布式锁
func AcquireLock(client *redis.Client, key string, ttl time.Duration) (*RedisLock, error) {
    lockValue := uuid.New().String()
    
    ctx := context.Background()
    ok, err := client.SetNX(ctx, key, lockValue, ttl).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to acquire lock: %w", err)
    }
    
    if !ok {
        return nil, errors.New("lock is held by another process")
    }
    
    return &RedisLock{
        client: client,
        key:    key,
        value:  lockValue,
        ttl:    ttl,
    }, nil
}

// Release 释放锁（使用 Lua 脚本保证原子性）
func (l *RedisLock) Release() error {
    ctx := context.Background()
    
    // Lua 脚本：只有持锁者才能释放
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
    
    result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Int64()
    if err != nil {
        return fmt.Errorf("failed to release lock: %w", err)
    }
    
    if result == 0 {
        return errors.New("lock was already released or expired")
    }
    
    return nil
}
```

#### 1.3 实现分布式锁扣款方法
在 `service/account_service.go` 添加：

```go
// DeductBalanceWithRedisLock 使用Redis分布式锁扣减余额
func (s *AccountService) DeductBalanceWithRedisLock(req *DeductRequest, requestID string) (*DeductResponse, error) {
    db := config.GetDB()
    redisClient := config.GetRedis() // 需要在 config 中添加
    var timeline Timeline
    
    // 🔒 获取分布式锁
    lockKey := fmt.Sprintf("lock:account:%d", req.UserID)
    lock, err := AcquireLock(redisClient, lockKey, 10*time.Second)
    if err != nil {
        return nil, fmt.Errorf("获取分布式锁失败: %w", err)
    }
    defer lock.Release()
    
    log.Printf("[%s] 🔓 [REDIS LOCK] 获取锁成功", requestID)
    
    // 后续逻辑与 DeductBalanceWithLock 相同
    // ... (读取、计算、更新)
    
    return &DeductResponse{
        UserID:     req.UserID,
        Balance:    newBalance,
        OldBalance: oldBalance,
        RequestID:  requestID,
        Timeline:   timeline,
    }, nil
}
```

#### 1.4 配置 Redis 连接
在 `config/redis.go` 中添加：

```go
package config

import (
    "context"
    "fmt"
    "log"
    
    "github.com/go-redis/redis/v8"
)

var redisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis() {
    cfg := GetConfig()
    
    redisClient = redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
        Password: cfg.Redis.Password,
        DB:       cfg.Redis.DB,
        PoolSize: cfg.Redis.PoolSize,
    })
    
    // 测试连接
    ctx := context.Background()
    if err := redisClient.Ping(ctx).Err(); err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }
    
    log.Println("✅ Redis connected successfully")
}

// GetRedis 获取 Redis 客户端
func GetRedis() *redis.Client {
    return redisClient
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() {
    if redisClient != nil {
        redisClient.Close()
    }
}
```

#### 1.5 更新 API 路由
在 `api/handler.go` 添加新的路由：

```go
// 分布式锁扣款接口
r.POST("/api/deduct/redis-lock", func(c *gin.Context) {
    var req service.DeductRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    requestID := generateRequestID()
    resp, err := accountService.DeductBalanceWithRedisLock(&req, requestID)
    
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, resp)
})
```

#### 1.6 前端界面添加新模式
在 `web/index.html` 中添加：

```html
<select id="lockMode">
    <option value="no-lock">无锁模式（教学）</option>
    <option value="mutex">单机互斥锁</option>
    <option value="redis-lock">Redis分布式锁 ⭐️</option>
</select>
```

#### 1.7 Docker 部署多实例验证
更新 `docker-compose.yml`：

```yaml
services:
  app-1:
    build: .
    container_name: zero-balance-app-1
    environment:
      - INSTANCE_ID=1
    ports:
      - "8081:8080"
    depends_on:
      - mysql
      - redis
    networks:
      - zero-balance-network
      
  app-2:
    build: .
    container_name: zero-balance-app-2
    environment:
      - INSTANCE_ID=2
    ports:
      - "8082:8080"
    depends_on:
      - mysql
      - redis
    networks:
      - zero-balance-network
      
  nginx:
    image: nginx:alpine
    container_name: zero-balance-nginx
    ports:
      - "8080:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - app-1
      - app-2
    networks:
      - zero-balance-network
```

### 验证效果
```bash
# 1. 启动多实例
docker-compose up -d

# 2. 发起并发攻击
# 观察：单机锁会失效（因为锁在不同进程中）
# 观察：Redis锁依然有效（跨进程共享）

# 3. 查看日志
docker logs zero-balance-app-1
docker logs zero-balance-app-2
```

### 预期成果
- ✅ 可演示多实例部署场景
- ✅ 对比单机锁 vs 分布式锁
- ✅ 可视化器显示锁竞争情况

---

## ⚡ **阶段2：乐观锁（Version字段）**
**优先级：P1 | 时间：1天 | 难度：⭐⭐**

### 为什么是第二优先级？
- ✅ 无需外部依赖
- ✅ 适合读多写少场景
- ✅ 能展示不同的并发控制思想

### 实施步骤

#### 2.1 添加 version 字段
```sql
ALTER TABLE accounts ADD COLUMN version INT DEFAULT 0;
```

#### 2.2 实现乐观锁扣款
```go
// DeductBalanceWithOptimisticLock 使用乐观锁扣减余额
func (s *AccountService) DeductBalanceWithOptimisticLock(req *DeductRequest, requestID string) (*DeductResponse, error) {
    db := config.GetDB()
    maxRetries := 3
    
    for retry := 0; retry < maxRetries; retry++ {
        // 1. 读取当前余额和版本号
        var account model.Account
        db.Where("user_id = ?", req.UserID).First(&account)
        
        oldBalance := account.Balance
        oldVersion := account.Version
        
        // 2. 检查余额
        if account.Balance < req.Amount {
            return nil, errors.New("insufficient balance")
        }
        
        // 3. 计算新余额
        newBalance := account.Balance - req.Amount
        
        // 4. CAS 更新（Compare-And-Swap）
        result := db.Model(&model.Account{}).
            Where("user_id = ? AND version = ?", req.UserID, oldVersion).
            Updates(map[string]interface{}{
                "balance": newBalance,
                "version": oldVersion + 1,
            })
        
        // 5. 检查是否更新成功
        if result.RowsAffected > 0 {
            log.Printf("[%s] ✅ [OPTIMISTIC LOCK] 更新成功", requestID)
            return &DeductResponse{
                UserID:     req.UserID,
                Balance:    newBalance,
                OldBalance: oldBalance,
                RequestID:  requestID,
            }, nil
        }
        
        // 6. 版本号冲突，重试
        log.Printf("[%s] ⚠️ [OPTIMISTIC LOCK] 版本冲突，重试 %d/%d", requestID, retry+1, maxRetries)
        time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
    }
    
    return nil, errors.New("乐观锁重试次数耗尽")
}
```

#### 2.3 可视化器显示重试次数
在冲突可视化器中添加：
- 显示版本号变化
- 显示重试次数
- 对比乐观锁 vs 悲观锁的性能差异

---

## 📊 **阶段3：Prometheus 监控**
**优先级：P1 | 时间：2天 | 难度：⭐⭐⭐**

### 关键指标
```go
// 扣款成功率
deduct_success_rate = 成功数 / 总请求数

// 余额丢失总额
balance_loss_total = 预期余额 - 实际余额

// 请求延迟分布
deduct_duration_seconds{quantile="0.99"}

// 锁等待时间
lock_wait_duration_seconds

// 重试次数（乐观锁）
optimistic_lock_retry_count
```

### Docker 部署
```yaml
prometheus:
  image: prom/prometheus:latest
  ports:
    - "9090:9090"
  volumes:
    - ./prometheus.yml:/etc/prometheus/prometheus.yml

grafana:
  image: grafana/grafana:latest
  ports:
    - "3000:3000"
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=admin
```

---

## 🛡️ **阶段4：限流 + 熔断**
**优先级：P2 | 时间：1-2天 | 难度：⭐⭐⭐**

### 令牌桶限流
```go
import "golang.org/x/time/rate"

var limiter = rate.NewLimiter(1000, 2000) // 每秒1000个请求

if !limiter.Allow() {
    c.JSON(429, gin.H{"error": "Too Many Requests"})
    return
}
```

### 熔断器
```go
import "github.com/sony/gobreaker"

var dbBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "MySQL",
    MaxRequests: 3,
    Timeout:     60 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 3 && failureRatio >= 0.6
    },
})
```

---

## 🚀 **阶段5：缓存层（Redis）**
**优先级：P2 | 时间：2-3天 | 难度：⭐⭐⭐⭐**

### Cache-Aside 模式
```go
func (s *AccountService) GetBalance(userID int64) (int64, error) {
    cacheKey := fmt.Sprintf("balance:%d", userID)
    
    // 1️⃣ 查缓存
    val, err := redis.Get(ctx, cacheKey).Int64()
    if err == nil {
        return val, nil // 缓存命中
    }
    
    // 2️⃣ 查数据库
    balance := queryDB(userID)
    
    // 3️⃣ 写缓存（随机TTL防雪崩）
    ttl := 5*time.Minute + time.Duration(rand.Intn(30))*time.Second
    redis.Set(ctx, cacheKey, balance, ttl)
    
    return balance, nil
}
```

### 先删缓存，再更新DB
```go
func (s *AccountService) UpdateBalance(userID int64, newBalance int64) error {
    cacheKey := fmt.Sprintf("balance:%d", userID)
    
    // 1️⃣ 删除缓存
    redis.Del(ctx, cacheKey)
    
    // 2️⃣ 更新数据库
    db.Model(&Account{}).
        Where("user_id = ?", userID).
        Update("balance", newBalance)
    
    return nil
}
```

---

## 📝 **实施建议**

### 1. 渐进式升级
每完成一个阶段，就部署一个新版本，通过配置开关切换：

```go
type LockMode string

const (
    NoLock         LockMode = "no-lock"
    MutexLock      LockMode = "mutex"
    RedisLock      LockMode = "redis-lock"
    OptimisticLock LockMode = "optimistic-lock"
)

func (s *AccountService) Deduct(req *DeductRequest, mode LockMode) error {
    switch mode {
    case RedisLock:
        return s.DeductBalanceWithRedisLock(req)
    case OptimisticLock:
        return s.DeductBalanceWithOptimisticLock(req)
    // ...
    }
}
```

### 2. 性能对比表
每个阶段完成后，做压测并记录：

| 锁类型 | TPS | P99延迟 | 成功率 | 余额丢失 |
|--------|-----|---------|--------|----------|
| 无锁 | 5000 | 10ms | 60% | -500元 |
| 单机锁 | 1000 | 50ms | 100% | 0元 |
| Redis锁 | 800 | 80ms | 100% | 0元 |
| 乐观锁 | 2000 | 30ms | 95% | 0元 |

### 3. 前端可视化增强
- 实时显示当前使用的锁类型
- 显示锁竞争情况（等待队列长度）
- 显示缓存命中率
- 显示限流/熔断状态

---

## 🎯 **总结**

按照这个路线图升级后，你的系统将能够：

✅ **演示多种生产级并发控制方案**（分布式锁、乐观锁、悲观锁）  
✅ **支持多实例部署**（Docker Compose + Nginx）  
✅ **实时监控告警**（Prometheus + Grafana）  
✅ **服务保护机制**（限流 + 熔断）  
✅ **性能优化**（Redis缓存）  

这样就能让学习者看到：
1. 从玩具级 → 生产级的完整演变过程
2. 不同方案的性能对比和适用场景
3. 真实系统需要考虑的各种细节

---

**下一步行动**：选择阶段1开始实施，需要我帮你写具体代码吗？
