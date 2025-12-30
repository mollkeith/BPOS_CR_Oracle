package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	password := flag.String("p", "", "Keystore password (required)")
	flag.Parse()

	// 检查密码是否提供
	if *password == "" {
		log.Fatal("Error: keystore password is required. Use -p flag to provide password.")
	}

	// 加载配置
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 创建监控器
	monitor, err := NewMonitor(config, *password)
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

	log.Printf("Monitor started with update interval: %v", interval)
	log.Printf("CR Pool Address: %s", config.Contracts.CRPoolAddress)
	log.Printf("BPoS Pool Address: %s", config.Contracts.BPoSPoolAddress)
	log.Printf("Email notification: %v", config.Email.Enabled)
	log.Printf("Web page generation: %v", config.Web.Enabled)

	// 立即执行一次检查和更新
	log.Println("Performing initial check...")
	if err := performCheckAndUpdate(monitor, emailService); err != nil {
		log.Printf("Initial check failed: %v", err)
	}

	// 生成初始网页
	if err := monitor.GenerateWebPage(); err != nil {
		log.Printf("Failed to generate initial web page: %v", err)
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
			if err := performCheckAndUpdate(monitor, emailService); err != nil {
				log.Printf("Scheduled check failed: %v", err)
			}

			// 生成网页
			if err := monitor.GenerateWebPage(); err != nil {
				log.Printf("Failed to generate web page: %v", err)
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

	return nil
}

