package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	password := flag.String("p", "", "Keystore password (optional, will prompt if not provided)")
	flag.Parse()

	// 如果密码未提供，提示用户输入
	var passwd string
	if *password == "" {
		fmt.Print("Enter keystore password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatalf("Failed to read password: %v", err)
		}
		fmt.Println() // 换行
		passwd = string(passwordBytes)
		if passwd == "" {
			log.Fatal("Error: keystore password cannot be empty.")
		}
	} else {
		passwd = *password
	}

	// 加载配置
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建监控器
	monitor, err := NewMonitor(config, passwd)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	// 创建邮件服务
	emailService := NewEmailService(config)

	// 获取更新间隔
	interval, err := config.GetUpdateInterval()
	if err != nil {
		log.Fatalf("Invalid update interval: %v", err)
	}

	// 获取并打印钱包信息
	walletAddress, balance, err := monitor.GetWalletInfo()
	if err != nil {
		log.Printf("Warning: failed to get wallet info: %v", err)
	} else {
		// 将余额从 Wei 转换为 ELA (假设 1 ELA = 10^18 Wei，类似以太坊)
		elaBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
		log.Printf("Wallet Address: %s", walletAddress)
		log.Printf("Wallet Balance: %s ELA (Wei: %s)", elaBalance.Text('f', 8), balance.String())
	}

	log.Printf("Monitor started with update interval: %v", interval)
	log.Printf("CR Pool Address: %s", config.Contracts.CRPoolAddress)
	log.Printf("BPoS Pool Address: %s", config.Contracts.BPoSPoolAddress)
	log.Printf("Email notification: %v", config.Email.Enabled)
	log.Printf("Web page generation: %v", config.Web.Enabled)

	// 启动 Web 服务器（先启动，即使没有数据也能访问）
	if err := monitor.StartWebServer(); err != nil {
		log.Printf("Failed to start web server: %v", err)
	}

	// 立即执行一次检查和更新（会在交易确认后更新 web 页面）
	log.Println("Performing initial check...")
	if err := performCheckAndUpdate(monitor, emailService); err != nil {
		log.Printf("Initial check failed: %v", err)
		// 即使失败也生成一次初始网页数据
		if err := monitor.GenerateWebPage(); err != nil {
			log.Printf("Failed to generate initial web page: %v", err)
		}
	}

	// 设置定时器
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 主循环
	for {
		select {
		case <-ticker.C:
			log.Println("Scheduled check triggered...")
			// performCheckAndUpdate 内部会在交易确认后更新 web 页面
			if err := performCheckAndUpdate(monitor, emailService); err != nil {
				log.Printf("Scheduled check failed: %v", err)
			}

		case sig := <-sigChan:
			log.Printf("Received signal: %v, shutting down...", sig)
			return
		}
	}
}

func performCheckAndUpdate(monitor *Monitor, emailService *EmailService) error {
	err := monitor.CheckAndUpdate()

	// 发送邮件通知
	emailErr := emailService.SendUpdateEmail(monitor, err == nil, "")
	if emailErr != nil {
		log.Printf("Failed to send email: %v", emailErr)
	}

	if err != nil {
		// 发送错误邮件
		emailService.SendUpdateEmail(monitor, false, err.Error())
		return err
	}

	// 交易确认后，更新 web 页面数据
	if err := monitor.GenerateWebPage(); err != nil {
		log.Printf("Failed to update web page data: %v", err)
	}

	return nil
}
