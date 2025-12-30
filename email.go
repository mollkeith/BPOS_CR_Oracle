package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	config *Config
}

func NewEmailService(config *Config) *EmailService {
	return &EmailService{
		config: config,
	}
}

// SendUpdateEmail 发送更新邮件
func (e *EmailService) SendUpdateEmail(monitor *Monitor, success bool, errMsg string) error {
	// 检查是否启用邮件
	if !e.config.Email.Enabled {
		return nil
	}

	if len(e.config.Email.To) == 0 {
		return nil // 没有配置收件人,跳过
	}

	subject := fmt.Sprintf("%s - 更新通知", e.config.Email.Subject)
	if !success {
		subject = fmt.Sprintf("%s - 更新失败", e.config.Email.Subject)
	}

	body := e.generateEmailBody(monitor, success, errMsg)

	msg := gomail.NewMessage()
	
	// 设置发件人
	from := e.config.Email.From.Address
	msg.SetHeader("From", from)
	
	msg.SetHeader("To", e.config.Email.To...)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", body)

	dialer := gomail.NewDialer(
		e.config.Email.SMTP.Host,
		e.config.Email.SMTP.Port,
		e.config.Email.From.Address,
		e.config.Email.From.Password,
	)

	// 设置TLS (gomail 默认使用 StartTLS,如果端口是465则使用SSL)
	if e.config.Email.SMTP.Port == 465 {
		dialer.SSL = true
	} else if e.config.Email.SMTP.TLS {
		dialer.SSL = false
		// gomail 默认会尝试 StartTLS
	}

	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent successfully to %d recipients", len(e.config.Email.To))
	return nil
}

func (e *EmailService) generateEmailBody(monitor *Monitor, success bool, errMsg string) string {
	var sb strings.Builder

	sb.WriteString("<html><body style='font-family: Arial, sans-serif;'>")
	sb.WriteString("<h2>BPoS & CR Monitor 更新报告</h2>")

	if success {
		sb.WriteString("<p style='color: green;'><strong>✓ 更新成功</strong></p>")
	} else {
		sb.WriteString("<p style='color: red;'><strong>✗ 更新失败</strong></p>")
		if errMsg != "" {
			sb.WriteString(fmt.Sprintf("<p><strong>错误信息:</strong> %s</p>", errMsg))
		}
	}

	sb.WriteString("<hr>")
	sb.WriteString("<h3>状态信息</h3>")
	sb.WriteString("<ul>")
	sb.WriteString(fmt.Sprintf("<li><strong>最后检查时间:</strong> %s</li>", monitor.lastCheckTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("<li><strong>最后更新时间:</strong> %s</li>", monitor.lastUpdateTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("<li><strong>今日是否有变更:</strong> %v</li>", monitor.hasChangesToday()))
	sb.WriteString(fmt.Sprintf("<li><strong>变更记录数:</strong> %d</li>", len(monitor.changeHistory)))
	sb.WriteString("</ul>")

	if len(monitor.changeHistory) > 0 {
		sb.WriteString("<hr>")
		sb.WriteString("<h3>最近变更记录</h3>")
		sb.WriteString("<table border='1' cellpadding='5' cellspacing='0' style='border-collapse: collapse;'>")
		sb.WriteString("<tr><th>时间</th><th>类型</th><th>描述</th></tr>")

		// 显示最近10条记录
		recentCount := 10
		if len(monitor.changeHistory) < recentCount {
			recentCount = len(monitor.changeHistory)
		}

		for i := len(monitor.changeHistory) - recentCount; i < len(monitor.changeHistory); i++ {
			record := monitor.changeHistory[i]
			sb.WriteString("<tr>")
			sb.WriteString(fmt.Sprintf("<td>%s</td>", record.Time.Format("2006-01-02 15:04:05")))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", record.Type))
			sb.WriteString(fmt.Sprintf("<td>%s</td>", record.Description))
			sb.WriteString("</tr>")
		}

		sb.WriteString("</table>")
	}

	sb.WriteString("<hr>")
	sb.WriteString(fmt.Sprintf("<p><small>报告生成时间: %s</small></p>", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("</body></html>")

	return sb.String()
}

