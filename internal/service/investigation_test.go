package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/schak04/crypto-fund-tracer/internal/repository"
)

type mockRepository struct {
	createInvestigationFunc     func(ctx context.Context, suspectAddress string, initialStatus string) (*repository.Investigation, error)
	getInvestigationFunc        func(ctx context.Context, id string) (*repository.InvestigationDetails, error)
	saveInvestigationResultFunc func(ctx context.Context, id string, params repository.SaveAnalysisParams) error
	failInvestigationFunc       func(ctx context.Context, id string, reason string) error
	upsertAddressFunc           func(ctx context.Context, address string) (string, error)
	upsertVASPFunc              func(ctx context.Context, name, vaspType string) (string, error)
	associateVASPAddressFunc    func(ctx context.Context, vaspID, addressID, status string) error
}

func (m *mockRepository) CreateInvestigation(ctx context.Context, suspectAddress string, initialStatus string) (*repository.Investigation, error) {
	if m.createInvestigationFunc != nil {
		return m.createInvestigationFunc(ctx, suspectAddress, initialStatus)
	}
	return nil, errors.New("unexpected call to CreateInvestigation")
}

func (m *mockRepository) GetInvestigation(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
	if m.getInvestigationFunc != nil {
		return m.getInvestigationFunc(ctx, id)
	}
	return nil, errors.New("unexpected call to GetInvestigation")
}

func (m *mockRepository) SaveInvestigationResult(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
	if m.saveInvestigationResultFunc != nil {
		return m.saveInvestigationResultFunc(ctx, id, params)
	}
	return errors.New("unexpected call to SaveInvestigationResult")
}

func (m *mockRepository) FailInvestigation(ctx context.Context, id string, reason string) error {
	if m.failInvestigationFunc != nil {
		return m.failInvestigationFunc(ctx, id, reason)
	}
	return errors.New("unexpected call to FailInvestigation")
}

func (m *mockRepository) UpsertAddress(ctx context.Context, address string) (string, error) {
	if m.upsertAddressFunc != nil {
		return m.upsertAddressFunc(ctx, address)
	}
	return "", errors.New("unexpected call to UpsertAddress")
}

func (m *mockRepository) UpsertVASP(ctx context.Context, name, vaspType string) (string, error) {
	if m.upsertVASPFunc != nil {
		return m.upsertVASPFunc(ctx, name, vaspType)
	}
	return "", errors.New("unexpected call to UpsertVASP")
}

func (m *mockRepository) AssociateVASPAddress(ctx context.Context, vaspID, addressID, status string) error {
	if m.associateVASPAddressFunc != nil {
		return m.associateVASPAddressFunc(ctx, vaspID, addressID, status)
	}
	return errors.New("unexpected call to AssociateVASPAddress")
}

func TestInvestigationService_CreateInvestigation_Success(t *testing.T) {
	expectedID := "inv-123"
	expectedSuspect := "0x1234567890abcdef"
	now := time.Now().UTC()

	mock := &mockRepository{
		createInvestigationFunc: func(ctx context.Context, suspectAddress string, initialStatus string) (*repository.Investigation, error) {
			if suspectAddress != expectedSuspect {
				t.Fatalf("expected suspect address %s, got %s", expectedSuspect, suspectAddress)
			}
			if initialStatus != repository.StatusRunning {
				t.Fatalf("expected initial status %s, got %s", repository.StatusRunning, initialStatus)
			}
			return &repository.Investigation{
				ID:        expectedID,
				Status:    initialStatus,
				CreatedAt: now,
			}, nil
		},
	}

	svc := New(mock)
	inv, err := svc.CreateInvestigation(context.Background(), "  0x1234567890abcdef  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.ID != expectedID {
		t.Fatalf("expected ID %s, got %s", expectedID, inv.ID)
	}
	if inv.Status != repository.StatusRunning {
		t.Fatalf("expected status %s, got %s", repository.StatusRunning, inv.Status)
	}
}

func TestInvestigationService_CreateInvestigation_EmptyAddress(t *testing.T) {
	svc := New(&mockRepository{})
	_, err := svc.CreateInvestigation(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidSuspectAddress) {
		t.Fatalf("expected ErrInvalidSuspectAddress, got: %v", err)
	}
}

func TestInvestigationService_CreateInvestigation_RepoError(t *testing.T) {
	repoErr := errors.New("database connection lost")
	mock := &mockRepository{
		createInvestigationFunc: func(ctx context.Context, suspectAddress string, initialStatus string) (*repository.Investigation, error) {
			return nil, repoErr
		},
	}

	svc := New(mock)
	_, err := svc.CreateInvestigation(context.Background(), "0x123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got: %v", err)
	}
}

