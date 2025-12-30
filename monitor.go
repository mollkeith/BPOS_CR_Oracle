package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
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
		m.lastUpdateTime = time.Now()
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

	log.Printf("CR update transaction sent: %s", txHash.Hex())

	// 记录变更
	m.changeHistory = append(m.changeHistory, ChangeRecord{
		Time:        time.Now(),
		Type:        "CR",
		Description: fmt.Sprintf("Updated %d CR nodes", len(rpcCRs)),
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
		// 过滤条件：State == "Active", active == true, dposv2votes > 80000
		if producer.State != "Active" || !producer.Active {
			continue
		}

		// 解析 dposv2votes 并检查是否 > 配置的最小值
		dposv2Votes, err := parseVotes(producer.DPoSV2Votes)
		if err != nil {
			continue
		}

		// 从配置获取最小投票数
		minVotes, err := m.config.BPoS.GetMinDPoSV2Votes()
		if err != nil {
			log.Printf("Warning: failed to parse min_dposv2_votes from config: %v, using default 80000", err)
			minVotes = big.NewInt(8000000000000) // 默认 80000 * 10^8
		}
		if dposv2Votes.Cmp(minVotes) <= 0 {
			continue
		}

		filteredProducers = append(filteredProducers, producer)
	}

	minVotesStr := m.config.BPoS.MinDPoSV2Votes
	if minVotesStr == "" {
		minVotesStr = "80000"
	}
	log.Printf("Filtered BPoS producers: %d out of %d meet criteria (State=Active, active=true, dposv2votes>%s)", len(filteredProducers), len(rpcProducers), minVotesStr)

	// 比较是否有变化（只比较过滤后的生产者）
	if !m.bposHasChanges(filteredProducers, contractNodes) {
		return false, nil
	}

	// 准备更新数据 - 只更新满足条件的节点
	ownerPublicKeys := make([][]byte, 0, len(filteredProducers))
	nickNames := make([]string, 0, len(filteredProducers))
	dposPublicKeys := make([][]byte, 0, len(filteredProducers))
	votes := make([]*big.Int, 0, len(filteredProducers))

	for _, producer := range filteredProducers {
		ownerPK, err := hexToBytes(producer.OwnerPublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse owner public key for producer %s: %v", producer.Nickname, err)
			continue
		}
		ownerPublicKeys = append(ownerPublicKeys, ownerPK)

		nickNames = append(nickNames, producer.Nickname)

		nodePK, err := hexToBytes(producer.NodePublicKey)
		if err != nil {
			log.Printf("Warning: failed to parse node public key for producer %s: %v", producer.Nickname, err)
			continue
		}
		dposPublicKeys = append(dposPublicKeys, nodePK)

		voteValue, err := parseVotes(producer.Votes)
		if err != nil {
			log.Printf("Warning: failed to parse votes for producer %s: %v", producer.Nickname, err)
			voteValue = big.NewInt(0)
		}
		votes = append(votes, voteValue)
	}

	// 调用合约更新
	txHash, err := m.bposContract.UpdateNodes(ownerPublicKeys, nickNames, dposPublicKeys, votes)
	if err != nil {
		return false, fmt.Errorf("failed to call updateNodes: %w", err)
	}

	log.Printf("BPoS update transaction sent: %s", txHash.Hex())

	// 记录变更
	m.changeHistory = append(m.changeHistory, ChangeRecord{
		Time:        time.Now(),
		Type:        "BPoS",
		Description: fmt.Sprintf("Updated %d BPoS nodes (filtered from %d total)", len(ownerPublicKeys), len(rpcProducers)),
		Details:     filteredProducers,
	})

	return true, nil
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

		voteValue, err := parseVotes(producer.Votes)
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
