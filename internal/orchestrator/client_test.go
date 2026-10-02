package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPAnalysisClient_Analyze_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/analysis" {
			t.Fatalf("expected /analysis, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		responseJSON := `{
			"wallet_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			"addresses": [
				{"address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed", "role": "suspect"},
				{"address": "0xdestination", "role": "destination"}
			],
			"transactions": [
				{
					"tx_hash": "0xtx1",
					"tx_time": "2026-09-16T18:30:00Z",
					"amount": "1.500000000000000000",
					"trace_depth": 1,
					"raw_reference": "https://explorer/tx/0xtx1"
				}
			],
			"fund_flow": [
				{
					"tx_hash": "0xtx1",
					"source_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
					"destination_address": "0xdestination",
					"amount": "1.500000000000000000"
				}
			],
			"vasp_attribution": {
				"name": "Binance",
				"type": "exchange",
				"matched_address": "0xdestination",
				"attribution_status": "known"
			}
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer server.Close()

	client := NewHTTPAnalysisClient(server.URL, server.Client())
	resp, err := client.Analyze(context.Background(), AnalysisRequest{
		WalletAddress: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
		MaxDepth:      3,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.WalletAddress != "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed" {
		t.Fatalf("expected wallet address 0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed, got %s", resp.WalletAddress)
	}
	if len(resp.Addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(resp.Addresses))
	}
	if len(resp.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(resp.Transactions))
	}
	if len(resp.FundFlow) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(resp.FundFlow))
	}
	if resp.VASPAttribution == nil || resp.VASPAttribution.Name != "Binance" {
		t.Fatalf("expected Binance VASP attribution, got %v", resp.VASPAttribution)
	}
}

func TestHTTPAnalysisClient_Analyze_ErrorPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":"ANALYSIS_FAILED","message":"provider timeout"}}`))
	}))
	defer server.Close()

	client := NewHTTPAnalysisClient(server.URL, server.Client())
	_, err := client.Analyze(context.Background(), AnalysisRequest{
		WalletAddress: "0x123",
		MaxDepth:      3,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedMsg := "ANALYSIS_FAILED: provider timeout"
	if err.Error() != expectedMsg {
		t.Fatalf("expected error message %s, got %s", expectedMsg, err.Error())
	}
}

func TestHTTPAnalysisClient_Analyze_StatusErrorWithoutJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal crash"))
	}))
	defer server.Close()

	client := NewHTTPAnalysisClient(server.URL, server.Client())
	_, err := client.Analyze(context.Background(), AnalysisRequest{
		WalletAddress: "0x123",
		MaxDepth:      3,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHTTPAnalysisClient_Analyze_NetworkError(t *testing.T) {
	client := NewHTTPAnalysisClient("http://127.0.0.1:59999", &http.Client{Timeout: 50 * time.Millisecond})
	_, err := client.Analyze(context.Background(), AnalysisRequest{
		WalletAddress: "0x123",
		MaxDepth:      3,
	})

	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}
