# Zero Balance Loss - 项目实施计划

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
