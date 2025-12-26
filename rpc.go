package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type RPCClient struct {
	client *resty.Client
	url    string
}

func NewRPCClient(url string) *RPCClient {
	client := resty.New()
	client.SetTimeout(30 * time.Second)
	return &RPCClient{
		client: client,
		url:    url,
	}
}

type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
	ID      int             `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *RPCClient) Call(method string, params interface{}) (json.RawMessage, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	var resp RPCResponse
	httpResp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetResult(&resp).
		Post(c.url)

	if err != nil {
		return nil, fmt.Errorf("RPC request failed: %w", err)
	}

	if httpResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("RPC request failed with status: %d", httpResp.StatusCode())
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	return resp.Result, nil
}

// CRMember 表示 CR 成员信息
type CRMember struct {
	Code            string `json:"code"`
	CID             string `json:"cid"`
	DID             string `json:"did"`
	Nickname        string `json:"nickname"`
	URL             string `json:"url"`
	Location        uint64 `json:"location"`
	ImpeachmentVotes int64  `json:"impeachmentvotes"`
	DepositAmount   string `json:"depositamount"`
	DepositAddress  string `json:"depositaddress"`
	Penalty         int64  `json:"penalty"`
	Index           uint64 `json:"index"`
	State           string `json:"State"`
}

type ListCurrentCRsResponse struct {
	CRMembersInfo []CRMember `json:"crmembersinfo"`
	TotalCounts   uint64     `json:"totalcounts"`
}

// ListCurrentCRs 获取当前 CR 列表
func (c *RPCClient) ListCurrentCRs() ([]CRMember, error) {
	params := map[string]string{
		"state": "all",
	}

	result, err := c.Call("listcurrentcrs", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		CRMembersInfo []CRMember `json:"crmembersinfo"`
		TotalCounts   uint64     `json:"totalcounts"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CR response: %w", err)
	}

	return response.CRMembersInfo, nil
}

// Producer 表示 BPoS 生产者信息
type Producer struct {
	OwnerPublicKey  string `json:"ownerpublickey"`
	NodePublicKey   string `json:"nodepublickey"`
	Nickname        string `json:"nickname"`
	URL             string `json:"url"`
	Location        uint64 `json:"location"`
	Active          bool   `json:"active"`
	Votes           string `json:"votes"`
	State           string `json:"state"`
	OnDuty          string `json:"onduty"`
	Identity        string `json:"identity"`
	RegisterHeight  uint32 `json:"registerheight"`
	CancelHeight    uint32 `json:"cancelheight"`
	InactiveHeight  uint32 `json:"inactiveheight"`
	IllegalHeight   uint32 `json:"illegalheight"`
	Index           uint64 `json:"index"`
}

type ListProducersResponse struct {
	Producers        []Producer `json:"producers"`
	TotalVotes       string     `json:"totalvotes"`
	TotalDPoSV1Votes string     `json:"totaldposv1votes"`
	TotalDPoSV2Votes string     `json:"totaldposv2votes"`
	TotalCounts      uint64     `json:"totalcounts"`
}

// ListProducers 获取生产者列表
func (c *RPCClient) ListProducers() ([]Producer, error) {
	params := map[string]interface{}{
		"start":    0,
		"limit":    1000, // 获取足够多的数据
		"identity": "all",
		"state":    "all",
	}

	result, err := c.Call("listproducers", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Producers        []Producer `json:"producers"`
		TotalVotes       string     `json:"totalvotes"`
		TotalDPoSV1Votes string     `json:"totaldposv1votes"`
		TotalDPoSV2Votes string     `json:"totaldposv2votes"`
		TotalCounts      uint64     `json:"totalcounts"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal producers response: %w", err)
	}

	return response.Producers, nil
}

// Helper function to make HTTP POST request (fallback)
func makeHTTPRequest(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