func TestInvestigationService_GetInvestigation_Success(t *testing.T) {
	expectedID := "inv-456"
	mock := &mockRepository{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			if id != expectedID {
				t.Fatalf("expected ID %s, got %s", expectedID, id)
			}
			return &repository.InvestigationDetails{
				ID:             expectedID,
				Status:         repository.StatusCompleted,
				SuspectAddress: "0xabc",
				Transactions:   []repository.TransactionItem{},
				FundFlow:       []repository.FlowEdgeItem{},
			}, nil
		},
	}

	svc := New(mock)
	details, err := svc.GetInvestigation(context.Background(), "  inv-456  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details.ID != expectedID {
		t.Fatalf("expected ID %s, got %s", expectedID, details.ID)
	}
}

func TestInvestigationService_GetInvestigation_EmptyID(t *testing.T) {
	svc := New(&mockRepository{})
	_, err := svc.GetInvestigation(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidInvestigationID) {
		t.Fatalf("expected ErrInvalidInvestigationID, got: %v", err)
	}
}

func TestInvestigationService_GetInvestigation_NotFound(t *testing.T) {
	mock := &mockRepository{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			return nil, repository.ErrNotFound
		},
	}

	svc := New(mock)
	_, err := svc.GetInvestigation(context.Background(), "inv-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestInvestigationService_GetInvestigation_RepoError(t *testing.T) {
	repoErr := errors.New("disk failure")
	mock := &mockRepository{
		getInvestigationFunc: func(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
			return nil, repoErr
		},
	}

	svc := New(mock)
	_, err := svc.GetInvestigation(context.Background(), "inv-err")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got: %v", err)
	}
}

func TestInvestigationService_SaveInvestigationResult_Success(t *testing.T) {
	called := false
	mock := &mockRepository{
		saveInvestigationResultFunc: func(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
			called = true
			if id != "inv-789" {
				t.Fatalf("expected ID inv-789, got %s", id)
			}
			return nil
		},
	}

	svc := New(mock)
	err := svc.SaveInvestigationResult(context.Background(), "  inv-789  ", repository.SaveAnalysisParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected saveInvestigationResultFunc to be called")
	}
}

func TestInvestigationService_SaveInvestigationResult_EmptyID(t *testing.T) {
	svc := New(&mockRepository{})
	err := svc.SaveInvestigationResult(context.Background(), "   ", repository.SaveAnalysisParams{})
	if !errors.Is(err, ErrInvalidInvestigationID) {
		t.Fatalf("expected ErrInvalidInvestigationID, got: %v", err)
	}
}

func TestInvestigationService_SaveInvestigationResult_NotFound(t *testing.T) {
	mock := &mockRepository{
		saveInvestigationResultFunc: func(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
			return repository.ErrNotFound
		},
	}

	svc := New(mock)
	err := svc.SaveInvestigationResult(context.Background(), "inv-missing", repository.SaveAnalysisParams{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestInvestigationService_SaveInvestigationResult_RepoError(t *testing.T) {
	repoErr := errors.New("deadlock detected")
	mock := &mockRepository{
		saveInvestigationResultFunc: func(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
			return repoErr
		},
	}

	svc := New(mock)
	err := svc.SaveInvestigationResult(context.Background(), "inv-789", repository.SaveAnalysisParams{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got: %v", err)
	}
}

func TestInvestigationService_FailInvestigation_Success(t *testing.T) {
	called := false
	mock := &mockRepository{
		failInvestigationFunc: func(ctx context.Context, id string, reason string) error {
			called = true
			if id != "inv-fail" {
				t.Fatalf("expected ID inv-fail, got %s", id)
			}
			if reason != "node timeout" {
				t.Fatalf("expected reason 'node timeout', got %s", reason)
			}
			return nil
		},
	}

	svc := New(mock)
	err := svc.FailInvestigation(context.Background(), "  inv-fail  ", "  node timeout  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected failInvestigationFunc to be called")
	}
}

func TestInvestigationService_FailInvestigation_EmptyID(t *testing.T) {
	svc := New(&mockRepository{})
	err := svc.FailInvestigation(context.Background(), "   ", "some reason")
	if !errors.Is(err, ErrInvalidInvestigationID) {
		t.Fatalf("expected ErrInvalidInvestigationID, got: %v", err)
	}
}

func TestInvestigationService_FailInvestigation_EmptyReason(t *testing.T) {
	svc := New(&mockRepository{})
	err := svc.FailInvestigation(context.Background(), "inv-1", "   ")
	if !errors.Is(err, ErrInvalidFailureReason) {
		t.Fatalf("expected ErrInvalidFailureReason, got: %v", err)
	}
}

func TestInvestigationService_FailInvestigation_NotFound(t *testing.T) {
	mock := &mockRepository{
		failInvestigationFunc: func(ctx context.Context, id string, reason string) error {
			return repository.ErrNotFound
		},
	}

	svc := New(mock)
	err := svc.FailInvestigation(context.Background(), "inv-missing", "timeout")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestInvestigationService_FailInvestigation_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	mock := &mockRepository{
		failInvestigationFunc: func(ctx context.Context, id string, reason string) error {
			return repoErr
		},
	}

	svc := New(mock)
	err := svc.FailInvestigation(context.Background(), "inv-1", "timeout")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got: %v", err)
	}
}
