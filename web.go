package main

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"
)

type WebData struct {
	CRNodes         []CRNodeDisplay
	BPoSNodes       []BPoSNodeDisplay
	HasChangesToday bool
	LastCheckTime   string
	LastUpdateTime  string
	ChangeHistory   []ChangeRecordDisplay
	UpdateTime      string
}

type CRNodeDisplay struct {
	NickName       string
	OwnerPublicKey string
	DPoSPublicKey  string
}

type BPoSNodeDisplay struct {
	NickName       string
	OwnerPublicKey string
	DPoSPublicKey  string
	Votes          string
	Rank           int
	Tier           string // "Tier1" (前25名) or "Tier2" (25名之后)
	SelectionProb  string // 选中概率
}

type ChangeRecordDisplay struct {
	Time        string
	Type        string
	Description string
}

// GenerateWebPage 更新网页数据（用于 HTTP 服务器）
func (m *Monitor) GenerateWebPage() error {
	return m.generateWebPageInternal()
}

// GenerateWebPageWithRetry 带重试机制的更新网页数据
func (m *Monitor) GenerateWebPageWithRetry(maxRetries int, retryDelay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			log.Printf("Retrying web page update (attempt %d/%d)...", i+1, maxRetries)
			time.Sleep(retryDelay)
		}

		err := m.generateWebPageInternal()
		if err == nil {
			if i > 0 {
				log.Printf("Web page update succeeded on retry %d", i+1)
			}
			return nil
		}
		lastErr = err
		log.Printf("Web page update attempt %d failed: %v", i+1, err)
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// generateWebPageInternal 内部实现：更新网页数据
func (m *Monitor) generateWebPageInternal() error {
	// 检查是否启用网页生成
	if !m.config.Web.Enabled {
		return nil
	}

	// 获取当前数据
	crNodes, err := m.crContract.GetAllNodes()
	if err != nil {
		return fmt.Errorf("failed to get CR nodes: %w", err)
	}

	bposNodes, err := m.bposContract.GetAllNodes()
	if err != nil {
		return fmt.Errorf("failed to get BPoS nodes: %w", err)
	}

	log.Printf("Updating web page: CR nodes=%d, BPoS nodes=%d", len(crNodes), len(bposNodes))

	// 打印 BPoS 节点详情（用于调试）
	for i, node := range bposNodes {
		if i < 5 || i >= len(bposNodes)-2 { // 只打印前5个和后2个
			log.Printf("  BPoS[%d]: Nickname=%s, Votes=%s", i, node.NickName, node.Votes.String())
		} else if i == 5 {
			log.Printf("  ... (showing first 5 and last 2 nodes)")
		}
	}

	// 转换为显示格式
	crDisplay := make([]CRNodeDisplay, len(crNodes))
	for i, node := range crNodes {
		crDisplay[i] = CRNodeDisplay{
			NickName:       node.NickName,
			OwnerPublicKey: hex.EncodeToString(node.OwnerPublicKey),
			DPoSPublicKey:  hex.EncodeToString(node.DPoSPublicKey),
		}
	}

	// BPoS 节点按 votes 排序
	bposDisplay := make([]BPoSNodeDisplay, len(bposNodes))
	for i, node := range bposNodes {
		votesStr := formatVotes(node.Votes)
		tier := "Tier2"
		prob := "随机选4个"
		if i < 25 {
			tier = "Tier1"
			prob = "随机选20个"
		}

		bposDisplay[i] = BPoSNodeDisplay{
			NickName:       node.NickName,
			OwnerPublicKey: hex.EncodeToString(node.OwnerPublicKey),
			DPoSPublicKey:  hex.EncodeToString(node.DPoSPublicKey),
			Votes:          votesStr,
			Rank:           i + 1,
			Tier:           tier,
			SelectionProb:  prob,
		}
	}

	// 按 votes 降序排序
	sort.Slice(bposDisplay, func(i, j int) bool {
		votesI, _ := parseVotesFromString(bposDisplay[i].Votes)
		votesJ, _ := parseVotesFromString(bposDisplay[j].Votes)
		return votesI.Cmp(votesJ) > 0
	})

	// 更新排名
	for i := range bposDisplay {
		bposDisplay[i].Rank = i + 1
		if i < 25 {
			bposDisplay[i].Tier = "Tier1"
			bposDisplay[i].SelectionProb = "随机选20个"
		} else {
			bposDisplay[i].Tier = "Tier2"
			bposDisplay[i].SelectionProb = "随机选4个"
		}
	}

	// 转换变更历史
	changeHistoryDisplay := make([]ChangeRecordDisplay, len(m.changeHistory))
	for i, record := range m.changeHistory {
		changeHistoryDisplay[i] = ChangeRecordDisplay{
			Time:        record.Time.Format("2006-01-02 15:04:05"),
			Type:        record.Type,
			Description: record.Description,
		}
	}

	// 准备数据
	data := WebData{
		CRNodes:         crDisplay,
		BPoSNodes:       bposDisplay,
		HasChangesToday: m.hasChangesToday(),
		LastCheckTime:   m.lastCheckTime.Format("2006-01-02 15:04:05"),
		LastUpdateTime:  m.lastUpdateTime.Format("2006-01-02 15:04:05"),
		ChangeHistory:   changeHistoryDisplay,
		UpdateTime:      time.Now().Format("2006-01-02 15:04:05"),
	}

	// 更新 webData（线程安全）
	m.webDataMutex.Lock()
	m.webData = &data
	m.webDataMutex.Unlock()

	return nil
}

