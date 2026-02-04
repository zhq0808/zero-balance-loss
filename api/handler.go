package api

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"zero-balance-loss/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	accountService = service.NewAccountService()

	// WebSocket 连接管理
	wsClients  = make(map[*websocket.Conn]bool)
	wsMutex    sync.Mutex
	wsUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许跨域
		},
	}

	// 统计数据
	stats = &Stats{
		TotalRequests: 0,
		SuccessCount:  0,
		FailureCount:  0,
		StartTime:     time.Now(),
	}
	statsMutex sync.Mutex

	// 监控状态控制
	// isMonitoringPaused 表示是否暂停实时监控
	// 当为 true 时，后台定时任务将停止查询数据库和广播余额更新
	isMonitoringPaused bool
	monitoringMutex    sync.RWMutex // 使用读写锁，读多写少的场景

	// 历史数据存储
	// balanceHistory 存储余额的历史数据点，用于历史查看功能
	// 最多保存最近1000条记录，超过则删除最早的记录
	balanceHistory      []BalanceHistory
	balanceHistoryMutex sync.RWMutex
	maxHistorySize      = 1000 // 最大历史记录数

	// 执行模式控制
	// useLockMode 表示是否使用加锁模式
	// true: 使用互斥锁保护，无并发问题但性能略低
	// false: 不加锁，演示并发问题
	useLockMode bool
	modeMutex   sync.RWMutex
)

// Stats 统计信息
type Stats struct {
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	StartTime     time.Time `json:"start_time"`
}

// BalanceHistory 余额历史数据点
// 用于记录每个时间点的实际余额和理论余额，支持历史查看功能
type BalanceHistory struct {
	Timestamp       int64 `json:"timestamp"`        // 时间戳（毫秒）
	ActualBalance   int64 `json:"actual_balance"`   // 实际余额（分）
	ExpectedBalance int64 `json:"expected_balance"` // 理论余额（分）
}

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// WSMessage WebSocket消息
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// TraceEvent 时序追踪事件
type TraceEvent struct {
	RequestID  string `json:"request_id"`
	Step       int    `json:"step"`
	StepName   string `json:"step_name"`
	Balance    int64  `json:"balance"`
	Amount     int64  `json:"amount"`
	NewBalance int64  `json:"new_balance,omitempty"`
	Timestamp  int64  `json:"timestamp"`
}

// ConflictSnapshot 冲突快照 - 记录最近一次并发冲突的完整过程
type ConflictSnapshot struct {
	// Goroutine A (先到达的请求)
	RequestA_ID         string `json:"request_a_id"`
	RequestA_ReadTime   int64  `json:"request_a_read_time"`   // 纳秒时间戳
	RequestA_ReadValue  int64  `json:"request_a_read_value"`  // 读到的余额
	RequestA_CalcStart  int64  `json:"request_a_calc_start"`  // 计算开始时间
	RequestA_CalcEnd    int64  `json:"request_a_calc_end"`    // 计算结束时间
	RequestA_NewValue   int64  `json:"request_a_new_value"`   // 计算后的新值
	RequestA_WriteTime  int64  `json:"request_a_write_time"`  // 写入时间
	RequestA_WriteValue int64  `json:"request_a_write_value"` // 写入的值

	// Goroutine B (后到达的请求)
	RequestB_ID         string `json:"request_b_id"`
	RequestB_ReadTime   int64  `json:"request_b_read_time"`
	RequestB_ReadValue  int64  `json:"request_b_read_value"` // 关键：可能和A读到相同的旧值
	RequestB_CalcStart  int64  `json:"request_b_calc_start"`
	RequestB_CalcEnd    int64  `json:"request_b_calc_end"`
	RequestB_NewValue   int64  `json:"request_b_new_value"`
	RequestB_WriteTime  int64  `json:"request_b_write_time"`
	RequestB_WriteValue int64  `json:"request_b_write_value"` // 可能覆盖A的写入

	// 数据库视角
	DB_InitialValue int64 `json:"db_initial_value"` // 初始值
	DB_AfterA       int64 `json:"db_after_a"`       // A写入后的值
	DB_AfterB       int64 `json:"db_after_b"`       // B写入后的值
	DB_ExpectedB    int64 `json:"db_expected_b"`    // 如果B基于A的结果计算，应该得到的值
	IsConflict      bool  `json:"is_conflict"`      // 是否发生冲突（B读到了旧值）
	LostAmount      int64 `json:"lost_amount"`      // 丢失的金额
	UseLock         bool  `json:"use_lock"`         // 是否使用了锁
	CapturedAt      int64 `json:"captured_at"`      // 快照捕获时间
	Amount          int64 `json:"amount"`           // 每次扣款金额
}

