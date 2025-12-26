package main

import (
	"encoding/json"
	"os"
)

// 从 JSON 文件加载 ABI
func loadABIFromFile(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}

	if err := json.Unmarshal(data, &artifact); err != nil {
		return "", err
	}

	return string(artifact.ABI), nil
}

// 获取 CRPool ABI
func getCRPoolABI() (string, error) {
	return loadABIFromFile("abi/CRPool.json")
}

// 获取 BPoSPool ABI
func getBPoSPoolABI() (string, error) {
	return loadABIFromFile("abi/BPoSPool.json")
}

