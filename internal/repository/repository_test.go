package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/schak04/crypto-fund-tracer/internal/database"
)

func setupTestRepo(t *testing.T) (*PostgresRepository, func()) {
	t.Helper()

	// These are integration tests by design, so without an explicit DB URL,
	// skip rather than silently falling back to a developer's local database.
	dsn := os.Getenv("CFT_DB_URL")
	if dsn == "" {
		t.Skip("CFT_DB_URL not set, skipping repository integration tests")
	}

	// Keep connection setup bounded so a broken/unreachable test database
	// cannot hang the test suite indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}

	repo := New(pool)
	cleanup := func() {
		pool.Close()
	}

	return repo, cleanup
}

func TestRepository_CreateAndGetInvestigation(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The address table enforces uniqueness, so make test data unique without
	// requiring database cleanup between runs.
	suspect := "0xsuspect_" + time.Now().Format("20060102150405.000000")
	inv, err := repo.CreateInvestigation(ctx, suspect, StatusRunning)
	if err != nil {
		t.Fatalf("CreateInvestigation failed: %v", err)
	}

	if inv.ID == "" {
		t.Fatal("expected non-empty investigation ID")
	}
	if inv.Status != StatusRunning {
		t.Fatalf("expected status %s, got %s", StatusRunning, inv.Status)
	}

	details, err := repo.GetInvestigation(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvestigation failed: %v", err)
	}

	if details.ID != inv.ID {
		t.Fatalf("expected ID %s, got %s", inv.ID, details.ID)
	}
	if details.SuspectAddress != suspect {
		t.Fatalf("expected suspect address %s, got %s", suspect, details.SuspectAddress)
	}
	if details.Status != StatusRunning {
		t.Fatalf("expected status %s, got %s", StatusRunning, details.Status)
	}
	if len(details.Transactions) != 0 {
		t.Fatalf("expected 0 transactions, got %d", len(details.Transactions))
	}
	if len(details.FundFlow) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(details.FundFlow))
	}
	if details.VASPAttribution != nil {
		t.Fatal("expected nil VASP attribution")
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := repo.GetInvestigation(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestRepository_FailInvestigation(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	suspect := "0xsuspect_fail_" + time.Now().Format("20060102150405.000000")
	inv, err := repo.CreateInvestigation(ctx, suspect, StatusRunning)
	if err != nil {
		t.Fatalf("CreateInvestigation failed: %v", err)
	}

	failureReason := "analysis service unreachable"
	if err := repo.FailInvestigation(ctx, inv.ID, failureReason); err != nil {
		t.Fatalf("FailInvestigation failed: %v", err)
	}

	details, err := repo.GetInvestigation(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvestigation failed: %v", err)
	}

	if details.Status != StatusFailed {
		t.Fatalf("expected status %s, got %s", StatusFailed, details.Status)
	}
	if details.FailureReason == nil || *details.FailureReason != failureReason {
		t.Fatalf("expected failure reason %q, got %v", failureReason, details.FailureReason)
	}
	if details.CompletedAt == nil {
		t.Fatal("expected completed_at to be non-nil")
	}
}

func TestRepository_SaveInvestigationResult(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Since integration tests share the database, unique test identifiers keep
	// repeated runs from colliding with persisted rows.
	tag := time.Now().Format("20060102150405.000000")
	suspect := "0xsuspect_" + tag
	interm := "0xinterm_" + tag
	dest := "0xdest_" + tag
	txHash1 := "0xtx1_" + tag
	txHash2 := "0xtx2_" + tag

	inv, err := repo.CreateInvestigation(ctx, suspect, StatusRunning)
	if err != nil {
		t.Fatalf("CreateInvestigation failed: %v", err)
	}

	// Build one small two-hop flow so this test exercises the complete
	// persistence path: transactions -> edges -> address roles -> attribution
	params := SaveAnalysisParams{
		Transactions: []TransactionItem{
			{
				TxHash:       txHash1,
				TxTime:       time.Now().Add(-1 * time.Hour).Truncate(time.Second),
				Amount:       "1.500000000000000000",
				TraceDepth:   1,
				RawReference: "https://explorer/tx/" + txHash1,
			},
			{
				TxHash:       txHash2,
				TxTime:       time.Now().Truncate(time.Second),
				Amount:       "1.450000000000000000",
				TraceDepth:   2,
				RawReference: "https://explorer/tx/" + txHash2,
			},
		},
		Edges: []FlowEdgeItem{
			{
				TxHash:             txHash1,
				SourceAddress:      suspect,
				DestinationAddress: interm,
				Amount:             "1.500000000000000000",
			},
			{
				TxHash:             txHash2,
				SourceAddress:      interm,
				DestinationAddress: dest,
				Amount:             "1.450000000000000000",
			},
		},
		AddressRoles: map[string]string{
			interm: RoleIntermediary,
			dest:   RoleDestination,
		},
		VASPAttribution: &VASPAttributionItem{
			Name:              "Binance",
			Type:              "exchange",
			MatchedAddress:    dest,
			AttributionStatus: AttributionKnown,
		},
	}

	if err := repo.SaveInvestigationResult(ctx, inv.ID, params); err != nil {
		t.Fatalf("SaveInvestigationResult failed: %v", err)
	}

	details, err := repo.GetInvestigation(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvestigation failed: %v", err)
	}

	if details.Status != StatusCompleted {
		t.Fatalf("expected status %s, got %s", StatusCompleted, details.Status)
	}
	if details.CompletedAt == nil {
		t.Fatal("expected non-nil completed_at")
	}
	if len(details.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(details.Transactions))
	}
	if details.Transactions[0].TxHash != txHash1 || details.Transactions[0].TraceDepth != 1 {
		t.Fatalf("unexpected first transaction: %+v", details.Transactions[0])
	}
	if details.Transactions[1].TxHash != txHash2 || details.Transactions[1].TraceDepth != 2 {
		t.Fatalf("unexpected second transaction: %+v", details.Transactions[1])
	}

	if len(details.FundFlow) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(details.FundFlow))
	}
	if details.FundFlow[0].SourceAddress != suspect || details.FundFlow[0].DestinationAddress != interm {
		t.Fatalf("unexpected first edge: %+v", details.FundFlow[0])
	}

	if details.VASPAttribution == nil {
		t.Fatal("expected non-nil VASP attribution")
	}
	if details.VASPAttribution.Name != "Binance" {
		t.Fatalf("expected VASP name Binance, got %s", details.VASPAttribution.Name)
	}
	if details.VASPAttribution.MatchedAddress != dest {
		t.Fatalf("expected matched address %s, got %s", dest, details.VASPAttribution.MatchedAddress)
	}
	if details.VASPAttribution.AttributionStatus != AttributionKnown {
		t.Fatalf("expected attribution status %s, got %s", AttributionKnown, details.VASPAttribution.AttributionStatus)
	}
}

func TestRepository_IdempotentAddressUpsert(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	addr := "0xidempotent_" + time.Now().Format("20060102150405.000000")
	id1, err := repo.UpsertAddress(ctx, addr)
	if err != nil {
		t.Fatalf("first UpsertAddress failed: %v", err)
	}

	id2, err := repo.UpsertAddress(ctx, addr)
	if err != nil {
		t.Fatalf("second UpsertAddress failed: %v", err)
	}

	if id1 != id2 {
		t.Fatalf("expected identical address IDs on upsert, got %s and %s", id1, id2)
	}
}
