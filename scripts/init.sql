-- 创建数据库
CREATE DATABASE IF NOT EXISTS zero_balance_loss DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE zero_balance_loss;

-- 创建账户表
CREATE TABLE IF NOT EXISTS accounts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键ID',
    user_id BIGINT NOT NULL UNIQUE COMMENT '用户ID',
    balance BIGINT NOT NULL DEFAULT 0 COMMENT '账户余额（单位：分）',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户表';

-- 插入测试数据：初始余额 1000.00 元 = 100000 分
INSERT INTO accounts (user_id, balance) VALUES (1, 100000)
ON DUPLICATE KEY UPDATE balance = 100000;

-- 查询验证
SELECT 
    id,
    user_id,
    balance,
    CONCAT(balance / 100, '元') AS balance_yuan,
    created_at,
    updated_at
FROM accounts;
