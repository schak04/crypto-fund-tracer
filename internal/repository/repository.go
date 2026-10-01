package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("investigation not found")

// Repository is the persistence boundary for the application.
// The service layer should not need to know whether these operations
// hit PostgreSQL, another database, or a test implementation.
type Repository interface {
	CreateInvestigation(ctx context.Context, suspectAddress string, initialStatus string) (*Investigation, error)
	GetInvestigation(ctx context.Context, id string) (*InvestigationDetails, error)
	SaveInvestigationResult(ctx context.Context, id string, params SaveAnalysisParams) error
	FailInvestigation(ctx context.Context, id string, reason string) error
	UpsertAddress(ctx context.Context, address string) (string, error)
	UpsertVASP(ctx context.Context, name, vaspType string) (string, error)
	AssociateVASPAddress(ctx context.Context, vaspID, addressID, status string) error
}

type PostgresRepository struct {
	// Keep the pool here rather than opening connections per operation.
	// pgxpool handles connection acquisition and reuse.
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PostgresRepository {
	// The pool is created during application startup and injected here.
	// The repository does not own its lifecycle.
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateInvestigation(ctx context.Context, suspectAddress string, initialStatus string) (*Investigation, error) {
	if initialStatus == "" {
		// An omitted status means "start processing now"; pending is only
		// the database default for inserts that don't specify a status.
		initialStatus = StatusRunning
	}

	// Creating the investigation and linking its suspect address must be
	// atomic: don't leave an investigation row without its suspect address.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	// Rollback is the failure-path cleanup. After a successful Commit,
	// the transaction is already closed, so this becomes a no-op.
	defer tx.Rollback(ctx)

	var inv Investigation
	queryInv := `
		INSERT INTO investigation (status)
		VALUES ($1)
		RETURNING id, status, created_at, completed_at, failure_reason
	`
	if err := tx.QueryRow(ctx, queryInv, initialStatus).Scan(
		&inv.ID,
		&inv.Status,
		&inv.CreatedAt,
		&inv.CompletedAt,
		&inv.FailureReason,
	); err != nil {
		return nil, fmt.Errorf("inserting investigation: %w", err)
	}

	addressID, err := upsertAddressTx(ctx, tx, suspectAddress)
	if err != nil {
		return nil, fmt.Errorf("upserting suspect address: %w", err)
	}

	queryLink := `
		INSERT INTO investigation_address (investigation_id, address_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (investigation_id, address_id) DO UPDATE SET role = EXCLUDED.role
	`
	if _, err := tx.Exec(ctx, queryLink, inv.ID, addressID, RoleSuspect); err != nil {
		return nil, fmt.Errorf("linking suspect address: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &inv, nil
}

func (r *PostgresRepository) GetInvestigation(ctx context.Context, id string) (*InvestigationDetails, error) {
	queryInv := `
		SELECT i.id, i.status, i.created_at, i.completed_at, i.failure_reason, COALESCE(a.address, '')
		FROM investigation i
		LEFT JOIN investigation_address ia ON i.id = ia.investigation_id AND ia.role = $2
		LEFT JOIN address a ON ia.address_id = a.id
		WHERE i.id = $1
	`

	var details InvestigationDetails
	details.Transactions = make([]TransactionItem, 0)
	details.FundFlow = make([]FlowEdgeItem, 0)

	err := r.pool.QueryRow(ctx, queryInv, id, RoleSuspect).Scan(
		&details.ID,
		&details.Status,
		&details.CreatedAt,
		&details.CompletedAt,
		&details.FailureReason,
		&details.SuspectAddress,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("querying investigation: %w", err)
	}

	if details.Status != StatusCompleted {
		// Analysis data is persisted only for completed investigations..
		// for running/failed investigations the status metadata is enough.
		return &details, nil
	}

	// The API model combines transaction data with investigation-specific
	// trace depth, so this is assembled here rather than mirroring the DB tables.
	queryTx := `
		SELECT t.tx_hash, t.tx_time, t.amount::text, it.trace_depth, t.raw_reference
		FROM investigation_transaction it
		JOIN transaction t ON it.tx_hash = t.tx_hash
		WHERE it.investigation_id = $1
		ORDER BY it.trace_depth ASC, t.tx_time ASC
	`
	rowsTx, err := r.pool.Query(ctx, queryTx, id)
	if err != nil {
		return nil, fmt.Errorf("querying transactions: %w", err)
	}
	defer rowsTx.Close()

	for rowsTx.Next() {
		var item TransactionItem
		if err := rowsTx.Scan(&item.TxHash, &item.TxTime, &item.Amount, &item.TraceDepth, &item.RawReference); err != nil {
			return nil, fmt.Errorf("scanning transaction row: %w", err)
		}
		details.Transactions = append(details.Transactions, item)
	}
	if err := rowsTx.Err(); err != nil {
		return nil, fmt.Errorf("iterating transaction rows: %w", err)
	}

	queryEdges := `
		SELECT te.tx_hash, sa.address, da.address, te.amount::text
		FROM transaction_edge te
		JOIN address sa ON te.source_address_id = sa.id
		JOIN address da ON te.destination_address_id = da.id
		JOIN investigation_transaction it ON te.tx_hash = it.tx_hash AND it.investigation_id = $1
		ORDER BY te.tx_hash
	`
	rowsEdges, err := r.pool.Query(ctx, queryEdges, id)
	if err != nil {
		return nil, fmt.Errorf("querying transaction edges: %w", err)
	}
	defer rowsEdges.Close()

	for rowsEdges.Next() {
		var edge FlowEdgeItem
		if err := rowsEdges.Scan(&edge.TxHash, &edge.SourceAddress, &edge.DestinationAddress, &edge.Amount); err != nil {
			return nil, fmt.Errorf("scanning edge row: %w", err)
		}
		details.FundFlow = append(details.FundFlow, edge)
	}
	if err := rowsEdges.Err(); err != nil {
		return nil, fmt.Errorf("iterating edge rows: %w", err)
	}

	// Attribution is optional: a completed investigation may have no
	// destination address that matches the known VASP dataset.
	queryAttribution := `
		SELECT v.id, v.name, v.type, a.address, va.attribution_status
		FROM investigation_address ia
		JOIN address a ON ia.address_id = a.id
		JOIN vasp_address va ON a.id = va.address_id
		JOIN vasp v ON va.vasp_id = v.id
		WHERE ia.investigation_id = $1 AND ia.role = $2
		LIMIT 1
	`
	var attr VASPAttributionItem
	err = r.pool.QueryRow(ctx, queryAttribution, id, RoleDestination).Scan(
		&attr.VASPID,
		&attr.Name,
		&attr.Type,
		&attr.MatchedAddress,
		&attr.AttributionStatus,
	)
	if err == nil {
		details.VASPAttribution = &attr
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("querying vasp attribution: %w", err)
	}

	return &details, nil
}

func (r *PostgresRepository) SaveInvestigationResult(ctx context.Context, id string, params SaveAnalysisParams) error {
	// Persist the entire analysis result as one unit. A partial graph or
	// half-written attribution is worse than failing the investigation.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Analysis results mention the same addresses repeatedly. Resolve each
	// address once per transaction instead of hitting PostgreSQL for every edge.
	addressIDCache := make(map[string]string)
	resolveAddressID := func(addr string) (string, error) {
		if cachedID, ok := addressIDCache[addr]; ok {
			return cachedID, nil
		}
		addrID, err := upsertAddressTx(ctx, tx, addr)
		if err != nil {
			return "", err
		}
		addressIDCache[addr] = addrID
		return addrID, nil
	}

	for addr, role := range params.AddressRoles {
		addrID, err := resolveAddressID(addr)
		if err != nil {
			return fmt.Errorf("resolving address %s: %w", addr, err)
		}
		queryLink := `
			INSERT INTO investigation_address (investigation_id, address_id, role)
			VALUES ($1, $2, $3)
			ON CONFLICT (investigation_id, address_id) DO UPDATE SET role = EXCLUDED.role
		`
		if _, err := tx.Exec(ctx, queryLink, id, addrID, role); err != nil {
			return fmt.Errorf("linking address %s with role %s: %w", addr, role, err)
		}
	}

	// transaction is global by tx_hash; investigation_transaction records
	// which transactions belong to this investigation and at what depth.
	for _, item := range params.Transactions {
		queryTx := `
			INSERT INTO transaction (tx_hash, tx_time, amount, raw_reference)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (tx_hash) DO NOTHING
		`
		if _, err := tx.Exec(ctx, queryTx, item.TxHash, item.TxTime, item.Amount, item.RawReference); err != nil {
			return fmt.Errorf("inserting transaction %s: %w", item.TxHash, err)
		}

		queryInvTx := `
			INSERT INTO investigation_transaction (investigation_id, tx_hash, trace_depth)
			VALUES ($1, $2, $3)
			ON CONFLICT (investigation_id, tx_hash) DO UPDATE SET trace_depth = EXCLUDED.trace_depth
		`
		if _, err := tx.Exec(ctx, queryInvTx, id, item.TxHash, item.TraceDepth); err != nil {
			return fmt.Errorf("linking transaction %s to investigation: %w", item.TxHash, err)
		}
	}

	// Edges reference addresses by database ID, while analysis output carries
	// the human-readable addresses, so resolve both endpoints first.
	for _, edge := range params.Edges {
		srcID, err := resolveAddressID(edge.SourceAddress)
		if err != nil {
			return fmt.Errorf("resolving source address %s: %w", edge.SourceAddress, err)
		}
		dstID, err := resolveAddressID(edge.DestinationAddress)
		if err != nil {
			return fmt.Errorf("resolving destination address %s: %w", edge.DestinationAddress, err)
		}

		queryEdge := `
			INSERT INTO transaction_edge (tx_hash, source_address_id, destination_address_id, amount)
			VALUES ($1, $2, $3, $4)
		`
		if _, err := tx.Exec(ctx, queryEdge, edge.TxHash, srcID, dstID, edge.Amount); err != nil {
			return fmt.Errorf("inserting transaction edge for %s: %w", edge.TxHash, err)
		}
	}

	if params.VASPAttribution != nil {
		vaspID, err := upsertVASPTx(ctx, tx, params.VASPAttribution.Name, params.VASPAttribution.Type)
		if err != nil {
			return fmt.Errorf("upserting vasp %s: %w", params.VASPAttribution.Name, err)
		}

		matchedAddrID, err := resolveAddressID(params.VASPAttribution.MatchedAddress)
		if err != nil {
			return fmt.Errorf("resolving matched address %s: %w", params.VASPAttribution.MatchedAddress, err)
		}

		queryVaspAddr := `
			INSERT INTO vasp_address (vasp_id, address_id, attribution_status)
			VALUES ($1, $2, $3)
			ON CONFLICT (vasp_id, address_id) DO UPDATE SET attribution_status = EXCLUDED.attribution_status
		`
		if _, err := tx.Exec(ctx, queryVaspAddr, vaspID, matchedAddrID, params.VASPAttribution.AttributionStatus); err != nil {
			return fmt.Errorf("linking vasp to address: %w", err)
		}
	}

	// Only mark the investigation completed after every analysis artifact
	// has been persisted successfully.
	queryUpdate := `
		UPDATE investigation
		SET status = $2, completed_at = now()
		WHERE id = $1
	`
	res, err := tx.Exec(ctx, queryUpdate, id, StatusCompleted)
	if err != nil {
		return fmt.Errorf("updating investigation status to completed: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *PostgresRepository) FailInvestigation(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE investigation
		SET status = $2, completed_at = now(), failure_reason = $3
		WHERE id = $1
	`
	res, err := r.pool.Exec(ctx, query, id, StatusFailed, reason)
	if err != nil {
		return fmt.Errorf("failing investigation: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) UpsertAddress(ctx context.Context, address string) (string, error) {
	return upsertAddressTx(ctx, r.pool, address)
}

func (r *PostgresRepository) UpsertVASP(ctx context.Context, name, vaspType string) (string, error) {
	return upsertVASPTx(ctx, r.pool, name, vaspType)
}

func (r *PostgresRepository) AssociateVASPAddress(ctx context.Context, vaspID, addressID, status string) error {
	query := `
		INSERT INTO vasp_address (vasp_id, address_id, attribution_status)
		VALUES ($1, $2, $3)
		ON CONFLICT (vasp_id, address_id) DO UPDATE SET attribution_status = EXCLUDED.attribution_status
	`
	if _, err := r.pool.Exec(ctx, query, vaspID, addressID, status); err != nil {
		return fmt.Errorf("associating vasp and address: %w", err)
	}
	return nil
}

// Both pgxpool.Pool and pgx.Tx provide QueryRow. Keeping the helper against
// this small interface lets the same upsert logic work inside and outside
// a transaction.
type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// INSERT and conflict both return the canonical address ID.
// The caller doesn't need to care whether the row already existed.
func upsertAddressTx(ctx context.Context, q queryable, address string) (string, error) {
	query := `
		INSERT INTO address (address)
		VALUES ($1)
		ON CONFLICT (address) DO UPDATE SET address = EXCLUDED.address
		RETURNING id
	`
	var id string
	if err := q.QueryRow(ctx, query, address).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// VASP names are unique, so repeated analysis runs reuse the existing row
// rather than creating duplicate VASP records.
func upsertVASPTx(ctx context.Context, q queryable, name, vaspType string) (string, error) {
	query := `
		INSERT INTO vasp (name, type)
		VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET type = EXCLUDED.type
		RETURNING id
	`
	var id string
	if err := q.QueryRow(ctx, query, name, vaspType).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}
