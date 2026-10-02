package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// payload sent to POST /analysis
type AnalysisRequest struct {
	WalletAddress string `json:"wallet_address"`
	MaxDepth      int    `json:"max_depth"`
}

// an address and its discovered role
type AddressItem struct {
	Address string `json:"address"`
	Role    string `json:"role"`
}

// a normalized transaction returned from tracing
type TransactionItem struct {
	TxHash       string    `json:"tx_hash"`
	TxTime       time.Time `json:"tx_time"`
	Amount       string    `json:"amount"`
	TraceDepth   int       `json:"trace_depth"`
	RawReference string    `json:"raw_reference"`
}

// a directed fund transfer between two addresses
type FlowEdgeItem struct {
	TxHash             string `json:"tx_hash"`
	SourceAddress      string `json:"source_address"`
	DestinationAddress string `json:"destination_address"`
	Amount             string `json:"amount"`
}

// exchange or entity attribution details
type VASPAttributionItem struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	MatchedAddress    string `json:"matched_address"`
	AttributionStatus string `json:"attribution_status"`
}

// the structured analysis graph returned by the analysis component
type AnalysisResponse struct {
	WalletAddress   string               `json:"wallet_address"`
	Addresses       []AddressItem        `json:"addresses"`
	Transactions    []TransactionItem    `json:"transactions"`
	FundFlow        []FlowEdgeItem       `json:"fund_flow"`
	VASPAttribution *VASPAttributionItem `json:"vasp_attribution"`
}

// failure payloads returned by the analysis component
type AnalysisErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// the integration boundary to the blockchain-analysis component
type AnalysisClient interface {
	Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error)
}

// communicates with the blockchain analysis service via HTTP
type HTTPAnalysisClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPAnalysisClient(baseURL string, client *http.Client) *HTTPAnalysisClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPAnalysisClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: client,
	}
}

func (c *HTTPAnalysisClient) Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
	endpoint := c.baseURL + "/analysis"
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling analysis request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("building analysis request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing analysis request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading analysis response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errPayload AnalysisErrorResponse
		if jsonErr := json.Unmarshal(body, &errPayload); jsonErr == nil && errPayload.Error.Message != "" {
			return nil, fmt.Errorf("%s: %s", errPayload.Error.Code, errPayload.Error.Message)
		}
		return nil, fmt.Errorf("analysis service returned status %d: %s", resp.StatusCode, string(body))
	}

	var analysisResp AnalysisResponse
	if err := json.Unmarshal(body, &analysisResp); err != nil {
		return nil, fmt.Errorf("decoding analysis response: %w", err)
	}

	if analysisResp.WalletAddress == "" {
		return nil, errors.New("analysis response missing wallet_address")
	}

	return &analysisResp, nil
}