// StartWebServer 启动 HTTP 服务器在 localhost:3000
func (m *Monitor) StartWebServer() error {
	if !m.config.Web.Enabled {
		return nil
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m.webDataMutex.RLock()
		data := m.webData
		m.webDataMutex.RUnlock()

		if data == nil {
			http.Error(w, "Web data not available yet", http.StatusServiceUnavailable)
			return
		}

		htmlContent := generateHTML(*data)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// 防止浏览器缓存，确保每次访问都获取最新数据
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlContent))
	})

	// 添加手动刷新端点
	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Manual refresh requested from %s", r.RemoteAddr)
		if err := m.GenerateWebPageWithRetry(3, 2*time.Second); err != nil {
			http.Error(w, fmt.Sprintf("Failed to refresh: %v", err), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})

	addr := ":3000"
	log.Printf("Starting web server on http://localhost%s", addr)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	return nil
}

func formatVotes(votes *big.Int) string {
	if votes == nil {
		return "0.00000000"
	}

	// 转换为字符串,假设有8位小数
	votesStr := votes.String()
	if len(votesStr) <= 8 {
		// 补零
		for len(votesStr) < 8 {
			votesStr = "0" + votesStr
		}
		return "0." + votesStr
	}

	// 插入小数点
	integerPart := votesStr[:len(votesStr)-8]
	decimalPart := votesStr[len(votesStr)-8:]
	return integerPart + "." + decimalPart
}

func parseVotesFromString(votesStr string) (*big.Int, error) {
	return parseVotes(votesStr)
}

