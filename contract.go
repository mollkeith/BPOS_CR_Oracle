package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"bpos_cr_monitor/contract"
)

type ContractClient struct {
	crossClient *contract.CrossClient
	privateKey  *ecdsa.PrivateKey
	chainID     *big.Int
}

// NewContractClientFromKeystore 从 keystore 文件创建合约客户端
func NewContractClientFromKeystore(rpcURL, keystorePath, password string) (*ContractClient, error) {
	crossClient, err := contract.ConnectRPC(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	chainID, err := crossClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// 读取 keystore 文件
	keyJSON, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore file: %w", err)
	}

	// 解锁 keystore
	key, err := keystore.DecryptKey(keyJSON, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt keystore: %w", err)
	}

	return &ContractClient{
		crossClient: crossClient,
		privateKey:  key.PrivateKey,
		chainID:     chainID,
	}, nil
}

// NewContractClient 从私钥字符串创建合约客户端 (保留向后兼容)
func NewContractClient(rpcURL, privateKeyHex string) (*ContractClient, error) {
	crossClient, err := contract.ConnectRPC(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	chainID, err := crossClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// 移除 0x 前缀
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &ContractClient{
		crossClient: crossClient,
		privateKey:  privateKey,
		chainID:     chainID,
	}, nil
}

func (c *ContractClient) GetAuth() (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return nil, err
	}

	nonce, err := c.crossClient.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(c.privateKey.PublicKey))
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	gasPrice, err := c.crossClient.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	auth.GasPrice = gasPrice
	auth.GasLimit = 3000000

	return auth, nil
}

// GetAddress 获取钱包地址
func (c *ContractClient) GetAddress() common.Address {
	return crypto.PubkeyToAddress(c.privateKey.PublicKey)
}

// GetBalance 获取钱包余额
func (c *ContractClient) GetBalance() (*big.Int, error) {
	address := c.GetAddress()
	balance, err := c.crossClient.GetBalance(context.Background(), address)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// ContractExists 检查合约是否存在
func (c *ContractClient) ContractExists(address common.Address) (bool, error) {
	code, err := c.crossClient.GetCode(context.Background(), address)
	if err != nil {
		return false, fmt.Errorf("failed to get contract code: %w", err)
	}
	// code 是 hex 字符串，如果为 "0x" 或 "0x0" 表示合约不存在
	return code != "" && code != "0x" && code != "0x0", nil
}

// CRNode 表示合约中的 CR 节点
type CRNode struct {
	NickName       string
	OwnerPublicKey []byte
	DPoSPublicKey  []byte
	Exists         bool
}

// BPoSNode 表示合约中的 BPoS 节点
type BPoSNode struct {
	NickName       string
	OwnerPublicKey []byte
	DPoSPublicKey  []byte
	Votes          *big.Int
	Exists         bool
}

// CRPoolContract CR 合约接口
type CRPoolContract struct {
	*ContractClient
	address common.Address
	abi     abi.ABI
}

func NewCRPoolContract(client *ContractClient, address string) (*CRPoolContract, error) {
	abiStr, err := getCRPoolABI()
	if err != nil {
		return nil, fmt.Errorf("failed to load CRPool ABI: %w", err)
	}

	abiJSON, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CRPool ABI: %w", err)
	}

	return &CRPoolContract{
		ContractClient: client,
		address:        common.HexToAddress(address),
		abi:            abiJSON,
	}, nil
}