var (
	// latestConflict 存储最近一次的冲突快照
	latestConflict      *ConflictSnapshot
	conflictSnapshotMux sync.RWMutex

	// pendingRequests 用于临时存储正在执行的请求信息
	pendingRequests    = make(map[string]*RequestTrace)
	pendingRequestsMux sync.RWMutex
)

// RequestTrace 单个请求的追踪信息
type RequestTrace struct {
	RequestID  string
	ReadTime   int64
	ReadValue  int64
	CalcStart  int64
	CalcEnd    int64
	NewValue   int64
	WriteTime  int64
	WriteValue int64
	Amount     int64
}

// RegisterRoutes 注册路由
// 注册所有HTTP路由和WebSocket端点
func RegisterRoutes(r *gin.Engine) {
	// 加载HTML模板
	r.LoadHTMLGlob("./web/*.html")

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// 冲突可视化器页面
	r.GET("/visualizer", func(c *gin.Context) {
		c.HTML(http.StatusOK, "conflict_visualizer.html", nil)
	})

	// API 路由组
	api := r.Group("/api")
	{
		// 余额扣减接口
		api.POST("/deduct", deductHandler)

		// 余额查询接口
		api.GET("/balance/:user_id", getBalanceHandler)

		// 重置余额接口
		api.POST("/reset", resetBalanceHandler)

		// 统计信息接口
		api.GET("/stats", getStatsHandler)

		// 监控控制接口
		monitoring := api.Group("/monitoring")
		{
			monitoring.POST("/pause", pauseMonitoringHandler)     // 暂停监控
			monitoring.POST("/resume", resumeMonitoringHandler)   // 恢复监控
			monitoring.GET("/status", getMonitoringStatusHandler) // 获取监控状态
		}

		// 执行模式控制接口
		mode := api.Group("/mode")
		{
			mode.POST("/switch", switchModeHandler)   // 切换执行模式
			mode.GET("/status", getModeStatusHandler) // 获取当前模式
		}

		// 历史数据接口
		api.GET("/balance/history", getBalanceHistoryHandler) // 获取历史数据

		// 冲突快照接口
		api.GET("/conflict/snapshot", getConflictSnapshotHandler) // 获取最近一次冲突快照
		api.POST("/conflict/clear", clearConflictSnapshotHandler) // 清除冲突快照
	}

	// WebSocket
	r.GET("/ws", wsHandler)
}

