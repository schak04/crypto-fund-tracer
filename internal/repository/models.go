package repository

import "time"

// investigation status constants
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// investigation address role constants
const (
	RoleSuspect      = "suspect"
	RoleIntermediary = "intermediary"
	RoleDestination  = "destination"
	RoleOther        = "other"
)

// attribution status constants
const (
	AttributionKnown      = "known"
	AttributionProbable   = "probable"
	AttributionUnverified = "unverified"
)

// represents the database record for an investigation request
type Investigation struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	FailureReason *string    `json:"failure_reason,omitempty"`
}

// represents a unique blockchain address observed across investigations
type Address struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

// links an address to an investigation with a specific role
type InvestigationAddress struct {
	InvestigationID string `json:"investigation_id"`
	AddressID       string `json:"address_id"`
	Role            string `json:"role"`
}

// represents a confirmed blockchain transaction record
type Transaction struct {
	TxHash       string    `json:"tx_hash"`
	TxTime       time.Time `json:"tx_time"`
	Amount       string    `json:"amount"`
	RawReference string    `json:"raw_reference"`
}

// links a transaction to an investigation with trace depth
type InvestigationTransaction struct {
	InvestigationID string `json:"investigation_id"`
	TxHash          string `json:"tx_hash"`
	TraceDepth      int    `json:"trace_depth"`
}

// represents a directional movement of funds between addresses
type TransactionEdge struct {
	ID                   string `json:"id"`
	TxHash               string `json:"tx_hash"`
	SourceAddressID      string `json:"source_address_id"`
	DestinationAddressID string `json:"destination_address_id"`
	Amount               string `json:"amount"`
}

// represents a Virtual Asset Service Provider or exchange entity
type VASP struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// maps a known blockchain address to a VASP entity
type VASPAddress struct {
	VASPID            string `json:"vasp_id"`
	AddressID         string `json:"address_id"`
	AttributionStatus string `json:"attribution_status"`
}

// represents a transaction item returned in investigation queries
type TransactionItem struct {
	TxHash       string    `json:"tx_hash"`
	TxTime       time.Time `json:"tx_time"`
	Amount       string    `json:"amount"`
	TraceDepth   int       `json:"trace_depth"`
	RawReference string    `json:"raw_reference"`
}

// represents a directed fund-flow hop returned in investigation queries
type FlowEdgeItem struct {
	TxHash             string `json:"tx_hash"`
	SourceAddress      string `json:"source_address"`
	DestinationAddress string `json:"destination_address"`
	Amount             string `json:"amount"`
}

// represents matched VASP entity details
type VASPAttributionItem struct {
	VASPID            string `json:"vasp_id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	MatchedAddress    string `json:"matched_address"`
	AttributionStatus string `json:"attribution_status"`
}

// represents the full aggregate view of an investigation
type InvestigationDetails struct {
	ID              string               `json:"id"`
	Status          string               `json:"status"`
	SuspectAddress  string               `json:"suspect_address"`
	CreatedAt       time.Time            `json:"created_at"`
	CompletedAt     *time.Time           `json:"completed_at"`
	FailureReason   *string              `json:"failure_reason"`
	Transactions    []TransactionItem    `json:"transactions"`
	FundFlow        []FlowEdgeItem       `json:"fund_flow"`
	VASPAttribution *VASPAttributionItem `json:"vasp_attribution"`
}

// contains analysis results to persist in an atomic transaction
type SaveAnalysisParams struct {
	Transactions    []TransactionItem
	Edges           []FlowEdgeItem
	AddressRoles    map[string]string
	VASPAttribution *VASPAttributionItem
}
