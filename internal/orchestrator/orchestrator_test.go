package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
	"github.com/schak04/crypto-fund-tracer/internal/repository"
)

type mockAnalysisClient struct {
	analyzeFunc func(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error)
}

func (m *mockAnalysisClient) Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, req)
	}
	return nil, errors.New("unexpected call to Analyze")
}

type mockRepo struct {
	failInvestigationFunc       func(ctx context.Context, id, reason string) error
	saveInvestigationResultFunc func(ctx context.Context, id string, params repository.SaveAnalysisParams) error
	getInvestigationFunc        func(ctx context.Context, id string) (*repository.InvestigationDetails, error)
}

func (m *mockRepo) CreateInvestigation(ctx context.Context, suspectAddress string, initialStatus string) (*repository.Investigation, error) {
	return nil, errors.New("not implemented")
}

func (m *mockRepo) GetInvestigation(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
	if m.getInvestigationFunc != nil {
		return m.getInvestigationFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRepo) SaveInvestigationResult(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
	if m.saveInvestigationResultFunc != nil {
		return m.saveInvestigationResultFunc(ctx, id, params)
	}
	return errors.New("not implemented")
}

func (m *mockRepo) FailInvestigation(ctx context.Context, id string, reason string) error {
	if m.failInvestigationFunc != nil {
		return m.failInvestigationFunc(ctx, id, reason)
	}
	return errors.New("not implemented")
}

func (m *mockRepo) UpsertAddress(ctx context.Context, address string) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockRepo) UpsertVASP(ctx context.Context, name, vaspType string) (string, error) {
	return "", errors.New("not implemented")
}

func (m *mockRepo) AssociateVASPAddress(ctx context.Context, vaspID, addressID, status string) error {
	return errors.New("not implemented")
}

func TestAnalysisOrchestrator_RunAnalysis_Success(t *testing.T) {
	invID := "inv-1"
	suspect := "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
	now := time.Now().UTC()

	client := &mockAnalysisClient{
		analyzeFunc: func(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
			if req.WalletAddress != suspect {
				t.Fatalf("expected suspect %s, got %s", suspect, req.WalletAddress)
			}
			if req.MaxDepth != 3 {
				t.Fatalf("expected depth 3, got %d", req.MaxDepth)
			}
			return &AnalysisResponse{
				WalletAddress: suspect,
				Addresses: []AddressItem{
					{Address: suspect, Role: repository.RoleSuspect},
					{Address: "0xdest", Role: repository.RoleDestination},
				},
				Transactions: []TransactionItem{
					{TxHash: "0xtx1", TxTime: now, Amount: "1.5", TraceDepth: 1, RawReference: "ref1"},
				},
				FundFlow: []FlowEdgeItem{
					{TxHash: "0xtx1", SourceAddress: suspect, DestinationAddress: "0xdest", Amount: "1.5"},
				},
				VASPAttribution: &VASPAttributionItem{
					Name:              "Binance",
					Type:              "exchange",
					MatchedAddress:    "0xdest",
					AttributionStatus: "known",
				},
			}, nil
		},
	}

	saved := false
	repo := &mockRepo{
		saveInvestigationResultFunc: func(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
			if id != invID {
				t.Fatalf("expected ID %s, got %s", invID, id)
			}
			if len(params.Transactions) != 1 || params.Transactions[0].TxHash != "0xtx1" {
				t.Fatalf("unexpected transactions: %v", params.Transactions)
			}
			if len(params.Edges) != 1 || params.Edges[0].TxHash != "0xtx1" {
				t.Fatalf("unexpected edges: %v", params.Edges)
			}
			if params.VASPAttribution == nil || params.VASPAttribution.Name != "Binance" {
				t.Fatalf("unexpected attribution: %v", params.VASPAttribution)
			}
			saved = true
			return nil
		},
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			return &repository.InvestigationDetails{
				ID:             invID,
				Status:         repository.StatusCompleted,
				SuspectAddress: suspect,
				CreatedAt:      now,
			}, nil
		},
	}

	orch := New(client, repo)
	details, err := orch.RunAnalysis(context.Background(), invID, suspect, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !saved {
		t.Fatal("expected SaveInvestigationResult to be called")
	}
	if details.ID != invID || details.Status != repository.StatusCompleted {
		t.Fatalf("unexpected details: %v", details)
	}
}

func TestAnalysisOrchestrator_RunAnalysis_ClientFailure(t *testing.T) {
	invID := "inv-2"
	suspect := "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"

	client := &mockAnalysisClient{
		analyzeFunc: func(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
			return nil, errors.New("upstream provider timeout")
		},
	}

	failedWithReason := ""
	repo := &mockRepo{
		failInvestigationFunc: func(ctx context.Context, id, reason string) error {
			if id != invID {
				t.Fatalf("expected ID %s, got %s", invID, id)
			}
			failedWithReason = reason
			return nil
		},
	}

	orch := New(client, repo)
	_, err := orch.RunAnalysis(context.Background(), invID, suspect, 2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperror.AppError, got %T", err)
	}
	if appErr.Code != apperror.CodeAnalysisServiceError {
		t.Fatalf("expected code %s, got %s", apperror.CodeAnalysisServiceError, appErr.Code)
	}
	if appErr.InvestigationID != invID {
		t.Fatalf("expected investigation ID %s, got %s", invID, appErr.InvestigationID)
	}
	if failedWithReason == "" {
		t.Fatal("expected failInvestigationFunc to record reason")
	}
}

func TestAnalysisOrchestrator_RunAnalysis_PersistenceFailure(t *testing.T) {
	invID := "inv-3"
	suspect := "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"

	client := &mockAnalysisClient{
		analyzeFunc: func(ctx context.Context, req AnalysisRequest) (*AnalysisResponse, error) {
			return &AnalysisResponse{
				WalletAddress: suspect,
			}, nil
		},
	}

	failed := false
	repo := &mockRepo{
		saveInvestigationResultFunc: func(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
			return errors.New("disk full")
		},
		failInvestigationFunc: func(ctx context.Context, id, reason string) error {
			failed = true
			return nil
		},
	}

	orch := New(client, repo)
	_, err := orch.RunAnalysis(context.Background(), invID, suspect, 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !failed {
		t.Fatal("expected failInvestigationFunc to be called on persistence failure")
	}
}