// deductHandler 余额扣减接口
func deductHandler(c *gin.Context) {
	var req service.DeductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 生成请求ID
	requestID := uuid.New().String()[:8]

	// 更新统计
	statsMutex.Lock()
	stats.TotalRequests++
	statsMutex.Unlock()

	// Step 1: 读取余额
	account, err := accountService.GetAccount(req.UserID)
	if err != nil {
		statsMutex.Lock()
		stats.FailureCount++
		statsMutex.Unlock()

		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: "account not found",
		})
		return
	}

	broadcastTrace(TraceEvent{
		RequestID: requestID,
		Step:      1,
		StepName:  "读取余额",
		Balance:   account.Balance,
		Amount:    req.Amount,
		Timestamp: time.Now().UnixMilli(),
	})

	// Step 2: 执行扣款（根据当前模式选择实现）
	modeMutex.RLock()
	currentMode := useLockMode
	modeMutex.RUnlock()

	var resp *service.DeductResponse
	if currentMode {
		// 加锁模式：使用互斥锁保护
		resp, err = accountService.DeductBalanceWithLock(&req, requestID)
	} else {
		// 无锁模式：演示并发问题
		resp, err = accountService.DeductBalance(&req, requestID)
	}

	if err != nil {
		statsMutex.Lock()
		stats.FailureCount++
		statsMutex.Unlock()

		broadcastTrace(TraceEvent{
			RequestID: requestID,
			Step:      2,
			StepName:  "扣款失败",
			Balance:   account.Balance,
			Amount:    req.Amount,
			Timestamp: time.Now().UnixMilli(),
		})

		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	statsMutex.Lock()
	stats.SuccessCount++
	statsMutex.Unlock()

	// Step 3: 写入完成
	broadcastTrace(TraceEvent{
		RequestID:  requestID,
		Step:       3,
		StepName:   "写入完成",
		Balance:    resp.OldBalance,
		Amount:     req.Amount,
		NewBalance: resp.Balance,
		Timestamp:  time.Now().UnixMilli(),
	})

	// 记录请求追踪信息，用于捕获冲突
	recordRequestTrace(requestID, resp, req.Amount)

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    resp,
	})
}

// getBalanceHandler 获取余额
func getBalanceHandler(c *gin.Context) {
	userID := int64(1) // 默认用户ID

	balance, err := accountService.GetBalance(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: "account not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"user_id": userID,
			"balance": balance,
		},
	})
}

// resetBalanceHandler 重置余额
func resetBalanceHandler(c *gin.Context) {
	var req struct {
		UserID  int64 `json:"user_id" binding:"required"`
		Balance int64 `json:"balance" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "invalid request",
		})
		return
	}

	if err := accountService.ResetBalance(req.UserID, req.Balance); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	// 重置统计
	statsMutex.Lock()
	stats = &Stats{
		TotalRequests: 0,
		SuccessCount:  0,
		FailureCount:  0,
		StartTime:     time.Now(),
	}
	statsMutex.Unlock()

	// 广播重置事件
	broadcast(WSMessage{
		Type: "reset",
		Data: map[string]interface{}{
			"user_id": req.UserID,
			"balance": req.Balance,
		},
		Timestamp: time.Now().UnixMilli(),
	})

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
	})
}

// getStatsHandler 获取统计信息
// 返回当前的请求统计数据，包括总请求数、成功数、失败数
func getStatsHandler(c *gin.Context) {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// pauseMonitoringHandler 暂停实时监控
// 暂停后台定时任务的余额查询和数据广播，减少数据库负载
// 适用场景：用户查看历史数据、系统维护、降低数据库压力
func pauseMonitoringHandler(c *gin.Context) {
	monitoringMutex.Lock()
	defer monitoringMutex.Unlock()

	// 如果已经是暂停状态，直接返回
	if isMonitoringPaused {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Message: "monitoring already paused",
			Data: map[string]interface{}{
				"status": "paused",
			},
		})
		return
	}

	// 设置暂停标志
	isMonitoringPaused = true
	log.Println("实时监控已暂停")

	// 广播监控状态变更
	broadcast(WSMessage{
		Type: "monitoring_status",
		Data: map[string]interface{}{
			"status": "paused",
		},
		Timestamp: time.Now().UnixMilli(),
	})

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "monitoring paused successfully",
		Data: map[string]interface{}{
			"status": "paused",
		},
	})
}

// resumeMonitoringHandler 恢复实时监控
// 恢复后台定时任务，继续查询和广播余额数据
func resumeMonitoringHandler(c *gin.Context) {
	monitoringMutex.Lock()
	defer monitoringMutex.Unlock()

	// 如果已经是运行状态，直接返回
	if !isMonitoringPaused {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Message: "monitoring already running",
			Data: map[string]interface{}{
				"status": "running",
			},
		})
		return
	}

	// 取消暂停标志
	isMonitoringPaused = false
	log.Println("实时监控已恢复")

	// 广播监控状态变更
	broadcast(WSMessage{
		Type: "monitoring_status",
		Data: map[string]interface{}{
			"status": "running",
		},
		Timestamp: time.Now().UnixMilli(),
	})

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "monitoring resumed successfully",
		Data: map[string]interface{}{
			"status": "running",
		},
	})
}

// switchModeHandler 切换执行模式（加锁/不加锁）
// 请求体: {"use_lock": true/false}
func switchModeHandler(c *gin.Context) {
	var req struct {
		UseLock bool `json:"use_lock"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "invalid request",
		})
		return
	}

	modeMutex.Lock()
	useLockMode = req.UseLock
	modeMutex.Unlock()

	mode := "unlocked"
	if req.UseLock {
		mode = "locked"
	}

	log.Printf("执行模式已切换: %s", mode)

	// 广播模式变更
	broadcast(WSMessage{
		Type: "mode_changed",
		Data: map[string]interface{}{
			"mode":     mode,
			"use_lock": req.UseLock,
		},
		Timestamp: time.Now().UnixMilli(),
	})

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "mode switched successfully",
		Data: map[string]interface{}{
			"mode":     mode,
			"use_lock": req.UseLock,
		},
	})
}

