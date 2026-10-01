package validator

import (
	"errors"
	"testing"

	"github.com/schak04/crypto-fund-tracer/internal/apperror"
)

func TestValidateWalletAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedCode  string
		expectedField string
		wantErr       bool
	}{
		{
			name:    "valid all lowercase address",
			address: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			wantErr: false,
		},
		{
			name:    "valid all uppercase address",
			address: "0X5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED",
			wantErr: false,
		},
		{
			name:    "valid EIP-55 checksum address",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			wantErr: false,
		},
		{
			name:    "valid EIP-55 checksum address 2",
			address: "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
			wantErr: false,
		},
		{
			name:          "empty address",
			address:       "   ",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
		{
			name:          "missing 0x prefix",
			address:       "5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
		{
			name:          "too short",
			address:       "0x1234",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
		{
			name:          "too long",
			address:       "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed1234",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
		{
			name:          "invalid hex characters",
			address:       "0x5aaeb6053f3e94c9b9a09f33669435e7ef1bezzz",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
		{
			name:          "invalid EIP-55 checksum casing",
			address:       "0x5aaeb6053F3e94C9b9A09f33669435E7Ef1BeAed",
			wantErr:       true,
			expectedCode:  apperror.CodeInvalidAddress,
			expectedField: "wallet_address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWalletAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateWalletAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) {
					t.Fatalf("expected *apperror.AppError, got %T", err)
				}
				if appErr.Code != tt.expectedCode {
					t.Fatalf("expected code %s, got %s", tt.expectedCode, appErr.Code)
				}
				if len(appErr.Details) > 0 && appErr.Details[0].Field != tt.expectedField {
					t.Fatalf("expected field %s, got %s", tt.expectedField, appErr.Details[0].Field)
				}
			}
		})
	}
}

func TestValidateInvestigationID(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		wantErr      bool
		expectedCode string
	}{
		{
			name:    "valid uuid v4 lowercase",
			id:      "c56a4180-65aa-42ec-a945-5fd21dec0538",
			wantErr: false,
		},
		{
			name:    "valid uuid v4 uppercase",
			id:      "C56A4180-65AA-42EC-A945-5FD21DEC0538",
			wantErr: false,
		},
		{
			name:         "empty id",
			id:           "   ",
			wantErr:      true,
			expectedCode: apperror.CodeInvalidInvestigationID,
		},
		{
			name:         "invalid uuid version (version 1)",
			id:           "c56a4180-65aa-12ec-a945-5fd21dec0538",
			wantErr:      true,
			expectedCode: apperror.CodeInvalidInvestigationID,
		},
		{
			name:         "not a uuid",
			id:           "not-a-uuid",
			wantErr:      true,
			expectedCode: apperror.CodeInvalidInvestigationID,
		},
		{
			name:         "wrong hyphen placement",
			id:           "c56a418065aa-42ec-a945-5fd21dec0538",
			wantErr:      true,
			expectedCode: apperror.CodeInvalidInvestigationID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInvestigationID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateInvestigationID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) {
					t.Fatalf("expected *apperror.AppError, got %T", err)
				}
				if appErr.Code != tt.expectedCode {
					t.Fatalf("expected code %s, got %s", tt.expectedCode, appErr.Code)
				}
			}
		})
	}
}

func TestValidateMaxDepth(t *testing.T) {
	tests := []struct {
		name    string
		depth   *int
		wantErr bool
	}{
		{
			name:    "nil depth is valid (uses default)",
			depth:   nil,
			wantErr: false,
		},
		{
			name:    "depth 1 is valid",
			depth:   intPtr(1),
			wantErr: false,
		},
		{
			name:    "depth 3 is valid",
			depth:   intPtr(3),
			wantErr: false,
		},
		{
			name:    "depth 5 is valid",
			depth:   intPtr(5),
			wantErr: false,
		},
		{
			name:    "depth 0 is invalid",
			depth:   intPtr(0),
			wantErr: true,
		},
		{
			name:    "depth negative is invalid",
			depth:   intPtr(-1),
			wantErr: true,
		},
		{
			name:    "depth 6 is invalid",
			depth:   intPtr(6),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMaxDepth(tt.depth)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMaxDepth() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var appErr *apperror.AppError
				if !errors.As(err, &appErr) {
					t.Fatalf("expected *apperror.AppError, got %T", err)
				}
				if appErr.Code != apperror.CodeInvalidRequest {
					t.Fatalf("expected code %s, got %s", apperror.CodeInvalidRequest, appErr.Code)
				}
			}
		})
	}
}

func TestValidateCreateInvestigationRequest(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := CreateInvestigationRequest{
			WalletAddress: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			MaxDepth:      intPtr(3),
		}
		if err := ValidateCreateInvestigationRequest(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid address", func(t *testing.T) {
		req := CreateInvestigationRequest{
			WalletAddress: "invalid-address",
		}
		if err := ValidateCreateInvestigationRequest(req); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid depth", func(t *testing.T) {
		req := CreateInvestigationRequest{
			WalletAddress: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			MaxDepth:      intPtr(10),
		}
		if err := ValidateCreateInvestigationRequest(req); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func intPtr(v int) *int {
	return &v
}
