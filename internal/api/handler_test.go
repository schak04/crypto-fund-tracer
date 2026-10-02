package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
	"github.com/schak04/crypto-fund-tracer/internal/repository"
	"github.com/schak04/crypto-fund-tracer/internal/service"
)

type mockService struct {
	createInvestigationFunc       func(ctx context.Context, suspectAddress string) (*repository.Investigation, error)
	createAndRunInvestigationFunc func(ctx context.Context, suspectAddress string, maxDepth int) (*repository.InvestigationDetails, error)
	getInvestigationFunc          func(ctx context.Context, id string) (*repository.InvestigationDetails, error)
	saveResultFunc                func(ctx context.Context, id string, params repository.SaveAnalysisParams) error
	failInvestigationFunc         func(ctx context.Context, id string, reason string) error
}

func (m *mockService) CreateInvestigation(ctx context.Context, suspectAddress string) (*repository.Investigation, error) {
	if m.createInvestigationFunc != nil {
		return m.createInvestigationFunc(ctx, suspectAddress)
	}
	return nil, errors.New("unexpected call to CreateInvestigation")
}

func (m *mockService) CreateAndRunInvestigation(ctx context.Context, suspectAddress string, maxDepth int) (*repository.InvestigationDetails, error) {
	if m.createAndRunInvestigationFunc != nil {
		return m.createAndRunInvestigationFunc(ctx, suspectAddress, maxDepth)
	}
	inv, err := m.CreateInvestigation(ctx, suspectAddress)
	if err != nil {
		return nil, err
	}
	return m.GetInvestigation(ctx, inv.ID)
}

func (m *mockService) GetInvestigation(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
	if m.getInvestigationFunc != nil {
		return m.getInvestigationFunc(ctx, id)
	}
	return nil, errors.New("unexpected call to GetInvestigation")
}

func (m *mockService) SaveInvestigationResult(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
	if m.saveResultFunc != nil {
		return m.saveResultFunc(ctx, id, params)
	}
	return errors.New("unexpected call to SaveInvestigationResult")
}

func (m *mockService) FailInvestigation(ctx context.Context, id string, reason string) error {
	if m.failInvestigationFunc != nil {
		return m.failInvestigationFunc(ctx, id, reason)
	}
	return errors.New("unexpected call to FailInvestigation")
}

func TestHandler_CreateInvestigation_Success(t *testing.T) {
	validID := "c56a4180-65aa-42ec-a945-5fd21dec0538"
	suspect := "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
	now := time.Now().UTC()

	mock := &mockService{
		createInvestigationFunc: func(ctx context.Context, suspectAddress string) (*repository.Investigation, error) {
			if suspectAddress != suspect {
				t.Fatalf("expected address %s, got %s", suspect, suspectAddress)
			}
			return &repository.Investigation{
				ID:        validID,
				Status:    repository.StatusRunning,
				CreatedAt: now,
			}, nil
		},
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			if id != validID {
				t.Fatalf("expected ID %s, got %s", validID, id)
			}
			return &repository.InvestigationDetails{
				ID:             validID,
				Status:         repository.StatusRunning,
				SuspectAddress: suspect,
				CreatedAt:      now,
				Transactions:   []repository.TransactionItem{},
				FundFlow:       []repository.FlowEdgeItem{},
			}, nil
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	expectedLocation := "/api/v1/investigations/" + validID
	if loc := resp.Header.Get("Location"); loc != expectedLocation {
		t.Fatalf("expected Location %s, got %s", expectedLocation, loc)
	}

	var details repository.InvestigationDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if details.ID != validID {
		t.Fatalf("expected ID %s, got %s", validID, details.ID)
	}
	if details.Status != repository.StatusRunning {
		t.Fatalf("expected status running, got %s", details.Status)
	}
}

func TestHandler_CreateInvestigation_MissingContentType(t *testing.T) {
	h := NewHandler(&mockService{})
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	// no Content-Type header

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvalidRequest {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvalidRequest, errResp.Error.Code)
	}
}

