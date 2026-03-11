package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"
)

type Monitor struct {
	config         *Config
	mainRPC        *RPCClient
	pgRPC          *RPCClient
	crContract     *CRPoolContract
	bposContract   *BPoSPoolContract
	contractClient *ContractClient
	lastCheckTime  time.Time
	lastUpdateTime time.Time
	changeHistory  []ChangeRecord
	webData        *WebData
	webDataMutex   sync.RWMutex
}

type ChangeRecord struct {
	Time        time.Time
	Type        string // "CR" or "BPoS"
	Description string
	Details     interface{}
}

func NewMonitor(config *Config, password string) (*Monitor, error) {
	if password == "" {
		return nil, fmt.Errorf("keystore password is required (use -p flag)")
	}

	mainRPC := NewRPCClient(config.RPC.MainChain)
	pgRPC := NewRPCClient(config.RPC.PGChain)

	contractClient, err := NewContractClientFromKeystore(config.RPC.PGChain, config.Account.KeystorePath, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract client: %w", err)
	}

	crContract, err := NewCRPoolContract(contractClient, config.Contracts.CRPoolAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create CR contract: %w", err)
	}

	bposContract, err := NewBPoSPoolContract(contractClient, config.Contracts.BPoSPoolAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create BPoS contract: %w", err)
	}

	return &Monitor{
		config:         config,
		mainRPC:        mainRPC,
		pgRPC:          pgRPC,
		crContract:     crContract,
		bposContract:   bposContract,
		contractClient: contractClient,
		changeHistory:  make([]ChangeRecord, 0),
		webData:        nil,
	}, nil
}

// GetWalletInfo 获取钱包地址和余额
func (m *Monitor) GetWalletInfo() (string, *big.Int, error) {
	address := m.contractClient.GetAddress()
	balance, err := m.contractClient.GetBalance()
	if err != nil {
		return "", nil, err
	}
	return address.Hex(), balance, nil
}

// CheckAndUpdate 检查并更新 CR 和 BPoS 信息
func (m *Monitor) CheckAndUpdate() error {
	m.lastCheckTime = time.Now()
	log.Printf("Starting check at %s", m.lastCheckTime.Format(time.RFC3339))

	// 检查 CR
	crChanged, err := m.checkAndUpdateCR()
	if err != nil {
		log.Printf("Error checking/updating CR: %v", err)
		return fmt.Errorf("CR update failed: %w", err)
	}

	// 检查 BPoS
	bposChanged, err := m.checkAndUpdateBPoS()
	if err != nil {
		log.Printf("Error checking/updating BPoS: %v", err)
		return fmt.Errorf("BPoS update failed: %w", err)
	}

	if crChanged || bposChanged {
		// lastUpdateTime 在交易确认时已经更新（在 checkAndUpdateCR 或 checkAndUpdateBPoS 中）
		// 这里只需要记录日志
		log.Printf("Update completed at %s", m.lastUpdateTime.Format(time.RFC3339))
	} else {
		log.Printf("No changes detected")
	}

	return nil
}

