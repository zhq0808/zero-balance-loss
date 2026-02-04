# 暂停/恢复监控功能 - 实现说明

## 功能概述

实现了对后台实时余额监控的暂停/恢复控制，用户可以通过前端按钮停止/启动后台SQL查询，减少数据库负载。

---

## 后端实现 (api/handler.go)

### 1. 新增全局变量

```go
// 监控状态控制
isMonitoringPaused bool        // 监控暂停标志
monitoringMutex    sync.RWMutex // 读写锁，保护监控状态
```

**设计理由：**
- 使用 `RWMutex` 而非 `Mutex`：读多写少的场景，定时器每500ms读取状态，写操作只在用户点击时发生
- 布尔标志简单高效，无需复杂状态机

---

### 2. 新增API接口

#### POST /api/monitoring/pause
**功能：** 暂停实时监控
**返回：** `{ "code": 200, "data": { "status": "paused" } }`

#### POST /api/monitoring/resume
**功能：** 恢复实时监控
**返回：** `{ "code": 200, "data": { "status": "running" } }`

#### GET /api/monitoring/status
**功能：** 获取当前监控状态
**返回：** `{ "code": 200, "data": { "status": "running|paused" } }`

**实现要点：**
- 幂等性：重复暂停/恢复不报错，直接返回当前状态
- 广播机制：状态变更时通过WebSocket通知所有客户端
- 日志记录：每次状态变更都记录日志，便于追踪

---

### 3. 修改定时任务 (init函数)

```go
for range ticker.C {
    // 检查监控是否被暂停
    monitoringMutex.RLock()
    isPaused := isMonitoringPaused
    monitoringMutex.RUnlock()

    // 如果监控已暂停，跳过本次查询和广播
    if isPaused {
        continue
    }

    // 执行余额查询...
}
```

**关键设计：**
- 使用 `RLock()` 读锁，不阻塞其他读取操作
- 先读取状态再释放锁，避免长时间持锁
- `continue` 跳过执行，不消耗数据库资源

---

## 前端实现 (web/index.html)

### 1. 新增UI按钮

```html
<button class="btn-warning" id="monitorBtn" onclick="toggleMonitoring()">
    ⏸️ 暂停监控
</button>
```

**位置：** 控制面板底部，独立一行

---

### 2. 新增全局变量

```javascript
let isMonitoringPaused = false;  // 本地监控状态
```

---

### 3. 核心函数

#### toggleMonitoring()
**功能：** 切换监控状态（暂停 ↔ 恢复）
**流程：**
1. 禁用按钮，防止重复点击
2. 根据当前状态调用对应API
3. 成功后更新本地状态和按钮显示
4. 失败处理和日志记录

#### updateMonitoringButton(status)
**功能：** 更新按钮显示状态
**参数：** `status` - "running" 或 "paused"
**效果：**
- 暂停状态：绿色 "▶️ 恢复监控"
- 运行状态：黄色 "⏸️ 暂停监控"

---

### 4. WebSocket消息处理

新增消息类型：
```javascript
case 'monitoring_status':
    // 监控状态变更：更新监控按钮状态
    updateMonitoringButton(msg.data.status);
    break;
```

**作用：** 当其他客户端改变监控状态时，本客户端自动同步

---

### 5. 初始化加载

```javascript
window.onload = async () => {
    // ...
    // 获取初始监控状态
    const response = await fetch('/api/monitoring/status');
    const result = await response.json();
    updateMonitoringButton(result.data.status);
};
```

**保证：** 页面刷新后状态一致

---

## 使用说明

### 用户操作流程

1. **启动服务器**
   ```bash
   go run main.go
   ```

2. **访问页面**
   ```
   http://localhost:8080
   ```

3. **暂停监控**
   - 点击 "⏸️ 暂停监控" 按钮
   - 按钮变为绿色 "▶️ 恢复监控"
   - 控制台显示：`✅ 监控已暂停，SQL查询已停止`
   - 服务器日志：`实时监控已暂停`

4. **观察效果**
   - 打开浏览器开发者工具 → Network 标签
   - 查看服务器终端日志
   - **暂停后：** 不再有 `SELECT * FROM accounts` 查询
   - 图表停止更新，统计数据冻结

5. **恢复监控**
   - 点击 "▶️ 恢复监控" 按钮
   - 按钮变回黄色 "⏸️ 暂停监控"
   - SQL查询恢复，数据继续更新

---

## 技术亮点

### 1. 并发安全
- 使用 `sync.RWMutex` 保护共享状态
- 读多写少场景优化
- 避免竞态条件

### 2. 用户体验
- 按钮状态实时同步
- 多客户端状态一致
- 操作反馈清晰（日志+UI）

### 3. 资源优化
- 暂停时完全停止SQL查询
- 降低数据库CPU和IO
- 适合长时间查看历史数据场景

### 4. 代码规范
- 完整的函数注释（JSDoc风格）
- 清晰的变量命名
- 统一的错误处理
- 日志记录完备

---

## 测试验证

### 测试点

✅ 1. 暂停功能
- 点击暂停按钮
- 检查服务器日志不再有SQL查询
- 图表停止更新

✅ 2. 恢复功能
- 点击恢复按钮
- SQL查询恢复
- 图表继续更新

✅ 3. 状态持久化
- 暂停监控后刷新页面
- 检查按钮状态是否正确显示

✅ 4. 多客户端同步
- 打开两个浏览器窗口
- 一个窗口暂停
- 另一个窗口自动同步状态

✅ 5. 并发场景
- 监控暂停时发起攻击
- 验证扣款请求仍正常执行
- 只是实时监控停止

---

## 后续扩展

### 可能的优化方向

1. **监控频率调节**
   - 支持用户自定义查询间隔（100ms、500ms、1s）
   
2. **自动暂停**
   - 页面失去焦点时自动暂停
   - 节省资源

3. **监控统计**
   - 显示已暂停时长
   - 显示总查询次数

4. **性能指标**
   - 显示SQL响应时间
   - 显示数据库连接数

---

## 实现日期

2026-02-02

## 作者注释

此功能为 Zero Balance Loss 项目的第一阶段优化，成功实现了对后台监控任务的精确控制，为后续历史数据查看功能奠定了基础。代码遵循 Go 和 JavaScript 最佳实践，注释完整，易于维护和扩展。
