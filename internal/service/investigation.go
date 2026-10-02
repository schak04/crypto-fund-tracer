package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/schak04/crypto-fund-tracer/internal/repository"
)

var (
	ErrNotFound               = repository.ErrNotFound
	ErrInvalidSuspectAddress  = errors.New("suspect address is required")
	ErrInvalidInvestigationID = errors.New("investigation id is required")
	ErrInvalidFailureReason   = errors.New("failure reason is required")
)

// boundary for executing blockchain analysis
type Orchestrator interface {
	RunAnalysis(ctx context.Context, investigationID, walletAddress string, maxDepth int) (*repository.InvestigationDetails, error)
}

// business contract for managing investigation lifecycles
type Service interface {
	CreateInvestigation(ctx context.Context, suspectAddress string) (*repository.Investigation, error)
	CreateAndRunInvestigation(ctx context.Context, suspectAddress string, maxDepth int) (*repository.InvestigationDetails, error)
	GetInvestigation(ctx context.Context, id string) (*repository.InvestigationDetails, error)
	SaveInvestigationResult(ctx context.Context, id string, params repository.SaveAnalysisParams) error
	FailInvestigation(ctx context.Context, id string, reason string) error
}

type InvestigationService struct {
	repo         repository.Repository
	orchestrator Orchestrator
}

var _ Service = (*InvestigationService)(nil)

func New(repo repository.Repository, orchestrator ...Orchestrator) *InvestigationService {
	var orch Orchestrator
	if len(orchestrator) > 0 {
		orch = orchestrator[0]
	}
	return &InvestigationService{
		repo:         repo,
		orchestrator: orch,
	}
}

func (s *InvestigationService) CreateInvestigation(ctx context.Context, suspectAddress string) (*repository.Investigation, error) {
	address := strings.TrimSpace(suspectAddress)
	if address == "" {
		return nil, ErrInvalidSuspectAddress
	}

	// MVP executes analysis synchronously upon submission, so the record
	// begins in running status rather than queued pending state.
	inv, err := s.repo.CreateInvestigation(ctx, address, repository.StatusRunning)
	if err != nil {
		return nil, fmt.Errorf("creating investigation: %w", err)
	}

	return inv, nil
}

func (s *InvestigationService) CreateAndRunInvestigation(ctx context.Context, suspectAddress string, maxDepth int) (*repository.InvestigationDetails, error) {
	inv, err := s.CreateInvestigation(ctx, suspectAddress)
	if err != nil {
		return nil, err
	}

	if s.orchestrator != nil {
		return s.orchestrator.RunAnalysis(ctx, inv.ID, suspectAddress, maxDepth)
	}

	return s.GetInvestigation(ctx, inv.ID)
}

func (s *InvestigationService) GetInvestigation(ctx context.Context, id string) (*repository.InvestigationDetails, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, ErrInvalidInvestigationID
	}

	details, err := s.repo.GetInvestigation(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting investigation %s: %w", trimmedID, err)
	}

	return details, nil
}

func (s *InvestigationService) SaveInvestigationResult(ctx context.Context, id string, params repository.SaveAnalysisParams) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return ErrInvalidInvestigationID
	}

	if err := s.repo.SaveInvestigationResult(ctx, trimmedID, params); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("saving investigation result %s: %w", trimmedID, err)
	}

	return nil
}

func (s *InvestigationService) FailInvestigation(ctx context.Context, id string, reason string) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return ErrInvalidInvestigationID
	}

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return ErrInvalidFailureReason
	}

	if err := s.repo.FailInvestigation(ctx, trimmedID, trimmedReason); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("failing investigation %s: %w", trimmedID, err)
	}

	return nil
}