// checkAndUpdateCR 检查并更新 CR
func (m *Monitor) checkAndUpdateCR() (bool, error) {
	// 从 RPC 获取当前 CR 列表
	rpcCRs, err := m.mainRPC.ListCurrentCRs()
	if err != nil {
		return false, fmt.Errorf("failed to get CRs from RPC: %w", err)
	}

	// 打印 RPC 获取到的 CR 结果
	log.Printf("RPC CR Results: Total %d CRs", len(rpcCRs))
	for i, cr := range rpcCRs {
		log.Printf("  CR[%d]: Nickname=%s, CID=%s, DPoSPublicKey=%s, ImpeachmentVotes=%s, Penalty=%s, State=%s",
			i, cr.Nickname, cr.CID, cr.DPoSPublicKey, cr.ImpeachmentVotes, cr.Penalty, cr.State)
	}

	// 从合约获取当前 CR 列表
	contractCRs, err := m.crContract.GetAllNodes()
	if err != nil {
		log.Printf("Warning: failed to get CRs from contract (contract may be empty or not initialized): %v", err)
		return false, fmt.Errorf("failed to get CRs from contract: %w", err)
	}

	// 打印合约获取到的 CR 结果
	log.Printf("Contract CR Results: Total %d CRs", len(contractCRs))
	for i, cr := range contractCRs {
		log.Printf("  Contract CR[%d]: Nickname=%s, OwnerPK=%s, DPoSPK=%s, Exists=%v",
			i, cr.NickName, hex.EncodeToString(cr.OwnerPublicKey), hex.EncodeToString(cr.DPoSPublicKey), cr.Exists)
	}

	// 比较是否有变化
	if !m.crHasChanges(rpcCRs, contractCRs) {
		return false, nil
	}

	// 准备更新数据
	nickNames := make([]string, 0, len(rpcCRs))
	ownerPublicKeys := make([][]byte, 0, len(rpcCRs))
	dposPublicKeys := make([][]byte, 0, len(rpcCRs))

	for _, cr := range rpcCRs {
		nickNames = append(nickNames, cr.Nickname)

		// CR 的 code 需要去掉头一个 byte 和尾一个 byte 才能是 ownerPublicKey
		ownerPK, err := codeToOwnerPublicKey(cr.Code)
		if err != nil {
			log.Printf("Warning: failed to parse owner public key for CR %s: %v", cr.Nickname, err)
			continue
		}
		ownerPublicKeys = append(ownerPublicKeys, ownerPK)

		// 使用 RPC 返回的 dpospublickey 作为 dposPublicKey
		dposPK, err := hexToBytes(cr.DPoSPublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse dpos public key for CR %s: %v", cr.Nickname, err)
			continue
		}
		dposPublicKeys = append(dposPublicKeys, dposPK)
	}

	// 调用合约更新
	txHash, err := m.crContract.SetNodes(nickNames, ownerPublicKeys, dposPublicKeys)
	if err != nil {
		return false, fmt.Errorf("failed to call setNodes: %w", err)
	}

	log.Printf("CR update transaction sent: %s, waiting for confirmation...", txHash.Hex())

	// 等待交易确认（最多等待 5 分钟）
	ctx := context.Background()
	receipt, err := m.contractClient.WaitMined(ctx, txHash, 5*time.Minute)
	if err != nil {
		// 检查是否是超时错误（交易可能还在 pending）
		// 使用 errors.Is 检查错误链中是否包含 ErrTransactionTimeout
		if errors.Is(err, ErrTransactionTimeout) {
			log.Printf("Warning: CR transaction %s timeout, but it may still be pending. Will check status on next update cycle.", txHash.Hex())
			// 不返回错误，允许继续执行，下次检查时会发现数据已更新（如果交易确认了）
			// 如果交易未确认，下次检查时会发现数据不一致，会再次尝试更新
			return false, nil // 返回 false 表示本次未完成更新，但不阻止后续检查
		}
		return false, fmt.Errorf("failed to wait for transaction confirmation: %w", err)
	}

	log.Printf("CR update transaction confirmed: %s (block: %d, status: %d)", txHash.Hex(), receipt.BlockNumber.Uint64(), receipt.Status)

	// 记录变更（使用交易确认时间）
	confirmTime := time.Now()
	m.lastUpdateTime = confirmTime
	m.changeHistory = append(m.changeHistory, ChangeRecord{
		Time:        confirmTime,
		Type:        "CR",
		Description: fmt.Sprintf("Updated %d CR nodes (tx: %s)", len(rpcCRs), txHash.Hex()),
		Details:     rpcCRs,
	})

	return true, nil
}