func TestHandler_CreateInvestigation_MalformedJSON(t *testing.T) {
	h := NewHandler(&mockService{})
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{invalid-json}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvalidRequest {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvalidRequest, errResp.Error.Code)
	}
}

func TestHandler_CreateInvestigation_InvalidAddress(t *testing.T) {
	h := NewHandler(&mockService{})
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"not-an-address"}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvalidAddress {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvalidAddress, errResp.Error.Code)
	}
}

func TestHandler_CreateInvestigation_InvalidDepth(t *testing.T) {
	h := NewHandler(&mockService{})
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed","max_depth":10}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvalidRequest {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvalidRequest, errResp.Error.Code)
	}
}

func TestHandler_CreateInvestigation_ServiceError(t *testing.T) {
	mock := &mockService{
		createInvestigationFunc: func(ctx context.Context, suspectAddress string) (*repository.Investigation, error) {
			return nil, errors.New("database connection failed")
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInternalError {
		t.Fatalf("expected code %s, got %s", apperror.CodeInternalError, errResp.Error.Code)
	}
}

func TestHandler_CreateInvestigation_OrchestrationFailure_502(t *testing.T) {
	invID := "inv-failed-123"
	mock := &mockService{
		createAndRunInvestigationFunc: func(ctx context.Context, suspectAddress string, maxDepth int) (*repository.InvestigationDetails, error) {
			cause := errors.New("provider timeout")
			return nil, apperror.NewAnalysisServiceError("Blockchain analysis component failed to complete fund tracing: provider timeout.", invID, cause)
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	payload := []byte(`{"wallet_address":"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"}`)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/investigations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeAnalysisServiceError {
		t.Fatalf("expected code %s, got %s", apperror.CodeAnalysisServiceError, errResp.Error.Code)
	}
	if errResp.Error.InvestigationID != invID {
		t.Fatalf("expected investigation ID %s, got %s", invID, errResp.Error.InvestigationID)
	}
}

func TestHandler_GetInvestigation_Success(t *testing.T) {
	validID := "c56a4180-65aa-42ec-a945-5fd21dec0538"
	mock := &mockService{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			if id != validID {
				t.Fatalf("expected ID %s, got %s", validID, id)
			}
			return &repository.InvestigationDetails{
				ID:             validID,
				Status:         repository.StatusCompleted,
				SuspectAddress: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
				Transactions:   []repository.TransactionItem{},
				FundFlow:       []repository.FlowEdgeItem{},
			}, nil
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/investigations/" + validID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var details repository.InvestigationDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if details.ID != validID {
		t.Fatalf("expected ID %s, got %s", validID, details.ID)
	}
}

func TestHandler_GetInvestigation_InvalidUUID(t *testing.T) {
	h := NewHandler(&mockService{})
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/investigations/not-a-valid-uuid")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvalidInvestigationID {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvalidInvestigationID, errResp.Error.Code)
	}
}

func TestHandler_GetInvestigation_NotFound(t *testing.T) {
	validID := "c56a4180-65aa-42ec-a945-5fd21dec0538"
	mock := &mockService{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			return nil, service.ErrNotFound
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/investigations/" + validID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInvestigationNotFound {
		t.Fatalf("expected code %s, got %s", apperror.CodeInvestigationNotFound, errResp.Error.Code)
	}
	expectedMsg := "Investigation with ID '" + validID + "' does not exist."
	if errResp.Error.Message != expectedMsg {
		t.Fatalf("expected message %s, got %s", expectedMsg, errResp.Error.Message)
	}
}

func TestHandler_GetInvestigation_ServiceError(t *testing.T) {
	validID := "c56a4180-65aa-42ec-a945-5fd21dec0538"
	mock := &mockService{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			return nil, errors.New("read replica failure")
		},
	}

	h := NewHandler(mock)
	server := httptest.NewServer(h.Routes())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/investigations/" + validID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}

	var errResp apperror.Response
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errResp.Error.Code != apperror.CodeInternalError {
		t.Fatalf("expected code %s, got %s", apperror.CodeInternalError, errResp.Error.Code)
	}
}
