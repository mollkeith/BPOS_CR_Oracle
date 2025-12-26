# BPoS & CR Monitor

一个用于监控和更新 Elastos 主链上 BPoS 和 CR 信息的 Go 应用程序。它会定期从主链 RPC 获取最新信息，并与智能合约中的数据进行对比，如果有变化则自动更新合约。

## 📋 目录

- [功能特性](#功能特性)
- [系统架构](#系统架构)
- [实现详解](#实现详解)
- [编译和运行](#编译和运行)
- [配置说明](#配置说明)
- [工作流程](#工作流程)
- [注意事项](#注意事项)

## ✨ 功能特性

- ✅ **定期监控**: 可配置的定期检查间隔，自动从主链 RPC 获取 CR 和 BPoS 信息
- ✅ **智能对比**: 自动对比 RPC 数据与合约数据，检测变更
- ✅ **自动更新**: 检测到变化时自动调用智能合约更新 (CRPool.setNodes 和 BPoSPool.updateNodes)
- ✅ **静态网页**: 生成美观的 HTML 网页展示当前状态、排名、层级等信息
- ✅ **邮件通知**: 支持 SMTP 邮件通知，更新成功/失败都会发送邮件
- ✅ **变更历史**: 记录所有变更历史，方便追踪

## 🏗️ 系统架构

```
┌─────────────────┐
│   Main Process  │
│   (main.go)     │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌──▼──────┐
│Monitor│ │ Email   │
│       │ │ Service │
└───┬───┘ └─────────┘
    │
    ├─── RPC Client (获取链上数据)
    ├─── Contract Client (合约交互)
    └─── Web Generator (生成网页)
```

## 🔧 实现详解

### 1. 配置管理 (`config.go`)

**功能**: 加载和验证 YAML 配置文件

**实现细节**:
- 使用 `gopkg.in/yaml.v3` 解析 YAML 配置
- 定义了结构化的配置结构:
  - `ContractConfig`: 合约地址配置
  - `RPCConfig`: RPC 端点配置
  - `UpdateConfig`: 更新间隔配置
  - `EmailConfig`: 邮件服务配置
  - `WebConfig`: 网页生成配置
  - `AccountConfig`: 账户私钥配置
- 配置验证: 检查必需字段是否存在
- 默认值设置: 为可选字段设置合理的默认值

**关键代码**:
```go
func LoadConfig(path string) (*Config, error) {
    // 读取文件
    data, err := os.ReadFile(path)
    // 解析 YAML
    var config Config
    yaml.Unmarshal(data, &config)
    // 验证必需字段
    // 设置默认值
}
```

### 2. RPC 客户端 (`rpc.go`)

**功能**: 与 Elastos 主链 RPC 通信，获取 CR 和 BPoS 数据

**实现细节**:
- 使用 `go-resty/resty` 库进行 HTTP 请求
- 实现 JSON-RPC 2.0 协议
- 支持的方法:
  - `listcurrentcrs`: 获取当前 CR 成员列表
  - `listproducers`: 获取 BPoS 生产者列表

**关键代码**:
```go
type RPCClient struct {
    client *resty.Client
    url    string
}

func (c *RPCClient) Call(method string, params interface{}) (json.RawMessage, error) {
    // 构建 JSON-RPC 请求
    req := RPCRequest{
        JSONRPC: "2.0",
        Method:  method,
        Params:  params,
        ID:      1,
    }
    // 发送 HTTP POST 请求
    // 解析响应
}
```

**数据模型**:
- `CRMember`: CR 成员信息 (code, cid, nickname, 等)
- `Producer`: BPoS 生产者信息 (ownerpublickey, nodepublickey, votes, 等)

### 3. 合约交互 (`contract.go`, `abi_helper.go`)

**功能**: 与智能合约交互，读取和更新节点信息

**实现细节**:
- 使用 `go-ethereum` 库与以太坊兼容链交互
- 从 JSON 文件加载 ABI (`abi_helper.go`)
- 支持两个合约:
  - `CRPoolContract`: CR 节点管理合约
  - `BPoSPoolContract`: BPoS 节点管理合约

**关键功能**:

1. **合约读取** (`GetAllNodes`):
   ```go
   func (c *CRPoolContract) GetAllNodes() ([]CRNode, error) {
       contract := bind.NewBoundContract(...)
       var result []struct { ... }
       opts := &bind.CallOpts{Context: context.Background()}
       contract.Call(opts, &result, "getAllNodes")
       // 转换并返回
   }
   ```

2. **合约更新** (`SetNodes` / `UpdateNodes`):
   ```go
   func (c *CRPoolContract) SetNodes(...) (*types.Transaction, error) {
       auth, _ := c.GetAuth() // 获取交易授权
       contract.Transact(auth, "setNodes", ...)
   }
   ```

**交易签名**:
- 使用私钥创建 `TransactOpts`
- 自动获取 nonce 和 gas price
- 设置合理的 gas limit

### 4. 监控逻辑 (`monitor.go`)

**功能**: 核心监控逻辑，对比数据并触发更新

**实现细节**:

1. **初始化** (`NewMonitor`):
   - 创建 RPC 客户端 (主链和 PG 链)
   - 创建合约客户端
   - 初始化 CR 和 BPoS 合约实例

2. **检查流程** (`CheckAndUpdate`):
   ```
   开始检查
   ├── 检查 CR
   │   ├── 从 RPC 获取 CR 列表
   │   ├── 从合约获取 CR 列表
   │   ├── 对比数据 (crHasChanges)
   │   └── 如有变化，调用 setNodes 更新
   │
   └── 检查 BPoS
       ├── 从 RPC 获取生产者列表
       ├── 从合约获取节点列表
       ├── 对比数据 (bposHasChanges)
       └── 如有变化，调用 updateNodes 更新
   ```

3. **数据对比逻辑**:

   **CR 对比** (`crHasChanges`):
   - 比较字段: `ownerPK`, `bposPK` (dposPublicKey), `NickName`
   - 使用 ownerPublicKey 作为唯一标识
   - 检查新增、删除、修改

   **BPoS 对比** (`bposHasChanges`):
   - 比较字段: `ownerPK`, `bposPK` (dposPublicKey), `NickName`, `votes`
   - 使用 ownerPublicKey 作为唯一标识
   - 检查新增、删除、修改

4. **变更记录**:
   - 每次更新都会记录到 `changeHistory`
   - 记录时间、类型、描述等信息

### 5. 网页生成 (`web.go`)

**功能**: 生成美观的静态 HTML 网页

**实现细节**:
- 使用 Go 的 `html/template` 生成 HTML
- 从合约读取最新数据
- 按 votes 对 BPoS 节点排序
- 标注层级 (Tier1: 前25名, Tier2: 25名之后)
- 显示变更历史

**网页内容**:
- 状态信息 (最后检查/更新时间, 今日是否有变更)
- CR 节点列表 (昵称, Public Keys)
- BPoS 节点列表 (排名, votes, 层级, 选中概率)
- 变更历史记录

**样式特点**:
- 现代化的渐变背景
- 响应式设计
- 清晰的表格展示
- 颜色编码的层级标识

### 6. 邮件服务 (`email.go`)

**功能**: 发送 HTML 格式的邮件通知

**实现细节**:
- 使用 `gomail` 库发送邮件
- 支持 SMTP 认证
- 支持 TLS/SSL
- HTML 格式邮件内容

**邮件内容**:
- 更新状态 (成功/失败)
- 错误信息 (如有)
- 状态统计
- 最近变更记录

**SMTP 配置**:
- 支持多种邮件服务商 (Gmail, QQ邮箱, 163邮箱等)
- 自动检测端口类型 (465 使用 SSL, 587 使用 StartTLS)

### 7. 主程序 (`main.go`)

**功能**: 程序入口，协调各个模块

**实现细节**:

1. **初始化**:
   ```go
   // 加载配置
   config := LoadConfig(configPath)
   // 创建监控器
   monitor := NewMonitor(config)
   // 创建邮件服务
   emailService := NewEmailService(config)
   ```

2. **定时任务**:
   ```go
   ticker := time.NewTicker(interval)
   for {
       select {
       case <-ticker.C:
           // 执行检查和更新
           performCheckAndUpdate(monitor, emailService)
           // 生成网页
           monitor.GenerateWebPage()
       }
   }
   ```

3. **信号处理**:
   - 监听 SIGINT 和 SIGTERM
   - 优雅关闭程序

## 🚀 编译和运行

### 前置要求

- Go 1.21 或更高版本
- 网络连接 (访问 RPC 和 SMTP 服务器)
- 有效的私钥 (用于签名交易)
- 足够的账户余额 (支付 Gas 费用)

### 步骤 1: 克隆或下载项目

```bash
cd /path/to/bpos_cr_monitor
```

### 步骤 2: 安装依赖

```bash
go mod download
```

或者使用 Makefile:
```bash
make deps
```

这会下载所有必需的 Go 依赖包:
- `github.com/ethereum/go-ethereum`: 以太坊客户端库
- `github.com/go-resty/resty/v2`: HTTP 客户端
- `gopkg.in/yaml.v3`: YAML 解析器
- `gopkg.in/gomail.v2`: 邮件发送库

### 步骤 3: 配置

复制配置文件模板:
```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，填入实际配置:

```yaml
contracts:
  cr_pool_address: "0x你的CR合约地址"
  bpos_pool_address: "0x你的BPoS合约地址"

rpc:
  main_chain: "https://api.elastos.io/ela"
  pg_chain: "https://api.elastos.io/pg"

update:
  interval: "24h"  # 更新间隔

email:
  enabled: true
  to:
    - "your_email@example.com"
  smtp:
    host: "smtp.gmail.com"
    port: 587
    username: "your_email@gmail.com"
    password: "your_app_password"
    from: "your_email@gmail.com"
    tls: true

account:
  private_key: "你的私钥hex字符串"
```

**重要配置说明**:

1. **合约地址**: 从合约部署者获取
2. **RPC 地址**: 使用 Elastos 官方 RPC 或自建节点
3. **私钥**: 
   - 格式: hex 字符串，不带 `0x` 前缀
   - 安全: 不要提交到版本控制系统
   - 权限: 确保账户有合约的 ORACLE_ROLE 权限
4. **邮件配置**:
   - Gmail: 需要使用[应用专用密码](https://support.google.com/accounts/answer/185833)
   - QQ邮箱: 端口 587, 使用授权码
   - 163邮箱: 端口 25 或 465

### 步骤 4: 编译

**方式 1: 使用 Go 命令**
```bash
go build -o bpos_cr_monitor
```

**方式 2: 使用 Makefile**
```bash
make build
```

编译成功后会在当前目录生成 `bpos_cr_monitor` 可执行文件。

### 步骤 5: 运行

**方式 1: 直接运行可执行文件**
```bash
./bpos_cr_monitor -config config.yaml
```

**方式 2: 使用 go run (开发模式)**
```bash
go run . -config config.yaml
```

**方式 3: 使用 Makefile**
```bash
make run
```

### 步骤 6: 验证运行

程序启动后会:
1. 加载配置并验证
2. 连接 RPC 和合约
3. 立即执行一次检查
4. 生成初始网页
5. 开始定时任务

查看日志输出:
```
Monitor started with update interval: 24h0m0s
CR Pool Address: 0x...
BPoS Pool Address: 0x...
Email notification: true
Web page generation: true
Performing initial check...
Starting check at 2024-01-01T12:00:00Z
...
```

### 步骤 7: 查看结果

1. **网页**: 打开 `web/index.html` 查看生成的网页
2. **邮件**: 检查配置的邮箱，应该收到更新通知
3. **日志**: 查看控制台输出的日志信息

## ⚙️ 配置说明

### 完整配置示例

```yaml
# ============================================
# 合约配置
# ============================================
contracts:
  cr_pool_address: "0x1234567890123456789012345678901234567890"
  bpos_pool_address: "0x0987654321098765432109876543210987654321"

# ============================================
# RPC配置
# ============================================
rpc:
  main_chain: "https://api.elastos.io/ela"
  pg_chain: "https://api.elastos.io/pg"

# ============================================
# 更新配置
# ============================================
update:
  # 支持格式: "1h", "24h", "1d", "30m", "2h30m" 等
  interval: "24h"

# ============================================
# 邮件配置
# ============================================
email:
  enabled: true  # 是否启用邮件通知
  to:
    - "admin@example.com"
    - "monitor@example.com"
  subject: "BPoS & CR Monitor"  # 邮件主题前缀
  
  smtp:
    host: "smtp.gmail.com"      # SMTP 服务器
    port: 587                   # 端口 (Gmail: 587, QQ: 587, 163: 25)
    username: "your_email@gmail.com"
    password: "your_app_password"  # Gmail 需要使用应用专用密码
    from: "your_email@gmail.com"
    from_name: "BPoS CR Monitor"
    tls: true                   # 是否使用 TLS

# ============================================
# 网页配置
# ============================================
web:
  enabled: true                 # 是否启用网页生成
  output_path: "./web"          # 输出路径

# ============================================
# 账户配置
# ============================================
account:
  # 私钥 (hex 格式, 不带 0x 前缀)
  # 警告: 请妥善保管, 不要泄露!
  private_key: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
```

### 配置项说明

| 配置项 | 类型 | 必需 | 说明 |
|--------|------|------|------|
| `contracts.cr_pool_address` | string | ✅ | CR 合约地址 |
| `contracts.bpos_pool_address` | string | ✅ | BPoS 合约地址 |
| `rpc.main_chain` | string | ✅ | 主链 RPC URL |
| `rpc.pg_chain` | string | ✅ | PG 链 RPC URL |
| `update.interval` | string | ❌ | 更新间隔 (默认: "24h") |
| `email.enabled` | bool | ❌ | 是否启用邮件 (默认: true) |
| `email.to` | []string | ❌ | 收件人列表 |
| `email.smtp.*` | - | ❌ | SMTP 服务器配置 |
| `web.enabled` | bool | ❌ | 是否启用网页 (默认: true) |
| `web.output_path` | string | ❌ | 网页输出路径 (默认: "./web") |
| `account.private_key` | string | ✅ | 私钥 (hex, 无 0x 前缀) |

## 🔄 工作流程

### 完整工作流程

```
启动程序
    ↓
加载配置
    ↓
初始化监控器
    ├── 创建 RPC 客户端
    ├── 创建合约客户端
    └── 初始化合约实例
    ↓
立即执行首次检查
    ├── 获取 RPC 数据
    ├── 获取合约数据
    ├── 对比数据
    ├── 如有变化 → 更新合约
    └── 发送邮件通知
    ↓
生成初始网页
    ↓
进入定时循环
    ├── 等待定时器触发
    ├── 执行检查更新
    ├── 生成网页
    └── 发送邮件
    ↓
监听系统信号
    └── 优雅关闭
```

### 数据更新流程

```
定时触发 / 手动触发
    ↓
检查 CR
    ├── RPC: listcurrentcrs
    ├── 合约: getAllNodes
    ├── 对比: ownerPK, bposPK, NickName
    └── 如有变化 → setNodes
    ↓
检查 BPoS
    ├── RPC: listproducers
    ├── 合约: getAllNodes
    ├── 对比: ownerPK, bposPK, NickName, votes
    └── 如有变化 → updateNodes
    ↓
记录变更历史
    ↓
生成网页
    ↓
发送邮件
```

## ⚠️ 注意事项

### 安全注意事项

1. **私钥安全**:
   - ⚠️ **永远不要**将私钥提交到版本控制系统
   - ⚠️ 使用 `.gitignore` 排除 `config.yaml`
   - ⚠️ 定期轮换私钥
   - ⚠️ 使用最小权限原则

2. **配置文件**:
   - 使用 `config.yaml.example` 作为模板
   - 不要将实际配置提交到仓库
   - 在生产环境使用环境变量或密钥管理服务

3. **网络安全**:
   - 使用 HTTPS RPC 端点
   - 验证 RPC 服务器的证书
   - 考虑使用 VPN 或私有网络

### 运行注意事项

1. **Gas 费用**:
   - 确保账户有足够的余额支付 Gas
   - 监控 Gas 价格，避免在高价时更新
   - 考虑设置合理的 gas limit

2. **合约权限**:
   - 确保用于签名的账户有 `ORACLE_ROLE` 权限
   - 联系合约管理员授予权限

3. **RPC 限制**:
   - 注意 RPC 服务器的速率限制
   - 如果频繁调用，考虑使用自建节点
   - 实现重试和错误处理

4. **数据一致性**:
   - CR 的 `dposPublicKey`: 当前使用 `code` 字段，如实际场景不同需调整
   - 确保 RPC 数据和合约数据格式匹配
   - 定期验证数据准确性

5. **邮件配置**:
   - Gmail 需要使用应用专用密码，不是普通密码
   - 某些邮件服务商可能阻止自动发送
   - 测试邮件配置是否正常工作

### 故障排查

1. **连接失败**:
   - 检查 RPC URL 是否正确
   - 检查网络连接
   - 检查防火墙设置

2. **合约调用失败**:
   - 检查合约地址是否正确
   - 检查账户权限
   - 检查账户余额
   - 查看交易回执中的错误信息

3. **邮件发送失败**:
   - 检查 SMTP 配置
   - 检查邮件服务商的限制
   - 查看日志中的错误信息

4. **数据不匹配**:
   - 检查 RPC 返回的数据格式
   - 检查合约 ABI 是否正确
   - 验证数据转换逻辑

## 📝 许可证

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📧 联系方式

如有问题，请通过 Issue 或邮件联系。