// getModeStatusHandler 获取当前执行模式
func getModeStatusHandler(c *gin.Context) {
	modeMutex.RLock()
	defer modeMutex.RUnlock()

	mode := "unlocked"
	if useLockMode {
		mode = "locked"
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"mode":     mode,
			"use_lock": useLockMode,
		},
	})
}

// getBalanceHistoryHandler 获取历史余额数据
// 支持通过查询参数指定时间范围：?start=timestamp&end=timestamp
// 如果不指定参数，返回所有历史数据
func getBalanceHistoryHandler(c *gin.Context) {
	// 获取查询参数
	startStr := c.Query("start")
	endStr := c.Query("end")

	balanceHistoryMutex.RLock()
	defer balanceHistoryMutex.RUnlock()

	// 如果没有指定时间范围，返回所有数据
	if startStr == "" && endStr == "" {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Message: "success",
			Data:    balanceHistory,
		})
		return
	}

	// 解析时间戳参数
	var startTime, endTime int64
	var err error

	if startStr != "" {
		startTime, err = parseTimestamp(startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Message: "invalid start timestamp",
			})
			return
		}
	}

	if endStr != "" {
		endTime, err = parseTimestamp(endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, Response{
				Code:    400,
				Message: "invalid end timestamp",
			})
			return
		}
	}

	// 过滤时间范围内的数据
	var filteredData []BalanceHistory
	for _, item := range balanceHistory {
		// 如果只指定了开始时间
		if startStr != "" && endStr == "" {
			if item.Timestamp >= startTime {
				filteredData = append(filteredData, item)
			}
		} else if startStr == "" && endStr != "" {
			// 如果只指定了结束时间
			if item.Timestamp <= endTime {
				filteredData = append(filteredData, item)
			}
		} else {
			// 如果指定了开始和结束时间
			if item.Timestamp >= startTime && item.Timestamp <= endTime {
				filteredData = append(filteredData, item)
			}
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    filteredData,
	})
}

// parseTimestamp 解析时间戳字符串
// 支持毫秒级时间戳
func parseTimestamp(s string) (int64, error) {
	var timestamp int64
	_, err := fmt.Sscanf(s, "%d", &timestamp)
	return timestamp, err
}

// addBalanceHistory 添加余额历史记录
// 当历史记录超过最大限制时，删除最早的记录
func addBalanceHistory(actualBalance, expectedBalance int64) {
	balanceHistoryMutex.Lock()
	defer balanceHistoryMutex.Unlock()

	// 创建新的历史记录
	history := BalanceHistory{
		Timestamp:       time.Now().UnixMilli(),
		ActualBalance:   actualBalance,
		ExpectedBalance: expectedBalance,
	}

	// 添加到切片
	balanceHistory = append(balanceHistory, history)

	// 如果超过最大限制，删除最早的记录
	if len(balanceHistory) > maxHistorySize {
		// 使用切片操作删除第一个元素
		balanceHistory = balanceHistory[1:]
	}
}

