# 🎯 Zero Balance Loss - 并发余额丢失演示系统

一个用于演示和教学并发问题的可视化系统，通过真实的余额扣减场景展示Lost Update问题。

## 📁 项目结构

```
zero-balance-loss/
├── api/                           # 后端 - API路由和处理器
│   └── handler.go                 # HTTP/WebSocket处理器
├── config/                        # 后端 - 配置管理
│   ├── config.go                  # 配置加载
│   └── database.go                # 数据库连接
├── model/                         # 后端 - 数据模型
│   └── account.go                 # 账户模型
├── service/                       # 后端 - 业务逻辑
│   └── account_service.go         # 账户服务
├── web/                           # 前端 - HTML页面
│   ├── index.html                 # 主页面（控制台）
│   └── conflict_visualizer.html   # 冲突可视化器
├── docs/                          # 文档 - 使用指南和说明
│   ├── VISUALIZER_QUICK_START.md  # 快速上手指南
│   ├── CONFLICT_VISUALIZER_GUIDE.md # 可视化器详细指南
│   ├── DOCKER.md                  # Docker部署指南
│   └── ...                        # 其他文档（共12个）
├── scripts/                       # 脚本 - 启动和初始化
│   ├── start.bat                  # Windows启动脚本
│   ├── start.sh                   # Mac/Linux启动脚本
│   └── init.sql                   # 数据库初始化SQL
├── .gitignore                     # Git忽略规则
├── main.go                        # 程序入口
├── config.yaml                    # 应用配置
├── docker-compose.yml             # Docker编排配置
├── go.mod                         # Go模块依赖
└── README.md                      # 项目说明文档
```

## 🚀 快速开始

### 方式1：使用启动脚本（推荐）

```bash
# Windows
scripts\start.bat

# Mac/Linux
chmod +x scripts/start.sh  # 首次需要赋予执行权限
./scripts/start.sh

# 访问
http://localhost:8080
```

### 方式2：手动启动

```bash
# 1. 启动MySQL
docker-compose up -d mysql

# 2. 启动应用
go run main.go

# 3. 访问
http://localhost:8080
```

## 📚 文档导航

- [快速上手指南](docs/VISUALIZER_QUICK_START.md) - 3分钟学会使用
- [冲突可视化器详细指南](docs/CONFLICT_VISUALIZER_GUIDE.md) - 完整功能说明
- [Docker部署指南](docs/DOCKER.md) - 容器化部署
- [测试清单](docs/TESTING_CHECKLIST.md) - 功能测试指南

## 🎮 核心功能

### 1. 并发余额扣减演示
- 🔓 无锁模式：演示Lost Update问题
- 🔒 加锁模式：展示正确的解决方案
- 📊 实时统计：成功率、丢失金额、QPS

### 2. 冲突可视化器
- ⚔️ 双边对决布局：直观对比两个并发请求
- 🎬 四阶段动画：慢动作回放冲突过程
- ⏱️ 精确时间线：毫秒级时间追踪
- 📝 操作历史：完整的操作记录

### 3. 实时监控
- 📈 余额变化图表：实时追踪余额走势
- 🔄 WebSocket推送：零延迟数据更新
- ⏸️ 监控控制：暂停/恢复监控

## 🎯 使用场景

- 📖 **教学演示**：向学生讲解并发问题
- 🔬 **技术分享**：团队技术培训
- 🐛 **问题排查**：定位并发bug
- 📚 **自主学习**：理解并发原理

## 🛠️ 技术栈

**后端**：
- Go 1.23.3
- Gin Web Framework
- GORM + MySQL
- WebSocket (gorilla/websocket)

**前端**：
- 原生 HTML/CSS/JavaScript
- Chart.js（余额图表）
- WebSocket（实时通信）

## 📋 系统要求

- Go 1.23+
- Docker Desktop（用于MySQL）
- 现代浏览器（Chrome/Edge/Firefox）

## 🎓 学习路径

1. **初级**：使用主页面发起并发攻击，观察余额丢失
2. **中级**：打开可视化器，理解冲突发生过程
3. **高级**：切换加锁模式，对比两种实现方式

## 📞 支持

如有问题，请查看 [docs/](docs/) 目录下的详细文档。

---

**⚠️ 注意**：本项目仅用于教学和演示，不适合生产环境使用。