// crHasChanges 检查 CR 是否有变化
func (m *Monitor) crHasChanges(rpcCRs []CRMember, contractCRs []CRNode) bool {
	// 创建合约节点的映射 (使用 ownerPublicKey 作为 key)
	contractMap := make(map[string]CRNode)
	for _, node := range contractCRs {
		key := hex.EncodeToString(node.OwnerPublicKey)
		contractMap[key] = node
	}

	// 创建 RPC CR 的有效映射（成功解析 ownerPK 的）
	rpcMap := make(map[string]CRMember)
	validRPCCount := 0
	for _, cr := range rpcCRs {
		// CR 的 code 需要去掉头一个 byte 和尾一个 byte 才能是 ownerPublicKey
		ownerPK, err := codeToOwnerPublicKey(cr.Code)
		if err != nil {
			log.Printf("Warning: failed to parse ownerPK from code for CR %s: %v", cr.Nickname, err)
			continue
		}
		key := hex.EncodeToString(ownerPK)
		rpcMap[key] = cr
		validRPCCount++
	}

	log.Printf("CR comparison: RPC valid CRs=%d, Contract CRs=%d", validRPCCount, len(contractCRs))

	// 检查数量是否一致
	if validRPCCount != len(contractCRs) {
		log.Printf("CR count mismatch: RPC=%d, Contract=%d", validRPCCount, len(contractCRs))
		return true
	}

	// 检查 RPC 中的每个 CR 是否在合约中存在且信息一致
	for key, cr := range rpcMap {
		contractNode, exists := contractMap[key]
		if !exists {
			log.Printf("CR not found in contract: Nickname=%s, OwnerPK=%s", cr.Nickname, key)
			return true // 有新的 CR
		}

		// 比较 NickName
		if contractNode.NickName != cr.Nickname {
			log.Printf("CR NickName mismatch: RPC=%s, Contract=%s, OwnerPK=%s", cr.Nickname, contractNode.NickName, key)
			return true
		}

		// 比较 dposPublicKey (使用 RPC 返回的 dpospublickey)
		rpcDPoSPK, err := hexToBytes(cr.DPoSPublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse dposPublicKey for CR %s: %v", cr.Nickname, err)
			continue
		}
		contractDPoSPKStr := hex.EncodeToString(contractNode.DPoSPublicKey)
		rpcDPoSPKStr := hex.EncodeToString(rpcDPoSPK)
		if contractDPoSPKStr != rpcDPoSPKStr {
			log.Printf("CR DPoSPublicKey mismatch: RPC=%s, Contract=%s, OwnerPK=%s", rpcDPoSPKStr, contractDPoSPKStr, key)
			return true
		}
	}

	// 检查合约中是否有 RPC 中没有的 CR（多余的 CR）
	for key, contractNode := range contractMap {
		if _, exists := rpcMap[key]; !exists {
			log.Printf("CR exists in contract but not in RPC: Nickname=%s, OwnerPK=%s", contractNode.NickName, key)
			return true
		}
	}

	log.Printf("CR comparison: No changes detected, all CRs match")
	return false
}