// getMonitoringStatusHandler 获取监控状态
// 返回当前监控是运行中还是已暂停
func getMonitoringStatusHandler(c *gin.Context) {
	monitoringMutex.RLock()
	defer monitoringMutex.RUnlock()

	status := "running"
	if isMonitoringPaused {
		status = "paused"
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"status": status,
		},
	})
}

// wsHandler WebSocket处理
// 处理WebSocket连接，用于实时推送数据到前端
func wsHandler(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	wsMutex.Lock()
	wsClients[conn] = true
	wsMutex.Unlock()

	log.Printf("WebSocket client connected, total clients: %d", len(wsClients))

	// 发送初始状态
	balance, _ := accountService.GetBalance(1)
	conn.WriteJSON(WSMessage{
		Type: "init",
		Data: map[string]interface{}{
			"balance": balance,
			"stats":   stats,
		},
		Timestamp: time.Now().UnixMilli(),
	})

	// 保持连接
	defer func() {
		wsMutex.Lock()
		delete(wsClients, conn)
		wsMutex.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected, remaining clients: %d", len(wsClients))
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// broadcast 广播消息到所有WebSocket客户端
func broadcast(msg WSMessage) {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	for client := range wsClients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			delete(wsClients, client)
		}
	}
}

// broadcastTrace 广播追踪事件
func broadcastTrace(event TraceEvent) {
	broadcast(WSMessage{
		Type:      "trace",
		Data:      event,
		Timestamp: time.Now().UnixMilli(),
	})
}

