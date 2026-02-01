package api

import (
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
)

// Stats 统计信息
type Stats struct {
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	StartTime     time.Time `json:"start_time"`
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

// RegisterRoutes 注册路由
func RegisterRoutes(r *gin.Engine) {
	// 静态文件
	r.Static("/static", "./web/static")
	r.LoadHTMLGlob("./web/*.html")

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// API 路由组
	api := r.Group("/api")
	{
		api.POST("/deduct", deductHandler)
		api.GET("/balance/:user_id", getBalanceHandler)
		api.POST("/reset", resetBalanceHandler)
		api.GET("/stats", getStatsHandler)
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

	// Step 2: 执行扣款
	resp, err := accountService.DeductBalance(&req, requestID)
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
func getStatsHandler(c *gin.Context) {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// wsHandler WebSocket处理
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

// 定期广播余额和统计信息
func init() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			balance, err := accountService.GetBalance(1)
			if err != nil {
				continue
			}

			statsMutex.Lock()
			currentStats := *stats
			statsMutex.Unlock()

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

	log.Println("Background balance broadcaster started")
}