// checkAndUpdateBPoS 检查并更新 BPoS
func (m *Monitor) checkAndUpdateBPoS() (bool, error) {
	// 从 RPC 获取当前生产者列表
	rpcProducers, err := m.mainRPC.ListProducers()
	if err != nil {
		return false, fmt.Errorf("failed to get producers from RPC: %w", err)
	}

	// 打印 RPC 获取到的 BPoS 结果
	log.Printf("RPC BPoS Results: Total %d Producers", len(rpcProducers))
	for i, producer := range rpcProducers {
		log.Printf("  BPoS[%d]: Nickname=%s, OwnerPK=%s, NodePK=%s, Votes=%s, DPoSV2Votes=%s, State=%s, Active=%v",
			i, producer.Nickname, producer.OwnerPublicKey, producer.NodePublicKey, producer.Votes, producer.DPoSV2Votes, producer.State, producer.Active)
	}

	// 从合约获取当前 BPoS 节点列表
	contractNodes, err := m.bposContract.GetAllNodes()
	if err != nil {
		return false, fmt.Errorf("failed to get BPoS nodes from contract: %w", err)
	}

	// 打印合约获取到的 BPoS 结果
	log.Printf("Contract BPoS Results: Total %d Nodes", len(contractNodes))
	for i, node := range contractNodes {
		log.Printf("  Contract BPoS[%d]: Nickname=%s, OwnerPK=%s, DPoSPK=%s, Votes=%s, Exists=%v",
			i, node.NickName, hex.EncodeToString(node.OwnerPublicKey), hex.EncodeToString(node.DPoSPublicKey), node.Votes.String(), node.Exists)
	}

	// 过滤满足条件的生产者
	filteredProducers := make([]Producer, 0, len(rpcProducers))
	for _, producer := range rpcProducers {
		// 过滤条件：State == Active or Inactive or Illegal and dposv2votes > min
		// 使用大小写不敏感比较，兼容 RPC 返回 "inactive" 等小写值
		stateNorm := strings.ToLower(strings.TrimSpace(producer.State))
		if stateNorm != "active" && stateNorm != "inactive" && stateNorm != "illegal" {
			log.Printf("Warning: Node %s is not active, inactive, or illegal, skipping, state: %s", producer.Nickname, producer.State)
			continue
		}

		// 解析 dposv2votes 并检查是否 > 配置的最小值
		dposv2Votes, err := parseVotes(producer.DPoSV2Votes)
		if err != nil {
			log.Printf("Warning: failed to parse dposv2votes for producer %s: %v, skipping", producer.Nickname, err)
			continue
		}

		// 从配置获取最小投票数
		minVotes, err := m.config.BPoS.GetMinDPoSV2Votes()
		if err != nil {
			log.Printf("Warning: failed to parse min_dposv2_votes from config: %v, using default 80000", err)
			minVotes = big.NewInt(8000000000000) // 默认 80000 * 10^8
		}
		if dposv2Votes.Cmp(minVotes) <= 0 {
			log.Printf("Warning: Node %s has less than %d votes, skipping, votes: %s", producer.Nickname, minVotes, dposv2Votes.String())
			continue
		}

		filteredProducers = append(filteredProducers, producer)
	}

	// print filtered producers
	log.Printf("Filtered BPoS producers: %d out of %d meet criteria (State=Active/Inactive/Illegal, dposv2votes>%s)", len(filteredProducers), len(rpcProducers), m.config.BPoS.MinDPoSV2Votes)
	for i, producer := range filteredProducers {
		log.Printf("  Filtered BPoS[%d]: Nickname=%s, OwnerPK=%s, NodePK=%s, Votes=%s, DPoSV2Votes=%s, State=%s, Active=%v",
			i, producer.Nickname, producer.OwnerPublicKey, producer.NodePublicKey, producer.Votes, producer.DPoSV2Votes, producer.State, producer.Active)
	}

	minVotesStr := m.config.BPoS.MinDPoSV2Votes
	if minVotesStr == "" {
		minVotesStr = "80000"
	}
	log.Printf("Filtered BPoS producers: %d out of %d meet criteria (State=Active/Inactive/Illegal, dposv2votes>%s)", len(filteredProducers), len(rpcProducers), minVotesStr)

	// 构建操作列表 - 比较 RPC 和合约数据，确定 OperationType
	operations := m.buildNodeOperations(filteredProducers, contractNodes)

	if len(operations) == 0 {
		log.Printf("No operations needed for BPoS nodes")
		return false, nil
	}

	// 调用合约同步
	txHash, err := m.bposContract.SyncNodes(operations)
	if err != nil {
		return false, fmt.Errorf("failed to call syncNodes: %w", err)
	}

	log.Printf("BPoS sync transaction sent: %s, waiting for confirmation...", txHash.Hex())

	// 等待交易确认（最多等待 5 分钟）
	ctx := context.Background()
	receipt, err := m.contractClient.WaitMined(ctx, txHash, 5*time.Minute)
	if err != nil {
		// 检查是否是超时错误（交易可能还在 pending）
		// 使用 errors.Is 检查错误链中是否包含 ErrTransactionTimeout
		if errors.Is(err, ErrTransactionTimeout) {
			log.Printf("Warning: BPoS transaction %s timeout, but it may still be pending. Will check status on next update cycle.", txHash.Hex())
			// 不返回错误，允许继续执行，下次检查时会发现数据已更新（如果交易确认了）
			// 如果交易未确认，下次检查时会发现数据不一致，会再次尝试更新
			return false, nil // 返回 false 表示本次未完成更新，但不阻止后续检查
		}
		return false, fmt.Errorf("failed to wait for transaction confirmation: %w", err)
	}

	log.Printf("BPoS sync transaction confirmed: %s (block: %d, status: %d)", txHash.Hex(), receipt.BlockNumber.Uint64(), receipt.Status)

	// 统计操作类型
	addCount := 0
	updateCount := 0
	removeCount := 0
	for _, op := range operations {
		switch op.OperationType {
		case OperationTypeAdd:
			addCount++
		case OperationTypeUpdate:
			updateCount++
		case OperationTypeRemove:
			removeCount++
		}
	}

	// 记录变更（使用交易确认时间）
	confirmTime := time.Now()
	m.lastUpdateTime = confirmTime
	m.changeHistory = append(m.changeHistory, ChangeRecord{
		Time:        confirmTime,
		Type:        "BPoS",
		Description: fmt.Sprintf("Synced BPoS nodes: %d Add, %d Update, %d Remove (filtered from %d total, tx: %s)", addCount, updateCount, removeCount, len(rpcProducers), txHash.Hex()),
		Details:     filteredProducers,
	})

	return true, nil
}

