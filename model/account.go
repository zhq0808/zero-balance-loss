package model

import (
	"time"
)

// Account 账户模型
type Account struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex" json:"user_id"`
	Balance   int64     `gorm:"column:balance;not null;default:0" json:"balance"` // 余额，单位：分
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (Account) TableName() string {
	return "accounts"
}
