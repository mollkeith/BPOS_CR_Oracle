package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// 合约配置
	Contracts ContractConfig `yaml:"contracts"`
	
	// RPC 配置
	RPC RPCConfig `yaml:"rpc"`
	
	// 更新配置
	Update UpdateConfig `yaml:"update"`
	
	// 邮件配置
	Email EmailConfig `yaml:"email"`
	
	// 网页配置
	Web WebConfig `yaml:"web"`
	
	// 账户配置
	Account AccountConfig `yaml:"account"`
}

type ContractConfig struct {
	CRPoolAddress   string `yaml:"cr_pool_address"`   // CR信息合约地址
	BPoSPoolAddress string `yaml:"bpos_pool_address"` // BPoS节点信息合约地址
}

type RPCConfig struct {
	MainChain string `yaml:"main_chain"` // 主链RPC地址
	PGChain   string `yaml:"pg_chain"`  // PG链RPC地址
}

type UpdateConfig struct {
	Interval string `yaml:"interval"` // 更新间隔，如: "24h", "1h", "30m"
}

type EmailConfig struct {
	Enabled  bool     `yaml:"enabled"`  // 是否启用邮件通知
	SMTP     SMTPConfig `yaml:"smtp"`  // SMTP服务器配置
	To       []string `yaml:"to"`      // 收件人列表
	Subject  string   `yaml:"subject"` // 邮件主题前缀
}

type SMTPConfig struct {
	Host     string `yaml:"host"`     // SMTP服务器地址
	Port     int    `yaml:"port"`      // SMTP端口
	Username string `yaml:"username"`  // SMTP用户名
	Password string `yaml:"password"`  // SMTP密码
	From     string `yaml:"from"`      // 发件人邮箱
	FromName string `yaml:"from_name"` // 发件人名称
	TLS      bool   `yaml:"tls"`       // 是否使用TLS
}

type WebConfig struct {
	Enabled bool   `yaml:"enabled"`      // 是否启用网页生成
	Path    string `yaml:"output_path"` // 静态网页输出路径
}

type AccountConfig struct {
	PrivateKey string `yaml:"private_key"` // 用于签名交易的私钥(hex格式,不带0x前缀)
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证必需配置
	if config.Contracts.CRPoolAddress == "" {
		return nil, fmt.Errorf("contracts.cr_pool_address is required")
	}
	if config.Contracts.BPoSPoolAddress == "" {
		return nil, fmt.Errorf("contracts.bpos_pool_address is required")
	}
	if config.RPC.MainChain == "" {
		return nil, fmt.Errorf("rpc.main_chain is required")
	}
	if config.RPC.PGChain == "" {
		return nil, fmt.Errorf("rpc.pg_chain is required")
	}
	if config.Account.PrivateKey == "" {
		return nil, fmt.Errorf("account.private_key is required")
	}

	// 设置默认值
	if config.Update.Interval == "" {
		config.Update.Interval = "24h" // 默认 24 小时
	}
	if config.Web.Path == "" {
		config.Web.Path = "./web"
	}
	if !config.Web.Enabled {
		config.Web.Enabled = true // 默认启用网页生成
	}
	if config.Email.Subject == "" {
		config.Email.Subject = "BPoS & CR Monitor"
	}

	return &config, nil
}

func (c *Config) GetUpdateInterval() (time.Duration, error) {
	return time.ParseDuration(c.Update.Interval)
}

