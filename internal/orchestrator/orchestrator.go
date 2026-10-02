package orchestrator

import (
	"context"
	"fmt"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
	"github.com/schak04/crypto-fund-tracer/internal/repository"
)

// coordinates external blockchain analysis with persistence
type Orchestrator interface {
	RunAnalysis(ctx context.Context, investigationID, walletAddress string, maxDepth int) (*repository.InvestigationDetails, error)
}

type AnalysisOrchestrator struct {
	client AnalysisClient
	repo   repository.Repository
}

var _ Orchestrator = (*AnalysisOrchestrator)(nil)

func New(client AnalysisClient, repo repository.Repository) *AnalysisOrchestrator {
	return &AnalysisOrchestrator{
		client: client,
		repo:   repo,
	}
}

func (o *AnalysisOrchestrator) RunAnalysis(ctx context.Context, investigationID, walletAddress string, maxDepth int) (*repository.InvestigationDetails, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	resp, err := o.client.Analyze(ctx, AnalysisRequest{
		WalletAddress: walletAddress,
		MaxDepth:      maxDepth,
	})
	if err != nil {
		reason := fmt.Sprintf("Blockchain analysis component failed to complete fund tracing: %v", err)
		if failErr := o.repo.FailInvestigation(ctx, investigationID, reason); failErr != nil {
			return nil, fmt.Errorf("failing investigation after analysis failure: %w (original: %v)", failErr, err)
		}
		return nil, apperror.NewAnalysisServiceError(reason, investigationID, err)
	}

	params := repository.SaveAnalysisParams{
		AddressRoles: make(map[string]string, len(resp.Addresses)),
		Transactions: make([]repository.TransactionItem, 0, len(resp.Transactions)),
		Edges:        make([]repository.FlowEdgeItem, 0, len(resp.FundFlow)),
	}

	for _, addr := range resp.Addresses {
		params.AddressRoles[addr.Address] = addr.Role
	}

	for _, tx := range resp.Transactions {
		params.Transactions = append(params.Transactions, repository.TransactionItem{
			TxHash:       tx.TxHash,
			TxTime:       tx.TxTime,
			Amount:       tx.Amount,
			TraceDepth:   tx.TraceDepth,
			RawReference: tx.RawReference,
		})
	}

	for _, edge := range resp.FundFlow {
		params.Edges = append(params.Edges, repository.FlowEdgeItem{
			TxHash:             edge.TxHash,
			SourceAddress:      edge.SourceAddress,
			DestinationAddress: edge.DestinationAddress,
			Amount:             edge.Amount,
		})
	}

	if resp.VASPAttribution != nil {
		params.VASPAttribution = &repository.VASPAttributionItem{
			Name:              resp.VASPAttribution.Name,
			Type:              resp.VASPAttribution.Type,
			MatchedAddress:    resp.VASPAttribution.MatchedAddress,
			AttributionStatus: resp.VASPAttribution.AttributionStatus,
		}
	}

	if err := o.repo.SaveInvestigationResult(ctx, investigationID, params); err != nil {
		reason := fmt.Sprintf("Failed to persist analysis results: %v", err)
		_ = o.repo.FailInvestigation(ctx, investigationID, reason)
		return nil, fmt.Errorf("saving investigation result: %w", err)
	}

	details, err := o.repo.GetInvestigation(ctx, investigationID)
	if err != nil {
		return nil, fmt.Errorf("retrieving completed investigation: %w", err)
	}

	return details, nil
}
