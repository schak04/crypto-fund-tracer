package analysis

import "time"

// standard endpoint path for the blockchain analysis service
const EndpointAnalysis = "/analysis"

// address role constants for addresses discovered during analysis
const (
	RoleSuspect      = "suspect"
	RoleIntermediary = "intermediary"
	RoleDestination  = "destination"
	RoleOther        = "other"
)

// attribution confidence level constants
const (
	AttributionKnown      = "known"
	AttributionProbable   = "probable"
	AttributionUnverified = "unverified"
)

// standard error codes returned by the analysis component
const (
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeAnalysisFailed = "ANALYSIS_FAILED"
	CodeRPCTimeout     = "RPC_TIMEOUT"
	CodeRateLimited    = "RATE_LIMITED"
)

// input payload sent to POST /analysis
type Request struct {
	WalletAddress string `json:"wallet_address"`
	MaxDepth      int    `json:"max_depth"`
}

// an address discovered in the fund flow and its assigned role
type AddressItem struct {
	Address string `json:"address"`
	Role    string `json:"role"`
}

// a confirmed on-chain transaction discovered during tracing
type TransactionItem struct {
	TxHash       string    `json:"tx_hash"`
	TxTime       time.Time `json:"tx_time"`
	Amount       string    `json:"amount"`
	TraceDepth   int       `json:"trace_depth"`
	RawReference string    `json:"raw_reference"`
}

// a directed fund movement from source to destination
type FlowEdgeItem struct {
	TxHash             string `json:"tx_hash"`
	SourceAddress      string `json:"source_address"`
	DestinationAddress string `json:"destination_address"`
	Amount             string `json:"amount"`
}

// exchange/VASP identification details
type VASPAttributionItem struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	MatchedAddress    string `json:"matched_address"`
	AttributionStatus string `json:"attribution_status"`
}

// output payload returned by POST /analysis on HTTP 200 OK
type Response struct {
	WalletAddress   string               `json:"wallet_address"`
	Addresses       []AddressItem        `json:"addresses"`
	Transactions    []TransactionItem    `json:"transactions"`
	FundFlow        []FlowEdgeItem       `json:"fund_flow"`
	VASPAttribution *VASPAttributionItem `json:"vasp_attribution"`
}

// the error code and description from the analysis component
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// error envelope returned on failure (HTTP 400, 500, 502, 504)
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
