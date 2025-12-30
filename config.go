package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	// 合约配置
	Contracts ContractConfig `json:"contracts"`
	
	// RPC 配置
	RPC RPCConfig `json:"rpc"`
	
	// 更新配置
	Update UpdateConfig `json:"update"`
	
	// 邮件配置
	Email EmailConfig `json:"email"`
	
	// 网页配置
	Web WebConfig `json:"web"`
	
	// 账户配置
	Account AccountConfig `json:"account"`
}

type ContractConfig struct {
	CRPoolAddress   string `json:"cr_pool_address"`   // CR信息合约地址
	BPoSPoolAddress string `json:"bpos_pool_address"` // BPoS节点信息合约地址
}

type RPCConfig struct {
	MainChain string `json:"main_chain"` // 主链RPC地址
	PGChain   string `json:"pg_chain"`   // PG链RPC地址
}

type UpdateConfig struct {
	Interval string `json:"interval"` // 更新间隔，如: "24h", "1h", "30m"
}

type EmailConfig struct {
	Enabled bool       `json:"enabled"`  // 是否启用邮件通知
	From    FromConfig `json:"from"`     // 发件人配置
	To      []string   `json:"to"`       // 收件人列表
	SMTP    SMTPConfig `json:"smtp"`     // SMTP服务器配置
	Subject string     `json:"subject"`  // 邮件主题前缀
}

type FromConfig struct {
	Address  string `json:"address"`  // 发件人邮箱地址
	Password string `json:"password"` // 发件人邮箱密码
}

type SMTPConfig struct {
	Host string `json:"host"` // SMTP服务器地址
	Port int    `json:"port"` // SMTP端口
	TLS  bool   `json:"tls"`  // 是否使用TLS
}

type WebConfig struct {
	Enabled bool   `json:"enabled"`      // 是否启用网页生成
	Path    string `json:"output_path"`  // 静态网页输出路径
}

type AccountConfig struct {
	KeystorePath string `json:"keystore_path"` // keystore文件路径
	// Password 不再从配置文件读取，改为通过命令行参数 -p 输入
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
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
	if config.Account.KeystorePath == "" {
		return nil, fmt.Errorf("account.keystore_path is required")
	}
	// Password 通过命令行参数提供，不在这里验证

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

