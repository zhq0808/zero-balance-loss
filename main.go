package main

import (
	"fmt"
	"log"

	"zero-balance-loss/api"
	"zero-balance-loss/config"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置文件
	if err := config.LoadConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库连接
	config.InitDB()
	defer config.CloseDB()

	// 创建 Gin 路由
	r := gin.Default()

	// 注册路由
	api.RegisterRoutes(r)

	// 启动服务
	port := fmt.Sprintf(":%d", config.GetConfig().Server.Port)
	log.Printf("Server starting on port %s...", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