// buildNodeOperations 构建节点操作列表，确定每个节点的 OperationType
func (m *Monitor) buildNodeOperations(rpcProducers []Producer, contractNodes []BPoSNode) []NodeOperation {
	// 创建合约节点的映射 (使用 ownerPublicKey 作为 key)
	contractMap := make(map[string]BPoSNode)
	for _, node := range contractNodes {
		key := hex.EncodeToString(node.OwnerPublicKey)
		contractMap[key] = node
	}

	// 创建 RPC 生产者的映射 (使用 ownerPublicKey 作为 key)
	rpcMap := make(map[string]Producer)
	for _, producer := range rpcProducers {
		ownerPK, err := hexToBytes(producer.OwnerPublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse owner public key for producer %s: %v", producer.Nickname, err)
			continue
		}
		key := hex.EncodeToString(ownerPK)
		rpcMap[key] = producer
	}

	operations := make([]NodeOperation, 0)

	// 1. 处理 RPC 中的节点：Add 或 Update
	for _, producer := range rpcProducers {
		ownerPK, err := hexToBytes(producer.OwnerPublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse owner public key for producer %s: %v", producer.Nickname, err)
			continue
		}
		key := hex.EncodeToString(ownerPK)

		contractNode, exists := contractMap[key]
		opType := OperationTypeAdd
		if exists {
			// 检查是否需要更新（只比较 nickName、dposPublicKey、votes，ownerPublicKey 不会变）
			needsUpdate := false
			updateReasons := make([]string, 0)

			// 比较 NickName
			if contractNode.NickName != producer.Nickname {
				needsUpdate = true
				updateReasons = append(updateReasons, fmt.Sprintf("NickName: contract='%s' != rpc='%s'", contractNode.NickName, producer.Nickname))
			}

			// 比较 DPoSPublicKey
			nodePK, err := hexToBytes(producer.NodePublicKey)
			if err != nil {
				log.Printf("Warning: failed to parse node public key for producer %s: %v", producer.Nickname, err)
				continue
			}
			contractDPoSKey := hex.EncodeToString(contractNode.DPoSPublicKey)
			rpcDPoSKey := hex.EncodeToString(nodePK)
			if contractDPoSKey != rpcDPoSKey {
				needsUpdate = true
				updateReasons = append(updateReasons, fmt.Sprintf("DPoSPublicKey: contract='%s' != rpc='%s'", contractDPoSKey, rpcDPoSKey))
			}

			// 比较 Votes (使用 DPoSV2Votes)
			voteValue, err := parseVotes(producer.DPoSV2Votes)
			if err != nil {
				log.Printf("Warning: failed to parse votes for producer %s: %v", producer.Nickname, err)
				voteValue = big.NewInt(0)
			}
			if contractNode.Votes.Cmp(voteValue) != 0 {
				needsUpdate = true
				updateReasons = append(updateReasons, fmt.Sprintf("Votes: contract='%s' != rpc='%s' (DPoSV2Votes)", contractNode.Votes.String(), voteValue.String()))
			}

			if needsUpdate {
				opType = OperationTypeUpdate
				log.Printf("Node %s needs Update. Reasons: %v", producer.Nickname, updateReasons)
			} else {
				// 不需要更新，跳过
				log.Printf("Node %s (ownerPK=%s) no changes needed, skipping", producer.Nickname, key)
				continue
			}
		}

		// 解析数据
		nodePK, err := hexToBytes(producer.NodePublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse node public key for producer %s: %v", producer.Nickname, err)
			continue
		}

		voteValue, err := parseVotes(producer.DPoSV2Votes)
		if err != nil {
			log.Printf("Warning: failed to parse votes for producer %s: %v", producer.Nickname, err)
			voteValue = big.NewInt(0)
		}

		operations = append(operations, NodeOperation{
			NickName:       producer.Nickname,
			OwnerPublicKey: ownerPK,
			DPoSPublicKey:  nodePK,
			Votes:          voteValue,
			OperationType:  opType,
		})
	}

	// 2. 处理合约中存在但 RPC 中不存在的节点：Remove（含投票不足的节点）
	for _, contractNode := range contractNodes {
		key := hex.EncodeToString(contractNode.OwnerPublicKey)
		if _, exists := rpcMap[key]; !exists {
			operations = append(operations, NodeOperation{
				NickName:       contractNode.NickName,
				OwnerPublicKey: contractNode.OwnerPublicKey,
				DPoSPublicKey:  contractNode.DPoSPublicKey,
				Votes:          contractNode.Votes,
				OperationType:  OperationTypeRemove,
			})
		}
	}

	log.Printf("Built %d node operations: %d Add, %d Update, %d Remove", len(operations),
		countOperationsByType(operations, OperationTypeAdd),
		countOperationsByType(operations, OperationTypeUpdate),
		countOperationsByType(operations, OperationTypeRemove))

	return operations
}

