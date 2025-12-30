package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"time"
)

type Monitor struct {
	config          *Config
	mainRPC         *RPCClient
	pgRPC           *RPCClient
	crContract      *CRPoolContract
	bposContract    *BPoSPoolContract
	lastCheckTime   time.Time
	lastUpdateTime  time.Time
	changeHistory   []ChangeRecord
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
		config:        config,
		mainRPC:      mainRPC,
		pgRPC:        pgRPC,
		crContract:   crContract,
		bposContract: bposContract,
		changeHistory: make([]ChangeRecord, 0),
	}, nil
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

	// 从合约获取当前 CR 列表
	contractCRs, err := m.crContract.GetAllNodes()
	if err != nil {
		return false, fmt.Errorf("failed to get CRs from contract: %w", err)
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
		
		ownerPK, err := hexToBytes(cr.Code)
		if err != nil {
			log.Printf("Warning: failed to parse owner public key for CR %s: %v", cr.Nickname, err)
			continue
		}
		ownerPublicKeys = append(ownerPublicKeys, ownerPK)

		// 对于 CR, dposPublicKey 可能和 ownerPublicKey 相同,或者需要从其他地方获取
		// 根据需求文档, bposPK 就是 nodepublickey,但 CR 可能没有 nodepublickey
		// 这里假设使用 code 作为 dposPublicKey
		dposPublicKeys = append(dposPublicKeys, ownerPK)
	}

	// 调用合约更新
	tx, err := m.crContract.SetNodes(nickNames, ownerPublicKeys, dposPublicKeys)
	if err != nil {
		return false, fmt.Errorf("failed to call setNodes: %w", err)
	}

	log.Printf("CR update transaction sent: %s", tx.Hash().Hex())

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

	// 检查 RPC 中的每个 CR 是否在合约中存在且信息一致
	for _, cr := range rpcCRs {
		ownerPK, err := hexToBytes(cr.Code)
		if err != nil {
			continue
		}
		key := hex.EncodeToString(ownerPK)

		contractNode, exists := contractMap[key]
		if !exists {
			return true // 有新的 CR
		}

		// 比较三个字段: ownerPK, bposPK (dposPublicKey), NickName
		if contractNode.NickName != cr.Nickname {
			return true
		}

		// 比较 dposPublicKey (这里假设使用 code)
		if hex.EncodeToString(contractNode.DPoSPublicKey) != hex.EncodeToString(ownerPK) {
			return true
		}
	}

	// 检查是否有 CR 被移除
	if len(rpcCRs) != len(contractCRs) {
		return true
	}

	return false
}

// checkAndUpdateBPoS 检查并更新 BPoS
func (m *Monitor) checkAndUpdateBPoS() (bool, error) {
	// 从 RPC 获取当前生产者列表
	rpcProducers, err := m.mainRPC.ListProducers()
	if err != nil {
		return false, fmt.Errorf("failed to get producers from RPC: %w", err)
	}

	// 从合约获取当前 BPoS 节点列表
	contractNodes, err := m.bposContract.GetAllNodes()
	if err != nil {
		return false, fmt.Errorf("failed to get BPoS nodes from contract: %w", err)
	}

	// 比较是否有变化
	if !m.bposHasChanges(rpcProducers, contractNodes) {
		return false, nil
	}

	// 准备更新数据
	ownerPublicKeys := make([][]byte, 0, len(rpcProducers))
	nickNames := make([]string, 0, len(rpcProducers))
	dposPublicKeys := make([][]byte, 0, len(rpcProducers))
	votes := make([]*big.Int, 0, len(rpcProducers))

	for _, producer := range rpcProducers {
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
	tx, err := m.bposContract.UpdateNodes(ownerPublicKeys, nickNames, dposPublicKeys, votes)
	if err != nil {
		return false, fmt.Errorf("failed to call updateNodes: %w", err)
	}

	log.Printf("BPoS update transaction sent: %s", tx.Hash().Hex())

	// 记录变更
	m.changeHistory = append(m.changeHistory, ChangeRecord{
		Time:        time.Now(),
		Type:        "BPoS",
		Description: fmt.Sprintf("Updated %d BPoS nodes", len(rpcProducers)),
		Details:     rpcProducers,
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
		"last_check_time":  m.lastCheckTime,
		"last_update_time":  m.lastUpdateTime,
		"change_count":      len(m.changeHistory),
		"has_changes_today": m.hasChangesToday(),
	}
}

// hasChangesToday 检查今天是否有变更
func (m *Monitor) hasChangesToday() bool {
	today := time.Now().Truncate(24 * time.Hour)
	for _, record := range m.changeHistory {
		if record.Time.Truncate(24*time.Hour).Equal(today) {
			return true
		}
	}
	return false
}

// GetChangeHistory 获取变更历史
func (m *Monitor) GetChangeHistory() []ChangeRecord {
	return m.changeHistory
}

