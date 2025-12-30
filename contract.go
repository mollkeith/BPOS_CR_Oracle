package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type ContractClient struct {
	client     *ethclient.Client
	privateKey *ecdsa.PrivateKey
	chainID    *big.Int
}

// NewContractClientFromKeystore 从 keystore 文件创建合约客户端
func NewContractClientFromKeystore(rpcURL, keystorePath, password string) (*ContractClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	chainID, err := client.NetworkID(context.Background())
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
		client:     client,
		privateKey: key.PrivateKey,
		chainID:    chainID,
	}, nil
}

// NewContractClient 从私钥字符串创建合约客户端 (保留向后兼容)
func NewContractClient(rpcURL, privateKeyHex string) (*ContractClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	chainID, err := client.NetworkID(context.Background())
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
		client:     client,
		privateKey: privateKey,
		chainID:    chainID,
	}, nil
}

func (c *ContractClient) GetAuth() (*bind.TransactOpts, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return nil, err
	}

	nonce, err := c.client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(c.privateKey.PublicKey))
	if err != nil {
		return nil, err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	gasPrice, err := c.client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	auth.GasPrice = gasPrice
	auth.GasLimit = 3000000

	return auth, nil
}

// CRNode 表示合约中的 CR 节点
type CRNode struct {
	NickName      string
	OwnerPublicKey []byte
	DPoSPublicKey []byte
	Exists        bool
}

// BPoSNode 表示合约中的 BPoS 节点
type BPoSNode struct {
	NickName      string
	OwnerPublicKey []byte
	DPoSPublicKey []byte
	Votes         *big.Int
	Exists        bool
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
	contract := bind.NewBoundContract(c.address, c.abi, c.client, c.client, c.client)
	
	type nodeResult struct {
		NickName      string
		OwnerPublicKey []byte
		DPoSPublicKey []byte
		Exists        bool
	}
	
	var result []nodeResult

	opts := &bind.CallOpts{Context: context.Background()}
	err := contract.Call(opts, &result, "getAllNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to call getAllNodes: %w", err)
	}

	nodes := make([]CRNode, len(result))
	for i, r := range result {
		nodes[i] = CRNode{
			NickName:      r.NickName,
			OwnerPublicKey: r.OwnerPublicKey,
			DPoSPublicKey: r.DPoSPublicKey,
			Exists:        r.Exists,
		}
	}

	return nodes, nil
}

// SetNodes 设置 CR 节点
func (c *CRPoolContract) SetNodes(nickNames []string, ownerPublicKeys [][]byte, dposPublicKeys [][]byte) (*types.Transaction, error) {
	auth, err := c.GetAuth()
	if err != nil {
		return nil, err
	}

	contract := bind.NewBoundContract(c.address, c.abi, c.client, c.client, c.client)
	
	tx, err := contract.Transact(auth, "setNodes", nickNames, ownerPublicKeys, dposPublicKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to call setNodes: %w", err)
	}

	return tx, nil
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
	contract := bind.NewBoundContract(c.address, c.abi, c.client, c.client, c.client)
	
	type nodeResult struct {
		NickName      string
		OwnerPublicKey []byte
		DPoSPublicKey []byte
		Votes         *big.Int
		Exists        bool
	}
	
	var result []nodeResult

	opts := &bind.CallOpts{Context: context.Background()}
	err := contract.Call(opts, &result, "getAllNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to call getAllNodes: %w", err)
	}

	nodes := make([]BPoSNode, len(result))
	for i, r := range result {
		nodes[i] = BPoSNode{
			NickName:      r.NickName,
			OwnerPublicKey: r.OwnerPublicKey,
			DPoSPublicKey: r.DPoSPublicKey,
			Votes:         r.Votes,
			Exists:        r.Exists,
		}
	}

	return nodes, nil
}

// UpdateNodes 更新 BPoS 节点
func (c *BPoSPoolContract) UpdateNodes(ownerPublicKeys [][]byte, nickNames []string, dposPublicKeys [][]byte, votes []*big.Int) (*types.Transaction, error) {
	auth, err := c.GetAuth()
	if err != nil {
		return nil, err
	}

	contract := bind.NewBoundContract(c.address, c.abi, c.client, c.client, c.client)
	
	tx, err := contract.Transact(auth, "updateNodes", ownerPublicKeys, nickNames, dposPublicKeys, votes)
	if err != nil {
		return nil, fmt.Errorf("failed to call updateNodes: %w", err)
	}

	return tx, nil
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

