# BPoS & CR Monitor

一个用于监控和更新 Elastos 主链上 BPoS 和 CR 信息的 Go 应用程序。它会定期从主链 RPC 获取最新信息，并与智能合约中的数据进行对比，如果有变化则自动更新合约。

## 📋 目录

- [快速开始](#快速开始)
- [功能特性](#功能特性)
- [系统架构](#系统架构)
- [实现详解](#实现详解)
- [编译和运行](#编译和运行)
- [配置说明](#配置说明)
- [工作流程](#工作流程)
- [注意事项](#注意事项)

## 🚀 快速开始

### 最简单的启动方式

1. **准备配置文件**
```bash
cp config.json.example config.json
# 编辑 config.json，填入合约地址、RPC 地址等
```

2. **准备 keystore 文件**
```bash
mkdir -p keystore
# 将你的 keystore 文件放到 keystore/ 目录下
```

3. **编译程序**
```bash
go build -o bpos_cr_monitor
```

4. **启动程序**
```bash
./bpos_cr_monitor -config config.json -p "your_keystore_password"
```

### 启动命令详解

**基本格式**:
```bash
./bpos_cr_monitor -config <配置文件路径> -p <keystore密码>
```

**参数说明**:
- `-config`: 配置文件路径 (默认: `config.json`)
- `-p`: **必需** keystore 密码

**示例**:
```bash
# 使用默认配置文件
./bpos_cr_monitor -p "mypassword123"

# 指定配置文件
./bpos_cr_monitor -config /path/to/config.json -p "mypassword123"

# 使用环境变量 (更安全)
export KEYSTORE_PASSWORD="mypassword123"
./bpos_cr_monitor -config config.json -p "$KEYSTORE_PASSWORD"

# 使用密码文件 (生产环境推荐)
echo "mypassword123" > .password
chmod 600 .password
./bpos_cr_monitor -config config.json -p "$(cat .password)"
```

**查看帮助**:
```bash
./bpos_cr_monitor -h
```

**后台运行** (Linux/macOS):
```bash
# 使用 nohup
nohup ./bpos_cr_monitor -config config.json -p "mypassword123" > monitor.log 2>&1 &

# 或使用 screen
screen -S monitor
./bpos_cr_monitor -config config.json -p "mypassword123"
# 按 Ctrl+A 然后 D 来分离会话

# 或使用 tmux
tmux new -s monitor
./bpos_cr_monitor -config config.json -p "mypassword123"
# 按 Ctrl+B 然后 D 来分离会话
```

**停止程序**:
```bash
# 如果在前台运行，按 Ctrl+C

# 如果在后台运行，找到进程并停止
ps aux | grep bpos_cr_monitor
kill <PID>

# 或使用 pkill
pkill -f bpos_cr_monitor
```

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

**功能**: 加载和验证 JSON 配置文件

**实现细节**:
- 使用 Go 标准库 `encoding/json` 解析 JSON 配置
- 定义了结构化的配置结构:
  - `ContractConfig`: 合约地址配置
  - `RPCConfig`: RPC 端点配置
  - `UpdateConfig`: 更新间隔配置
  - `EmailConfig`: 邮件服务配置
  - `WebConfig`: 网页生成配置
  - `AccountConfig`: 账户 keystore 配置
- 配置验证: 检查必需字段是否存在
- 默认值设置: 为可选字段设置合理的默认值

**关键代码**:
```go
func LoadConfig(path string) (*Config, error) {
    // 读取文件
    data, err := os.ReadFile(path)
    // 解析 JSON
    var config Config
    json.Unmarshal(data, &config)
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
- 从 keystore 解锁私钥并创建 `TransactOpts`
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
- 有效的 keystore 文件和密码 (用于签名交易)
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
- `encoding/json`: JSON 解析器 (Go 标准库)
- `gopkg.in/gomail.v2`: 邮件发送库

### 步骤 3: 准备 Keystore

如果你还没有 keystore 文件，可以通过以下方式生成:

**方式 1: 使用项目自带的工具 (推荐) ⭐**

这是最简单的方式，使用项目内置的 keystore 生成工具:

```bash
# 方式 1a: 生成新的 keystore (会生成新私钥)
go run ./cmd/create_keystore

# 方式 1b: 从现有私钥创建 keystore
go run ./cmd/create_keystore -key "your_private_key_hex_without_0x"

# 方式 1c: 指定 keystore 目录
go run ./cmd/create_keystore -dir ./keystore

# 方式 1d: 使用 Makefile
make create-keystore
```

**工具参数说明**:
- `-dir`: keystore 文件保存目录 (默认: `./keystore`)
- `-key`: 可选的私钥 (hex 格式，不带 0x 前缀)。如果不提供，将生成新私钥
- `-p`: 可选的密码。如果不提供，会提示输入

**使用示例**:
```bash
# 生成新 keystore (交互式)
$ go run ./cmd/create_keystore
Generated new private key
Enter password for keystore: 
Confirm password: 

✅ Keystore created successfully!
Address: 0x1234567890123456789012345678901234567890
Keystore file: /path/to/keystore/UTC--2024-01-01T00-00-00.000000000Z--1234567890123456789012345678901234567890
Keystore directory: ./keystore

# 从私钥创建 keystore
$ go run ./cmd/create_keystore -key "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
Using provided private key
Enter password for keystore: 
Confirm password: 

✅ Keystore created successfully!
Address: 0x...
```

**方式 2: 使用 geth**
```bash
# 安装 geth (如果还没有)
# macOS: brew install ethereum
# Linux: apt-get install ethereum

# 创建 keystore 目录
mkdir -p keystore

# 创建新账户 (会提示输入密码)
geth account new --keystore ./keystore
```

**方式 3: 使用 MetaMask**
1. 在 MetaMask 中导出账户
2. 选择 "Export Private Key"
3. 然后可以使用工具将私钥转换为 keystore 格式

**方式 4: 使用在线工具**
- 使用 [MyEtherWallet](https://www.myetherwallet.com/) 或类似工具
- 创建新钱包并导出 keystore 文件

生成的 keystore 文件通常位于 `keystore/` 目录下，文件名格式为:
```
UTC--2024-01-01T00-00-00.000000000Z--your_address
```

### 步骤 4: 配置

复制配置文件模板:
```bash
cp config.json.example config.json
```

编辑 `config.json`，填入实际配置:

```json
{
  "contracts": {
    "cr_pool_address": "0x你的CR合约地址",
    "bpos_pool_address": "0x你的BPoS合约地址"
  },
  "rpc": {
    "main_chain": "https://api.elastos.io/ela",
    "pg_chain": "https://api.elastos.io/pg"
  },
  "update": {
    "interval": "24h"
  },
  ...
  "account": {
    "keystore_path": "./keystore/UTC--2024-01-01T00-00-00.000000000Z--your_address"
  }
}
```

**重要配置说明**:

1. **合约地址**: 从合约部署者获取
2. **RPC 地址**: 使用 Elastos 官方 RPC 或自建节点
3. **Keystore**: 
   - 路径: keystore 文件的完整路径 (在配置文件中)
   - 密码: keystore 文件的解锁密码 (**通过命令行参数 `-p` 提供，不在配置文件中**)
   - 格式: 标准的以太坊 keystore 格式 (UTC--开头)
   - 安全: 不要将 keystore 文件和密码提交到版本控制系统
   - 权限: 确保账户有合约的 ORACLE_ROLE 权限
   - 生成: 可以使用 `geth account new` 或 MetaMask 导出
4. **邮件配置**:
   - Gmail: 需要使用[应用专用密码](https://support.google.com/accounts/answer/185833)
   - QQ邮箱: 端口 587, 使用授权码
   - 163邮箱: 端口 25 或 465

### 步骤 5: 编译

**方式 1: 使用 Go 命令**
```bash
go build -o bpos_cr_monitor
```

**方式 2: 使用 Makefile**
```bash
make build
```

编译成功后会在当前目录生成 `bpos_cr_monitor` 可执行文件。

### 步骤 6: 运行

**⚠️ 重要**: 启动程序时必须使用 `-p` 参数提供 keystore 密码！

**方式 1: 直接运行可执行文件**
```bash
./bpos_cr_monitor -config config.json -p "your_keystore_password"
```

**方式 2: 使用 go run (开发模式)**
```bash
go run . -config config.json -p "your_keystore_password"
```

**方式 3: 使用 Makefile**
```bash
# 使用 Makefile (需要提供 PASSWORD 参数)
make run PASSWORD="your_keystore_password"
```

**方式 4: 从环境变量读取密码 (更安全)**
```bash
# 设置环境变量
export KEYSTORE_PASSWORD="your_keystore_password"

# 运行程序
./bpos_cr_monitor -config config.json -p "$KEYSTORE_PASSWORD"
```

**方式 5: 使用密码文件 (生产环境推荐)**
```bash
# 创建密码文件 (设置适当权限)
echo "your_keystore_password" > .password
chmod 600 .password

# 运行程序
./bpos_cr_monitor -config config.json -p "$(cat .password)"
```

**命令行参数说明**:
- `-config`: 配置文件路径 (默认: `config.json`)
- `-p`: **必需** keystore 密码

**查看帮助**:
```bash
./bpos_cr_monitor -h
```

**使用启动脚本** (可选，更方便):
```bash
# 复制启动脚本模板
cp start.sh.example start.sh

# 编辑 start.sh，填入你的 keystore 密码
# 然后运行
chmod +x start.sh
./start.sh
```

### 步骤 7: 验证运行

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

### 步骤 8: 查看结果

1. **网页**: 打开 `web/index.html` 查看生成的网页
2. **邮件**: 检查配置的邮箱，应该收到更新通知
3. **日志**: 查看控制台输出的日志信息

## ⚙️ 配置说明

### 完整配置示例

```json
{
  "contracts": {
    "cr_pool_address": "0x1234567890123456789012345678901234567890",
    "bpos_pool_address": "0x0987654321098765432109876543210987654321"
  },
  "rpc": {
    "main_chain": "https://api.elastos.io/ela",
    "pg_chain": "https://api.elastos.io/pg"
  },
  "update": {
    "interval": "24h"
  },
  "email": {
    "enabled": true,
    "to": [
      "admin@example.com",
      "monitor@example.com"
    ],
    "subject": "BPoS & CR Monitor",
    "smtp": {
      "host": "smtp.gmail.com",
      "port": 587,
      "username": "your_email@gmail.com",
      "password": "your_app_password",
      "from": "your_email@gmail.com",
      "from_name": "BPoS CR Monitor",
      "tls": true
    }
  },
  "web": {
    "enabled": true,
    "output_path": "./web"
  },
  "account": {
    "keystore_path": "./keystore/UTC--2024-01-01T00-00-00.000000000Z--your_address"
  }
}
```

**注意**: 密码不在配置文件中，需要通过命令行参数 `-p` 提供。

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
| `account.keystore_path` | string | ✅ | Keystore 文件路径 |
| `-p` (命令行参数) | string | ✅ | Keystore 密码 (通过 `-p` 参数提供) |

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

1. **Keystore 安全**:
   - ⚠️ **永远不要**将 keystore 文件和密码提交到版本控制系统
   - ⚠️ 使用 `.gitignore` 排除 `config.json` 和 `keystore/` 目录
   - ⚠️ 密码通过命令行参数 `-p` 提供，不在配置文件中
   - ⚠️ 定期轮换 keystore 和密码
   - ⚠️ 使用最小权限原则
   - ⚠️ 在生产环境考虑使用环境变量或密码文件存储密码
   - ⚠️ 避免在命令行历史中留下密码 (使用环境变量或密码文件)

2. **配置文件**:
   - 使用 `config.json.example` 作为模板
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
   - 检查 keystore 文件路径是否正确
   - 检查是否提供了 `-p` 参数
   - 检查 keystore 密码是否正确
   - 检查账户权限 (ORACLE_ROLE)
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
