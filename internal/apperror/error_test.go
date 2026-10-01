package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/schak04/crypto-fund-tracer/internal/service"
)

func TestAppError_ResponseEnvelope(t *testing.T) {
	err := NewInvalidAddress("The supplied wallet address is not a valid address format for the supported blockchain.", FieldError{
		Field:   "wallet_address",
		Message: "Supplied address fails checksum or length validation.",
	})

	resp := err.Response()
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		t.Fatalf("marshaling error response: %v", marshalErr)
	}

	expectedJSON := `{"error":{"code":"INVALID_ADDRESS","message":"The supplied wallet address is not a valid address format for the supported blockchain.","details":[{"field":"wallet_address","message":"Supplied address fails checksum or length validation."}]}}`
	if string(data) != expectedJSON {
		t.Fatalf("unexpected JSON: got %s, want %s", string(data), expectedJSON)
	}
}

func TestAppError_AnalysisServiceErrorWithID(t *testing.T) {
	cause := errors.New("timeout")
	err := NewAnalysisServiceError("provider timeout", "inv-123", cause)

	if err.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", err.HTTPStatus)
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected unwrap to match cause")
	}

	resp := err.Response()
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		t.Fatalf("marshaling: %v", marshalErr)
	}

	expectedJSON := `{"error":{"code":"ANALYSIS_SERVICE_ERROR","message":"provider timeout","investigation_id":"inv-123"}}`
	if string(data) != expectedJSON {
		t.Fatalf("unexpected JSON: got %s, want %s", string(data), expectedJSON)
	}
}

func TestFromError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if FromError(nil) != nil {
			t.Fatal("expected nil for nil error")
		}
	})

	t.Run("already AppError", func(t *testing.T) {
		orig := NewNotFound("not found")
		converted := FromError(orig)
		if converted != orig {
			t.Fatalf("expected identical pointer, got %v", converted)
		}
	})

	t.Run("service ErrNotFound", func(t *testing.T) {
		converted := FromError(service.ErrNotFound)
		if converted.Code != CodeInvestigationNotFound {
			t.Fatalf("expected code %s, got %s", CodeInvestigationNotFound, converted.Code)
		}
		if converted.HTTPStatus != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", converted.HTTPStatus)
		}
	})

	t.Run("service ErrInvalidSuspectAddress", func(t *testing.T) {
		converted := FromError(service.ErrInvalidSuspectAddress)
		if converted.Code != CodeInvalidAddress {
			t.Fatalf("expected code %s, got %s", CodeInvalidAddress, converted.Code)
		}
		if converted.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", converted.HTTPStatus)
		}
	})

	t.Run("service ErrInvalidInvestigationID", func(t *testing.T) {
		converted := FromError(service.ErrInvalidInvestigationID)
		if converted.Code != CodeInvalidInvestigationID {
			t.Fatalf("expected code %s, got %s", CodeInvalidInvestigationID, converted.Code)
		}
		if converted.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", converted.HTTPStatus)
		}
	})

	t.Run("unknown error maps to internal", func(t *testing.T) {
		unknown := errors.New("db connection failed")
		converted := FromError(unknown)
		if converted.Code != CodeInternalError {
			t.Fatalf("expected code %s, got %s", CodeInternalError, converted.Code)
		}
		if converted.HTTPStatus != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", converted.HTTPStatus)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	err := NewNotFound("Investigation with ID 'inv-1' does not exist.")

	if writeErr := WriteJSON(rec, err); writeErr != nil {
		t.Fatalf("WriteJSON error: %v", writeErr)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	var resp Response
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &resp); decodeErr != nil {
		t.Fatalf("decoding written response: %v", decodeErr)
	}
	if resp.Error.Code != CodeInvestigationNotFound {
		t.Fatalf("expected code %s, got %s", CodeInvestigationNotFound, resp.Error.Code)
	}
}