// init 初始化函数
// 启动后台定时任务，每500ms广播一次余额和统计数据
// 该任务会检查监控状态，如果被暂停则跳过执行
func init() {
	go func() {
		// 创建定时器，每500毫秒触发一次
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		log.Println("后台余额监控任务已启动（500ms间隔）")

		for range ticker.C {
			// 检查监控是否被暂停
			monitoringMutex.RLock()
			isPaused := isMonitoringPaused
			monitoringMutex.RUnlock()

			// 如果监控已暂停，跳过本次查询和广播
			if isPaused {
				continue
			}

			// 查询当前余额（这里会执行 SELECT 查询）
			balance, err := accountService.GetBalance(1)
			if err != nil {
				log.Printf("查询余额失败: %v", err)
				continue
			}

			// 获取当前统计数据
			statsMutex.Lock()
			currentStats := *stats
			statsMutex.Unlock()

			// 计算理论余额（用于对比）
			// 理论余额 = 初始余额 - (成功请求数 × 扣款金额)
			// 这里假设每次扣款金额一致，实际项目中可能需要更复杂的计算
			expectedBalance := balance // 简化处理，可以根据实际业务调整

			// 记录历史数据点
			addBalanceHistory(balance, expectedBalance)

			// 通过WebSocket广播余额更新给所有连接的客户端
			broadcast(WSMessage{
				Type: "balance_update",
				Data: map[string]interface{}{
					"balance": balance,
					"stats":   currentStats,
				},
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}()
}

// recordRequestTrace 记录请求追踪信息
func recordRequestTrace(requestID string, resp *service.DeductResponse, amount int64) {
	pendingRequestsMux.Lock()
	defer pendingRequestsMux.Unlock()

	timeline := resp.Timeline

	// 创建请求追踪
	trace := &RequestTrace{
		RequestID:  requestID,
		ReadTime:   timeline.ReadStart,
		ReadValue:  resp.OldBalance,
		CalcStart:  timeline.ComputeStart,
		CalcEnd:    timeline.ComputeEnd,
		NewValue:   resp.Balance,
		WriteTime:  timeline.WriteStart,
		WriteValue: resp.Balance,
		Amount:     amount,
	}

	pendingRequests[requestID] = trace

	// 尝试捕获冲突：检查是否有两个请求读到了相同的值
	tryCapture冲突Snapshot()

	// 清理旧的追踪记录（保留最近的10个）
	if len(pendingRequests) > 10 {
		// 简单清理：找到最旧的删除
		var oldestID string
		var oldestTime int64 = 999999999999999999
		for id, t := range pendingRequests {
			if t.ReadTime < oldestTime {
				oldestTime = t.ReadTime
				oldestID = id
			}
		}
		delete(pendingRequests, oldestID)
	}
}

// tryCapture冲突Snapshot 尝试捕获冲突快照
func tryCapture冲突Snapshot() {
	// 获取当前模式
	modeMutex.RLock()
	currentLockMode := useLockMode
	modeMutex.RUnlock()

	// 找出所有读到相同值的请求
	valueMap := make(map[int64][]*RequestTrace)
	for _, trace := range pendingRequests {
		valueMap[trace.ReadValue] = append(valueMap[trace.ReadValue], trace)
	}

	// 检查是否有至少两个请求读到了相同的值
	for readValue, traces := range valueMap {
		if len(traces) >= 2 {
			// 找到冲突！选择前两个请求作为A和B
			traceA := traces[0]
			traceB := traces[1]

			// 确保A是先读取的
			if traceA.ReadTime > traceB.ReadTime {
				traceA, traceB = traceB, traceA
			}

			// 构建冲突快照
			snapshot := &ConflictSnapshot{
				RequestA_ID:         traceA.RequestID,
				RequestA_ReadTime:   traceA.ReadTime,
				RequestA_ReadValue:  traceA.ReadValue,
				RequestA_CalcStart:  traceA.CalcStart,
				RequestA_CalcEnd:    traceA.CalcEnd,
				RequestA_NewValue:   traceA.NewValue,
				RequestA_WriteTime:  traceA.WriteTime,
				RequestA_WriteValue: traceA.WriteValue,

				RequestB_ID:         traceB.RequestID,
				RequestB_ReadTime:   traceB.ReadTime,
				RequestB_ReadValue:  traceB.ReadValue,
				RequestB_CalcStart:  traceB.CalcStart,
				RequestB_CalcEnd:    traceB.CalcEnd,
				RequestB_NewValue:   traceB.NewValue,
				RequestB_WriteTime:  traceB.WriteTime,
				RequestB_WriteValue: traceB.WriteValue,

				DB_InitialValue: readValue,
				DB_AfterA:       traceA.WriteValue,
				DB_AfterB:       traceB.WriteValue,
				DB_ExpectedB:    traceA.WriteValue - traceB.Amount, // 如果B基于A的结果计算
				IsConflict:      true,                              // B读到了旧值
				LostAmount:      traceA.WriteValue - traceB.WriteValue,
				UseLock:         currentLockMode,
				CapturedAt:      time.Now().UnixMilli(),
				Amount:          traceA.Amount,
			}

			// 保存快照
			conflictSnapshotMux.Lock()
			latestConflict = snapshot
			conflictSnapshotMux.Unlock()

			log.Printf("⚠️ 捕获冲突快照: A[%s] vs B[%s], 读取值=%d, A写入=%d, B写入=%d, 丢失=%d",
				traceA.RequestID, traceB.RequestID, readValue,
				traceA.WriteValue, traceB.WriteValue, snapshot.LostAmount)

			// 找到一个冲突就够了
			break
		}
	}
}

// getConflictSnapshotHandler 获取最近一次冲突快照
func getConflictSnapshotHandler(c *gin.Context) {
	conflictSnapshotMux.RLock()
	snapshot := latestConflict
	conflictSnapshotMux.RUnlock()

	if snapshot == nil {
		c.JSON(http.StatusOK, Response{
			Code:    200,
			Message: "no conflict captured yet",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    snapshot,
	})
}

// clearConflictSnapshotHandler 清除冲突快照
func clearConflictSnapshotHandler(c *gin.Context) {
	conflictSnapshotMux.Lock()
	latestConflict = nil
	conflictSnapshotMux.Unlock()

	pendingRequestsMux.Lock()
	pendingRequests = make(map[string]*RequestTrace)
	pendingRequestsMux.Unlock()

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "conflict snapshot cleared",
	})
}
