package validator

import (
	"regexp"
	"strings"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
)

// UUIDv4 format according to RFC 4122 section 4.4 (version 4 and variant 1)
var uuidV4Regex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// hex character set for address validation
var hexRegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

const (
	MinMaxDepth = 1
	MaxMaxDepth = 5
)

// the payload for POST /api/v1/investigations
type CreateInvestigationRequest struct {
	WalletAddress string `json:"wallet_address"`
	MaxDepth      *int   `json:"max_depth,omitempty"`
}

// checks mandatory presence, length, hex characters, and EIP-55 checksum
func ValidateWalletAddress(address string) error {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return apperror.NewInvalidAddress(
			"The supplied wallet address is not a valid address format for the supported blockchain.",
			apperror.FieldError{
				Field:   "wallet_address",
				Message: "Field is required and cannot be empty.",
			},
		)
	}

	if len(trimmed) != 42 || (!strings.HasPrefix(trimmed, "0x") && !strings.HasPrefix(trimmed, "0X")) {
		return apperror.NewInvalidAddress(
			"The supplied wallet address is not a valid address format for the supported blockchain.",
			apperror.FieldError{
				Field:   "wallet_address",
				Message: "Supplied address fails checksum or length validation.",
			},
		)
	}

	raw := trimmed[2:]
	if !hexRegex.MatchString(raw) {
		return apperror.NewInvalidAddress(
			"The supplied wallet address is not a valid address format for the supported blockchain.",
			apperror.FieldError{
				Field:   "wallet_address",
				Message: "Supplied address fails checksum or length validation.",
			},
		)
	}

	// An all-lowercase or all-uppercase address is valid without checksum.
	// Only mixed-case addresses are required to satisfy the EIP-55 hash test.
	isAllLower := raw == strings.ToLower(raw)
	isAllUpper := raw == strings.ToUpper(raw)
	if !isAllLower && !isAllUpper {
		if !ValidateEIP55Checksum(trimmed) {
			return apperror.NewInvalidAddress(
				"The supplied wallet address is not a valid address format for the supported blockchain.",
				apperror.FieldError{
					Field:   "wallet_address",
					Message: "Supplied address fails checksum or length validation.",
				},
			)
		}
	}

	return nil
}

// ensures path parameters conform to standard UUIDv4
func ValidateInvestigationID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return apperror.NewInvalidInvestigationID(
			"Path parameter 'id' is not a valid UUIDv4 string.",
			apperror.FieldError{
				Field:   "id",
				Message: "Field is required and cannot be empty.",
			},
		)
	}

	if !uuidV4Regex.MatchString(trimmed) {
		return apperror.NewInvalidInvestigationID("Path parameter 'id' is not a valid UUIDv4 string.")
	}

	return nil
}

// enforces the MVP bounds (1 to 5) when max_depth is provided
func ValidateMaxDepth(depth *int) error {
	if depth == nil {
		return nil
	}

	if *depth < MinMaxDepth || *depth > MaxMaxDepth {
		return apperror.NewInvalidRequest(
			"The max_depth parameter must be a positive integer between 1 and 5.",
			apperror.FieldError{
				Field:   "max_depth",
				Message: "Value must be between 1 and 5.",
			},
		)
	}

	return nil
}

// executes all request-level validations on input payloads
func ValidateCreateInvestigationRequest(req CreateInvestigationRequest) error {
	if err := ValidateWalletAddress(req.WalletAddress); err != nil {
		return err
	}

	if err := ValidateMaxDepth(req.MaxDepth); err != nil {
		return err
	}

	return nil
}
