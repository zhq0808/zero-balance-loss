package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	serverURL  = "http://localhost:8080"
	userID     = 1
	amount     = 1000 // 10元 = 1000分
	goroutines = 100  // 并发协程数
	requests   = 10   // 每个协程的请求次数
)

type DeductRequest struct {
	UserID int64 `json:"user_id"`
	Amount int64 `json:"amount"`
}

type Response struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func main() {
	fmt.Println("===========================================")
	fmt.Println("  零余额丢失 - 并发攻击脚本")
	fmt.Println("===========================================")
	fmt.Printf("目标服务器: %s\n", serverURL)
	fmt.Printf("并发协程数: %d\n", goroutines)
	fmt.Printf("每协程请求: %d 次\n", requests)
	fmt.Printf("每次扣款: %.2f 元\n", float64(amount)/100)
	fmt.Printf("预期总扣款: %.2f 元\n", float64(goroutines*requests*amount)/100)
	fmt.Println("===========================================\n")

	// 获取初始余额
	initialBalance := getBalance()
	fmt.Printf("初始余额: %.2f 元\n\n", float64(initialBalance)/100)

	// 等待用户确认
	fmt.Print("按 Enter 键开始攻击...")
	fmt.Scanln()

	// 开始攻击
	fmt.Println("\n🚀 开始并发攻击...")
	startTime := time.Now()

	var wg sync.WaitGroup
	successCount := int64(0)
	failureCount := int64(0)
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < requests; j++ {
				success := deductBalance()
				mu.Lock()
				if success {
					successCount++
				} else {
					failureCount++
				}
				mu.Unlock()

				// 短暂延迟，模拟真实场景
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 获取最终余额
	time.Sleep(500 * time.Millisecond) // 等待所有请求完成
	finalBalance := getBalance()

	// 计算结果
	fmt.Println("\n===========================================")
	fmt.Println("  攻击完成 - 结果分析")
	fmt.Println("===========================================")
	fmt.Printf("耗时: %v\n", duration)
	fmt.Printf("成功请求: %d\n", successCount)
	fmt.Printf("失败请求: %d\n", failureCount)
	fmt.Printf("QPS: %.2f\n\n", float64(successCount+failureCount)/duration.Seconds())

	fmt.Printf("初始余额: %.2f 元\n", float64(initialBalance)/100)
	fmt.Printf("最终余额: %.2f 元\n", float64(finalBalance)/100)
	fmt.Printf("实际扣款: %.2f 元\n\n", float64(initialBalance-finalBalance)/100)

	expectedBalance := initialBalance - (successCount * amount)
	lostAmount := finalBalance - expectedBalance

	fmt.Printf("理论余额: %.2f 元\n", float64(expectedBalance)/100)
	fmt.Printf("💸 丢失金额: %.2f 元\n", float64(lostAmount)/100)
	fmt.Printf("📊 丢失比例: %.2f%%\n", float64(lostAmount)/float64(successCount*amount)*100)
	fmt.Println("===========================================")

	if lostAmount > 0 {
		fmt.Println("\n⚠️  检测到余额丢失! 并发问题已复现!")
		fmt.Println("原因: 多个请求同时读取余额，基于过期数据进行更新")
		fmt.Println("解决方案:")
		fmt.Println("  1. 使用数据库行锁 (SELECT FOR UPDATE)")
		fmt.Println("  2. 使用乐观锁 (版本号)")
		fmt.Println("  3. 使用分布式锁 (Redis)")
	} else {
		fmt.Println("\n✓ 未检测到余额丢失")
	}
}

// 获取余额
func getBalance() int64 {
	resp, err := http.Get(fmt.Sprintf("%s/api/balance/%d", serverURL, userID))
	if err != nil {
		log.Printf("获取余额失败: %v", err)
		return 0
	}
	defer resp.Body.Close()

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("解析响应失败: %v", err)
		return 0
	}

	if result.Code == 200 {
		if balance, ok := result.Data["balance"].(float64); ok {
			return int64(balance)
		}
	}

	return 0
}

// 扣减余额
func deductBalance() bool {
	req := DeductRequest{
		UserID: userID,
		Amount: amount,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return false
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/api/deduct", serverURL),
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return result.Code == 200
}