// countOperationsByType 统计指定类型的操作数量
func countOperationsByType(operations []NodeOperation, opType OperationType) int {
	count := 0
	for _, op := range operations {
		if op.OperationType == opType {
			count++
		}
	}
	return count
}

// bposHasChanges 检查 BPoS 是否有变化
func (m *Monitor) bposHasChanges(rpcProducers []Producer, contractNodes []BPoSNode) bool {
	// 创建合约节点的映射 (使用 ownerPublicKey 作为 key)
	contractMap := make(map[string]BPoSNode)
	for _, node := range contractNodes {
		key := hex.EncodeToString(node.OwnerPublicKey)
		contractMap[key] = node
	}

	// 检查 RPC 中的每个生产者是否在合约中存在且信息一致
	for _, producer := range rpcProducers {
		ownerPK, err := hexToBytes(producer.OwnerPublicKey)
		if err != nil {
			continue
		}
		key := hex.EncodeToString(ownerPK)

		contractNode, exists := contractMap[key]
		if !exists {
			return true // 有新的节点
		}

		// 比较四个字段: ownerPK, bposPK (dposPublicKey), NickName, votes
		if contractNode.NickName != producer.Nickname {
			return true
		}

		nodePK, err := hexToBytes(producer.NodePublicKey)
		if err != nil {
			continue
		}
		if hex.EncodeToString(contractNode.DPoSPublicKey) != hex.EncodeToString(nodePK) {
			return true
		}

		voteValue, err := parseVotes(producer.DPoSV2Votes)
		if err != nil {
			continue
		}
		if contractNode.Votes.Cmp(voteValue) != 0 {
			return true
		}
	}

	// 检查是否有节点被移除
	if len(rpcProducers) != len(contractNodes) {
		return true
	}

	return false
}

// GetStatus 获取监控状态
func (m *Monitor) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"last_check_time":   m.lastCheckTime,
		"last_update_time":  m.lastUpdateTime,
		"change_count":      len(m.changeHistory),
		"has_changes_today": m.hasChangesToday(),
	}
}

// hasChangesToday 检查今天是否有变更
func (m *Monitor) hasChangesToday() bool {
	today := time.Now().Truncate(24 * time.Hour)
	for _, record := range m.changeHistory {
		if record.Time.Truncate(24 * time.Hour).Equal(today) {
			return true
		}
	}
	return false
}

// GetChangeHistory 获取变更历史
func (m *Monitor) GetChangeHistory() []ChangeRecord {
	return m.changeHistory
}

// codeToOwnerPublicKey 将 CR 的 code 转换为 ownerPublicKey
// CR 的 code 需要去掉头一个 byte 和尾一个 byte 才能是 ownerPublicKey
func codeToOwnerPublicKey(code string) ([]byte, error) {
	// 先转换为字节
	codeBytes, err := hexToBytes(code)
	if err != nil {
		return nil, fmt.Errorf("failed to parse code: %w", err)
	}

	// 检查长度，至少需要 2 个字节才能去掉首尾
	if len(codeBytes) < 2 {
		return nil, fmt.Errorf("code too short: %d bytes", len(codeBytes))
	}

	// 去掉头一个 byte 和尾一个 byte
	ownerPK := codeBytes[1 : len(codeBytes)-1]
	return ownerPK, nil
}
