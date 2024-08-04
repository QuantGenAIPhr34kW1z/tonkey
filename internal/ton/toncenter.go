package ton

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TonCenter struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type tonCenterResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

type addressInfo struct {
	Balance string `json:"balance"`
	State   string `json:"state"`
}

type sendBocResult struct {
	Hash string `json:"hash"`
}

func NewTonCenter(baseURL string) *TonCenter {
	return &TonCenter{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func NewTonCenterWithKey(baseURL, apiKey string) *TonCenter {
	tc := NewTonCenter(baseURL)
	tc.APIKey = apiKey
	return tc
}

func (t *TonCenter) doRequest(ctx context.Context, method string, params url.Values) (json.RawMessage, error) {
	reqURL := fmt.Sprintf("%s/%s", t.BaseURL, method)

	if t.APIKey != "" {
		if params == nil {
			params = url.Values{}
		}
		params.Set("api_key", t.APIKey)
	}

	if params != nil && len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tcResp tonCenterResponse
	if err := json.Unmarshal(body, &tcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !tcResp.OK {
		return nil, fmt.Errorf("TON Center API error: %s", tcResp.Error)
	}

	return tcResp.Result, nil
}

func (t *TonCenter) Balance(ctx context.Context, addr string) (int64, error) {
	params := url.Values{}
	params.Set("address", addr)

	result, err := t.doRequest(ctx, "getAddressInformation", params)
	if err != nil {
		return 0, err
	}

	var info addressInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return 0, err
	}

	// Parse balance (in nanoTON)
	var balance int64
	_, err = fmt.Sscanf(info.Balance, "%d", &balance)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance: %w", err)
	}

	return balance, nil
}

func (t *TonCenter) Send(ctx context.Context, from, to string, amount int64) (string, error) {
	// Note: TON Center's sendBoc requires a pre-signed BOC (Bag of Cells)
	// This is a simplified implementation. In production, you would need:
	// 1. Access to the sender's private key
	// 2. Build and sign the transaction using TON SDK
	// 3. Serialize to BOC format
	// 4. Submit via sendBoc

	return "", errors.New("Send not fully implemented: requires BOC creation with private keys")
}

func (t *TonCenter) TxStatus(ctx context.Context, id string) (string, error) {
	// Note: This would query getTransactions and search for the transaction by hash
	// The implementation depends on how you store transaction hashes
	return "", errors.New("TxStatus not fully implemented: requires transaction lookup")
}

// GetTransactions retrieves transactions for an address
func (t *TonCenter) GetTransactions(ctx context.Context, addr string, limit int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("address", addr)
	params.Set("limit", fmt.Sprintf("%d", limit))

	return t.doRequest(ctx, "getTransactions", params)
}

// GetMasterchainInfo retrieves masterchain info
func (t *TonCenter) GetMasterchainInfo(ctx context.Context) (json.RawMessage, error) {
	return t.doRequest(ctx, "getMasterchainInfo", nil)
}
