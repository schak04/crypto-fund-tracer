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

	"github.com/schak04/crypto-fund-tracer/internal/analysis"
)

// re-export contract types from internal/analysis
type AnalysisRequest = analysis.Request
type AnalysisResponse = analysis.Response
type AddressItem = analysis.AddressItem
type TransactionItem = analysis.TransactionItem
type FlowEdgeItem = analysis.FlowEdgeItem
type VASPAttributionItem = analysis.VASPAttributionItem
type AnalysisErrorResponse = analysis.ErrorResponse

// integration boundary to the blockchain-analysis component
type AnalysisClient interface {
	Analyze(ctx context.Context, req analysis.Request) (*analysis.Response, error)
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

func (c *HTTPAnalysisClient) Analyze(ctx context.Context, req analysis.Request) (*analysis.Response, error) {
	endpoint := c.baseURL + analysis.EndpointAnalysis
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
		var errPayload analysis.ErrorResponse
		if jsonErr := json.Unmarshal(body, &errPayload); jsonErr == nil && errPayload.Error.Message != "" {
			return nil, fmt.Errorf("%s: %s", errPayload.Error.Code, errPayload.Error.Message)
		}
		return nil, fmt.Errorf("analysis service returned status %d: %s", resp.StatusCode, string(body))
	}

	var analysisResp analysis.Response
	if err := json.Unmarshal(body, &analysisResp); err != nil {
		return nil, fmt.Errorf("decoding analysis response: %w", err)
	}

	if analysisResp.WalletAddress == "" {
		return nil, errors.New("analysis response missing wallet_address")
	}

	return &analysisResp, nil
}
