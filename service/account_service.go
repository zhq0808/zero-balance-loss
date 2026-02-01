package service

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"zero-balance-loss/config"
	"zero-balance-loss/model"
)

// 全局互斥锁，用于加锁模式
var accountMutex sync.Mutex

// DeductRequest 扣款请求
type DeductRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
	Amount int64 `json:"amount" binding:"required"` // 单位：分
}

// DeductResponse 扣款响应
type DeductResponse struct {
	UserID     int64  `json:"user_id"`
	Balance    int64  `json:"balance"`     // 单位：分
	OldBalance int64  `json:"old_balance"` // 单位：分
	RequestID  string `json:"request_id"`
}

// AccountService 账户服务
type AccountService struct{}

// NewAccountService 创建账户服务实例
func NewAccountService() *AccountService {
	return &AccountService{}
}

// GetAccount 获取账户信息
func (s *AccountService) GetAccount(userID int64) (*model.Account, error) {
	db := config.GetDB()
	var account model.Account

	if err := db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

// DeductBalance 扣减余额（故意不加锁，演示并发问题）
// 这是一个有问题的实现，会导致并发场景下的余额丢失
func (s *AccountService) DeductBalance(req *DeductRequest, requestID string) (*DeductResponse, error) {
	db := config.GetDB()

	// 步骤1: 查询当前余额
	log.Printf("[%s] Step 1: 读取账户 user_id=%d", requestID, req.UserID)
	account, err := s.GetAccount(req.UserID)
	if err != nil {
		return nil, err
	}

	oldBalance := account.Balance
	log.Printf("[%s] Step 2: 当前余额=%d分 (%.2f元)", requestID, oldBalance, float64(oldBalance)/100)

	// 步骤2: 检查余额是否充足
	if account.Balance < req.Amount {
		return nil, errors.New("insufficient balance")
	}

	// 模拟一些处理时间，增加并发冲突的概率
	time.Sleep(10 * time.Millisecond)

	// 步骤3: 计算新余额
	newBalance := account.Balance - req.Amount
	log.Printf("[%s] Step 3: 计算新余额=%d分 (%.2f元)", requestID, newBalance, float64(newBalance)/100)

	// 步骤4: 更新数据库（问题所在：基于读取时的旧值更新，没有任何并发保护）
	// 这里使用 Update 而不是事务，会导致 Lost Update 问题
	result := db.Model(&model.Account{}).
		Where("user_id = ?", req.UserID).
		Update("balance", newBalance)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to update balance: %w", result.Error)
	}

	log.Printf("[%s] Step 4: 更新成功，影响行数=%d", requestID, result.RowsAffected)

	return &DeductResponse{
		UserID:     req.UserID,
		Balance:    newBalance,
		OldBalance: oldBalance,
		RequestID:  requestID,
	}, nil
}

// DeductBalanceWithLock 扣减余额（加锁版本，解决并发问题）
// 使用互斥锁保护临界区，确保并发安全
func (s *AccountService) DeductBalanceWithLock(req *DeductRequest, requestID string) (*DeductResponse, error) {
	// 🔒 加锁：进入临界区
	accountMutex.Lock()
	defer accountMutex.Unlock() // 确保函数返回时释放锁

	db := config.GetDB()

	// 步骤1: 查询当前余额
	log.Printf("[%s] 🔒 [LOCKED] Step 1: 读取账户 user_id=%d", requestID, req.UserID)
	account, err := s.GetAccount(req.UserID)
	if err != nil {
		return nil, err
	}

	oldBalance := account.Balance
	log.Printf("[%s] 🔒 [LOCKED] Step 2: 当前余额=%d分 (%.2f元)", requestID, oldBalance, float64(oldBalance)/100)

	// 步骤2: 检查余额是否充足
	if account.Balance < req.Amount {
		return nil, errors.New("insufficient balance")
	}

	// 模拟一些处理时间
	time.Sleep(10 * time.Millisecond)

	// 步骤3: 计算新余额
	newBalance := account.Balance - req.Amount
	log.Printf("[%s] 🔒 [LOCKED] Step 3: 计算新余额=%d分 (%.2f元)", requestID, newBalance, float64(newBalance)/100)

	// 步骤4: 更新数据库（在锁的保护下，安全更新）
	result := db.Model(&model.Account{}).
		Where("user_id = ?", req.UserID).
		Update("balance", newBalance)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to update balance: %w", result.Error)
	}

	log.Printf("[%s] 🔒 [LOCKED] Step 4: 更新成功，影响行数=%d", requestID, result.RowsAffected)

	return &DeductResponse{
		UserID:     req.UserID,
		Balance:    newBalance,
		OldBalance: oldBalance,
		RequestID:  requestID,
	}, nil
}

// GetBalance 获取账户余额
func (s *AccountService) GetBalance(userID int64) (int64, error) {
	account, err := s.GetAccount(userID)
	if err != nil {
		return 0, err
	}
	return account.Balance, nil
}

// ResetBalance 重置账户余额（用于测试）
func (s *AccountService) ResetBalance(userID int64, balance int64) error {
	db := config.GetDB()
	result := db.Model(&model.Account{}).
		Where("user_id = ?", userID).
		Update("balance", balance)

	if result.Error != nil {
		return fmt.Errorf("failed to reset balance: %w", result.Error)
	}

	log.Printf("重置账户余额: user_id=%d, balance=%d分 (%.2f元)", userID, balance, float64(balance)/100)
	return nil
}