// GetAllNodes 获取所有 CR 节点
func (c *CRPoolContract) GetAllNodes() ([]CRNode, error) {
	ctx := context.Background()

	// 先检查合约是否存在
	exists, err := c.ContractExists(c.address)
	if err != nil {
		return nil, fmt.Errorf("failed to check if contract exists: %w", err)
	}
	if !exists {
		log.Printf("Contract at %s does not exist, returning empty array", c.address.Hex())
		return nil, fmt.Errorf("contract at %s does not exist", c.address.Hex())
	}

	// 构造调用数据
	data, err := c.abi.Pack("getAllNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to pack method getAllNodes: %w", err)
	}

	// 调用合约（使用 crossClient）
	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}
	result, err := c.crossClient.CallContract(ctx, msg, nil)
	if err != nil {
		// 如果调用失败，返回错误，因为拿不到数据不能提交
		return nil, fmt.Errorf("failed to call contract getAllNodes at %s: %w", c.address.Hex(), err)
	}

	// 如果结果为空，返回空数组
	if len(result) == 0 {
		return make([]CRNode, 0), nil
	}

	// 解包结果 - 使用 UnpackIntoInterface 直接解包到结构体数组
	type nodeResult struct {
		NickName       string
		OwnerPublicKey []byte
		DPoSPublicKey  []byte
		Exists         bool
	}

	var nodesResult []nodeResult
	err = c.abi.UnpackIntoInterface(&nodesResult, "getAllNodes", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	log.Printf("######## unpacked nodes count: %d", len(nodesResult))

	nodes := make([]CRNode, len(nodesResult))
	for i, r := range nodesResult {
		nodes[i] = CRNode{
			NickName:       r.NickName,
			OwnerPublicKey: r.OwnerPublicKey,
			DPoSPublicKey:  r.DPoSPublicKey,
			Exists:         r.Exists,
		}
	}

	return nodes, nil
}

// SetNodes 设置 CR 节点
func (c *CRPoolContract) SetNodes(nickNames []string, ownerPublicKeys [][]byte, dposPublicKeys [][]byte) (common.Hash, error) {
	ctx := context.Background()
	from := c.GetAddress()

	// 使用 ABI 打包方法调用数据
	fmt.Println("######## nickNames: %v", nickNames)
	// print ownerPublicKeys as hex and length
	for i, key := range ownerPublicKeys {
		fmt.Printf("######## ownerPublicKey[%d]: %v, length: %d\n", i, hex.EncodeToString(key), len(key))
	}
	// print dposPublicKeys as hex and length
	for i, key := range dposPublicKeys {
		fmt.Printf("######## dposPublicKey[%d]: %v, length: %d\n", i, hex.EncodeToString(key), len(key))
	}
	data, err := c.abi.Pack("setNodes", nickNames, ownerPublicKeys, dposPublicKeys)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack setNodes: %w", err)
	}

	// 获取 gasPrice
	gasPrice, err := c.crossClient.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get gas price: %w", err)
	}

	// 估算 gas
	msg := ethereum.CallMsg{
		From:     from,
		To:       &c.address,
		Data:     data,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
	}
	gasLimit, err := c.crossClient.EstimateGas(ctx, msg)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to estimate gas: %w", err)
	}
	gasLimit = gasLimit + gasLimit*2/10 // 增加 20% 的 gas

	// 获取 nonce
	nonce, err := c.crossClient.PendingNonceAt(ctx, from)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get nonce: %w", err)
	}

	// 创建交易
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &c.address,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})

	// 签名交易
	signer, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create signer: %w", err)
	}
	signedTx, err := signer.Signer(from, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// 编码交易
	rawTx, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode transaction: %w", err)
	}

	// 发送交易
	txHash, err := c.crossClient.SendRawTransaction(ctx, rawTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

// BPoSPoolContract BPoS 合约接口
type BPoSPoolContract struct {
	*ContractClient
	address common.Address
	abi     abi.ABI
}

func NewBPoSPoolContract(client *ContractClient, address string) (*BPoSPoolContract, error) {
	abiStr, err := getBPoSPoolABI()
	if err != nil {
		return nil, fmt.Errorf("failed to load BPoSPool ABI: %w", err)
	}

	abiJSON, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse BPoSPool ABI: %w", err)
	}

	return &BPoSPoolContract{
		ContractClient: client,
		address:        common.HexToAddress(address),
		abi:            abiJSON,
	}, nil
}

