package apperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/schak04/crypto-fund-tracer/internal/service"
)

// standard error codes defined by the API contract
const (
	CodeInvalidRequest         = "INVALID_REQUEST"
	CodeInvalidAddress         = "INVALID_ADDRESS"
	CodeInvalidInvestigationID = "INVALID_INVESTIGATION_ID"
	CodeInvestigationNotFound  = "INVESTIGATION_NOT_FOUND"
	CodeAnalysisServiceError   = "ANALYSIS_SERVICE_ERROR"
	CodeInternalError          = "INTERNAL_ERROR"
)

// specific field-level validation issues
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// the payload nested within the error response envelope
type ErrorDetail struct {
	Code            string       `json:"code"`
	Message         string       `json:"message"`
	Details         []FieldError `json:"details,omitempty"`
	InvestigationID string       `json:"investigation_id,omitempty"`
}

// the standard JSON error envelope
type Response struct {
	Error ErrorDetail `json:"error"`
}

// the structured application error carrying HTTP status and code
type AppError struct {
	HTTPStatus      int
	Code            string
	Message         string
	Details         []FieldError
	InvestigationID string
	Cause           error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Response() Response {
	return Response{
		Error: ErrorDetail{
			Code:            e.Code,
			Message:         e.Message,
			Details:         e.Details,
			InvestigationID: e.InvestigationID,
		},
	}
}

func NewInvalidRequest(msg string, details ...FieldError) *AppError {
	return &AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       CodeInvalidRequest,
		Message:    msg,
		Details:    details,
	}
}

func NewInvalidAddress(msg string, details ...FieldError) *AppError {
	return &AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       CodeInvalidAddress,
		Message:    msg,
		Details:    details,
	}
}

func NewInvalidInvestigationID(msg string, details ...FieldError) *AppError {
	return &AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       CodeInvalidInvestigationID,
		Message:    msg,
		Details:    details,
	}
}

func NewNotFound(msg string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusNotFound,
		Code:       CodeInvestigationNotFound,
		Message:    msg,
	}
}

func NewAnalysisServiceError(msg string, investigationID string, cause error) *AppError {
	return &AppError{
		HTTPStatus:      http.StatusBadGateway,
		Code:            CodeAnalysisServiceError,
		Message:         msg,
		InvestigationID: investigationID,
		Cause:           cause,
	}
}

func NewInternalError(msg string, cause error) *AppError {
	return &AppError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       CodeInternalError,
		Message:    msg,
		Cause:      cause,
	}
}

// converts any Go error into a structured AppError according to established sentinels
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}

	if appErr, ok := errors.AsType[*AppError](err); ok {
		return appErr
	}

	if errors.Is(err, service.ErrNotFound) {
		return NewNotFound("Investigation does not exist.")
	}

	if errors.Is(err, service.ErrInvalidSuspectAddress) {
		return NewInvalidAddress("The supplied wallet address is not a valid address format for the supported blockchain.", FieldError{
			Field:   "wallet_address",
			Message: "Field is required and cannot be empty.",
		})
	}

	if errors.Is(err, service.ErrInvalidInvestigationID) {
		return NewInvalidInvestigationID("Path parameter 'id' is not a valid UUIDv4 string.")
	}

	return NewInternalError("An unexpected internal error occurred.", err)
}

// serialises the error into the standardized JSON error envelope and writes to the client
func WriteJSON(w http.ResponseWriter, err error) error {
	appErr := FromError(err)
	if appErr == nil {
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	return json.NewEncoder(w).Encode(appErr.Response())
}
