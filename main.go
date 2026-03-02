package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zero-balance-loss/api"
	"zero-balance-loss/config"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 加载配置文件
	if err := config.LoadConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化数据库连接
	cfg := config.GetConfig()
	config.InitDB()

	// 3. 创建路由并注册
	r := gin.Default()
	api.RegisterRoutes(r)

	// 4. 启动后台监控任务（可控的，能被优雅停止）
	api.StartBackgroundMonitoring()

	// 5. 创建 HTTP Server（不用 gin.Run，这样才能优雅关闭）
	port := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    port,
		Handler: r,
	}

	// 6. 在独立 goroutine 中启动服务器，不阻塞主流程
	go func() {
		log.Printf("Server starting on port %s...", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 1. 创建一个通道，用来接收信号
	sigChan := make(chan os.Signal, 1)
	// 2. 告诉系统：我要拦截 SIGINT (Ctrl+C) 和 SIGTERM，不要执行 Default Action
	// 这里的 Notify 就是在注册我们关心的信号，当这些信号发生时，系统会把它们发送到 sigChan 这个通道中
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// 3. 阻塞在这里，直到信号传过来
	// 这就像是工人在等管家的纸条
	sig := <-sigChan

	// 4. 收到纸条后，执行我们自定义的“优雅逻辑” (Signal Handler 的一部分)
	fmt.Println("收到信号:", sig, "正在清理资源...")

	// 8. 执行优雅关闭
	gracefulShutdown(srv)
}

// gracefulShutdown 按顺序关闭所有资源
// 顺序：HTTP → WebSocket → 后台任务 → 数据库
// 原则：先停止接受新请求，再等待进行中的操作完成，最后释放资源
func gracefulShutdown(srv *http.Server) {
	// Step 1: 停止接受新 HTTP 请求，等待已有请求完成（最多30秒）
	// 保证正在处理的扣款请求不会被强制中断，避免数据不一致
	log.Println("[1/4] 停止 HTTP 服务器...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP 关闭超时: %v", err)
	} else {
		log.Println("[1/4] HTTP 服务器已停止")
	}

	// Step 2: 通知 WebSocket 客户端，给前端3秒时间收到消息
	// 避免前端显示"连接断开"错误，而是显示友好的维护提示
	log.Println("[2/4] 通知 WebSocket 客户端...")
	api.NotifyShutdownToWebSockets()
	time.Sleep(3 * time.Second)
	api.CloseAllWebSockets()
	log.Println("[2/4] WebSocket 连接已全部关闭")

	// Step 3: 停止后台监控任务
	// 等待当前正在执行的数据库查询完成，避免连接泄漏
	log.Println("[3/4] 停止后台监控任务...")
	api.StopBackgroundMonitoring()
	log.Println("[3/4] 后台任务已停止")

	// Step 4: 关闭数据库连接池
	// 必须最后关闭，因为前面的步骤可能还需要数据库
	log.Println("[4/4] 关闭数据库连接...")
	config.CloseDB()
	log.Println("[4/4] 数据库连接已关闭")

	log.Println("优雅关闭完成")
}