// GetAllNodes 获取所有 BPoS 节点
func (c *BPoSPoolContract) GetAllNodes() ([]BPoSNode, error) {
	ctx := context.Background()

	// 先检查合约是否存在
	exists, err := c.ContractExists(c.address)
	if err != nil {
		return nil, fmt.Errorf("failed to check if contract exists: %w", err)
	}
	if !exists {
		log.Printf("Contract at %s does not exist, returning empty array", c.address.Hex())
		return nil, fmt.Errorf("contract at %s does not exist", c.address.Hex())
	}

	// 构造调用数据
	data, err := c.abi.Pack("getAllNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to pack method getAllNodes: %w", err)
	}

	// 调用合约（使用 crossClient）
	msg := ethereum.CallMsg{
		To:   &c.address,
		Data: data,
	}
	result, err := c.crossClient.CallContract(ctx, msg, nil)
	if err != nil {
		// 如果调用失败，返回错误，因为拿不到数据不能提交
		return nil, fmt.Errorf("failed to call contract getAllNodes at %s: %w", c.address.Hex(), err)
	}

	// 如果结果为空，返回空数组
	if len(result) == 0 {
		return make([]BPoSNode, 0), nil
	}

	// 解包结果 - 使用 UnpackIntoInterface 直接解包到结构体数组
	type nodeResult struct {
		NickName       string
		OwnerPublicKey []byte
		DPoSPublicKey  []byte
		Votes          *big.Int
		Exists         bool
	}

	var nodesResult []nodeResult
	err = c.abi.UnpackIntoInterface(&nodesResult, "getAllNodes", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	log.Printf("######## unpacked nodes count: %d", len(nodesResult))

	nodes := make([]BPoSNode, len(nodesResult))
	for i, r := range nodesResult {
		nodes[i] = BPoSNode{
			NickName:       r.NickName,
			OwnerPublicKey: r.OwnerPublicKey,
			DPoSPublicKey:  r.DPoSPublicKey,
			Votes:          r.Votes,
			Exists:         r.Exists,
		}
	}

	return nodes, nil
}

// UpdateNodes 更新 BPoS 节点
func (c *BPoSPoolContract) UpdateNodes(ownerPublicKeys [][]byte, nickNames []string, dposPublicKeys [][]byte, votes []*big.Int) (common.Hash, error) {
	ctx := context.Background()
	from := c.GetAddress()

	// 使用 ABI 打包方法调用数据
	data, err := c.abi.Pack("updateNodes", ownerPublicKeys, nickNames, dposPublicKeys, votes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack updateNodes: %w", err)
	}

	// 获取 gasPrice
	gasPrice, err := c.crossClient.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get gas price: %w", err)
	}

	// 估算 gas
	msg := ethereum.CallMsg{
		From:     from,
		To:       &c.address,
		Data:     data,
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
	}
	gasLimit, err := c.crossClient.EstimateGas(ctx, msg)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to estimate gas: %w", err)
	}
	gasLimit = gasLimit + gasLimit*2/10 // 增加 20% 的 gas

	// 获取 nonce
	nonce, err := c.crossClient.PendingNonceAt(ctx, from)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get nonce: %w", err)
	}

	// 创建交易
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &c.address,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})

	// 签名交易
	signer, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create signer: %w", err)
	}
	signedTx, err := signer.Signer(from, tx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// 编码交易
	rawTx, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to encode transaction: %w", err)
	}

	// 发送交易
	txHash, err := c.crossClient.SendRawTransaction(ctx, rawTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

// Helper function to convert hex string to bytes
func hexToBytes(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	return hex.DecodeString(hexStr)
}

// Helper function to parse votes string to big.Int
func parseVotes(votesStr string) (*big.Int, error) {
	// 移除小数点,假设有8位小数
	parts := strings.Split(votesStr, ".")
	if len(parts) == 1 {
		parts = append(parts, "0")
	}

	// 确保小数部分有8位
	for len(parts[1]) < 8 {
		parts[1] += "0"
	}
	if len(parts[1]) > 8 {
		parts[1] = parts[1][:8]
	}

	combined := parts[0] + parts[1]
	value := new(big.Int)
	value, ok := value.SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse votes: %s", votesStr)
	}

	return value, nil
}