func generateHTML(data WebData) string {
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BPoS & CR Monitor</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        .header {
            background: white;
            border-radius: 10px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .header h1 {
            color: #333;
            margin-bottom: 15px;
        }
        .status {
            display: flex;
            gap: 20px;
            flex-wrap: wrap;
        }
        .status-item {
            background: #f8f9fa;
            padding: 10px 15px;
            border-radius: 5px;
            flex: 1;
            min-width: 200px;
        }
        .status-item strong {
            color: #667eea;
        }
        .status-badge {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: bold;
            margin-left: 10px;
        }
        .status-badge.yes {
            background: #28a745;
            color: white;
        }
        .status-badge.no {
            background: #dc3545;
            color: white;
        }
        .section {
            background: white;
            border-radius: 10px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .section h2 {
            color: #333;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #667eea;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #e9ecef;
        }
        th {
            background: #f8f9fa;
            font-weight: 600;
            color: #495057;
        }
        tr:hover {
            background: #f8f9fa;
        }
        .tier-badge {
            display: inline-block;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: bold;
        }
        .tier1 {
            background: #ffc107;
            color: #000;
        }
        .tier2 {
            background: #17a2b8;
            color: white;
        }
        .change-item {
            padding: 15px;
            margin-bottom: 10px;
            border-left: 4px solid #667eea;
            background: #f8f9fa;
            border-radius: 5px;
        }
        .change-item.cr {
            border-left-color: #28a745;
        }
        .change-item.bpos {
            border-left-color: #17a2b8;
        }
        .footer {
            text-align: center;
            color: white;
            margin-top: 20px;
            padding: 20px;
        }
        .public-key {
            font-family: monospace;
            font-size: 11px;
            color: #6c757d;
            word-break: break-all;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔍 BPoS & CR Monitor</h1>
            <div class="status">
                <div class="status-item">
                    <strong>最后检查时间:</strong> {{.LastCheckTime}}
                </div>
                <div class="status-item">
                    <strong>最后更新时间:</strong> {{.LastUpdateTime}}
                </div>
                <div class="status-item">
                    <strong>今日是否有变更:</strong>
                    {{if .HasChangesToday}}
                        <span class="status-badge yes">是</span>
                    {{else}}
                        <span class="status-badge no">否</span>
                    {{end}}
                </div>
                <div class="status-item">
                    <strong>页面更新时间:</strong> {{.UpdateTime}}
                </div>
            </div>
        </div>

        <div class="section">
            <h2>📋 CR 节点列表</h2>
            <table>
                <thead>
                    <tr>
                        <th>序号</th>
                        <th>昵称</th>
                        <th>Owner Public Key</th>
                        <th>DPoS Public Key</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $index, $node := .CRNodes}}
                    <tr>
                        <td>{{add $index 1}}</td>
                        <td>{{$node.NickName}}</td>
                        <td class="public-key">{{$node.OwnerPublicKey}}</td>
                        <td class="public-key">{{$node.DPoSPublicKey}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <div class="section">
            <h2>⚡ BPoS 节点列表 (按 Votes 排名)</h2>
            <table>
                <thead>
                    <tr>
                        <th>排名</th>
                        <th>昵称</th>
                        <th>Votes</th>
                        <th>层级</th>
                        <th>选中概率</th>
                        <th>Owner Public Key</th>
                        <th>DPoS Public Key</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $node := .BPoSNodes}}
                    <tr>
                        <td><strong>#{{$node.Rank}}</strong></td>
                        <td>{{$node.NickName}}</td>
                        <td><strong>{{$node.Votes}}</strong></td>
                        <td><span class="tier-badge {{lower $node.Tier}}">{{$node.Tier}}</span></td>
                        <td>{{$node.SelectionProb}}</td>
                        <td class="public-key">{{$node.OwnerPublicKey}}</td>
                        <td class="public-key">{{$node.DPoSPublicKey}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        {{if .ChangeHistory}}
        <div class="section">
            <h2>📝 变更历史</h2>
            {{range $record := .ChangeHistory}}
            <div class="change-item {{lower $record.Type}}">
                <strong>{{$record.Type}}</strong> - {{$record.Description}}<br>
                <small style="color: #6c757d;">时间: {{$record.Time}}</small>
            </div>
            {{end}}
        </div>
        {{end}}

        <div class="footer">
            <p>BPoS & CR Monitor - 自动更新系统</p>
        </div>
    </div>
</body>
</html>`

	// 创建模板函数
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
	}

	t, err := template.New("web").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return fmt.Sprintf("Error generating template: %v", err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Error executing template: %v", err)
	}

	return buf.String()
}
