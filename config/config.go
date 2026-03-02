package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Kafka    KafkaConfig    `yaml:"kafka"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Brokers       []string          `yaml:"brokers"`
	Topics        map[string]string `yaml:"topics"`
	ConsumerGroup string            `yaml:"consumer_group"`
}

var AppConfig *Config

// LoadConfig 加载配置文件，并用环境变量覆盖敏感配置
// 优先级：环境变量 > config.yaml
// 这样 config.yaml 可以安全提交到 Git，敏感信息通过环境变量注入
func LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// 用环境变量覆盖配置（生产环境通过环境变量注入，不用改配置文件）
	if mode := os.Getenv("SERVER_MODE"); mode != "" {
		AppConfig.Server.Mode = mode
	}
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		AppConfig.Database.Password = dbPassword
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		AppConfig.Database.Host = dbHost
	}

	log.Printf("Config loaded: mode=%s, db_host=%s", AppConfig.Server.Mode, AppConfig.Database.Host)
	return nil
}

// GetConfig 获取配置实例
func GetConfig() *Config {
	return AppConfig
}
