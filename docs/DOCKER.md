# Zero Balance Loss Docker 环境

本项目使用 Docker Compose 快速搭建开发环境，包含以下服务：

## 服务列表

| 服务 | 端口 | 说明 |
|------|------|------|
| MySQL | 3306 | 数据库 |
| Redis | 6379 | 缓存 |
| Kafka | 9092 | 消息队列 |
| Zookeeper | 2181 | Kafka依赖 |
| Kafka UI | 8090 | Kafka管理界面 |

## 快速启动

### 1. 启动所有服务
```bash
docker-compose up -d
```

### 2. 查看服务状态
```bash
docker-compose ps
```

### 3. 查看日志
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f mysql
docker-compose logs -f kafka
```

### 4. 停止服务
```bash
docker-compose down
```

### 5. 停止并删除数据卷
```bash
docker-compose down -v
```

## 服务访问

### MySQL
```bash
# 命令行连接
docker exec -it zero-balance-mysql mysql -uroot -proot zero_balance_loss

# 或使用客户端连接
Host: localhost
Port: 3306
User: root
Password: root
Database: zero_balance_loss
```

### Redis
```bash
# 命令行连接
docker exec -it zero-balance-redis redis-cli

# 测试连接
PING
```

### Kafka UI
访问 http://localhost:8090 查看Kafka集群状态、主题、消费者组等信息

### Kafka
```bash
# 创建主题
docker exec -it zero-balance-kafka kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic balance-change-topic \
  --partitions 3 \
  --replication-factor 1

# 列出所有主题
docker exec -it zero-balance-kafka kafka-topics --list \
  --bootstrap-server localhost:9092

# 查看主题详情
docker exec -it zero-balance-kafka kafka-topics --describe \
  --bootstrap-server localhost:9092 \
  --topic balance-change-topic

# 生产消息
docker exec -it zero-balance-kafka kafka-console-producer \
  --bootstrap-server localhost:9092 \
  --topic balance-change-topic

# 消费消息
docker exec -it zero-balance-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic balance-change-topic \
  --from-beginning
```

## 数据持久化

所有数据都持久化到 Docker volumes：
- `mysql_data` - MySQL数据
- `redis_data` - Redis数据
- `kafka_data` - Kafka数据
- `zookeeper_data` - Zookeeper数据
- `zookeeper_logs` - Zookeeper日志

## 网络

所有服务都在 `zero-balance-network` 网络中，可以通过服务名互相访问。

## 健康检查

所有服务都配置了健康检查，确保服务正常运行后才标记为 healthy。

## 初始化

MySQL容器启动时会自动执行 `init.sql` 脚本，创建数据库表并插入初始数据。

## 故障排查

### 查看容器状态
```bash
docker-compose ps
```

### 重启特定服务
```bash
docker-compose restart mysql
docker-compose restart kafka
```

### 查看资源使用
```bash
docker stats
```

### 清理并重建
```bash
docker-compose down -v
docker-compose up -d --build
```
