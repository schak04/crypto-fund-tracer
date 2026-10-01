package validator

import (
	"encoding/hex"
	"testing"
)

func TestKeccak256(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		},
		{
			input:    "The quick brown fox jumps over the lazy dog",
			expected: "4d741b6f1eb29cb2a9b9911c82f56fa8d73b04959d3d9d222895df6c0b28aa15",
		},
	}

	for _, tt := range tests {
		hash := Keccak256([]byte(tt.input))
		actual := hex.EncodeToString(hash)
		if actual != tt.expected {
			t.Errorf("Keccak256(%q) = %s, expected %s", tt.input, actual, tt.expected)
		}
	}
}

func TestValidateEIP55Checksum(t *testing.T) {
	validAddresses := []string{
		"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		"0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		"0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
	}

	for _, addr := range validAddresses {
		if !ValidateEIP55Checksum(addr) {
			t.Errorf("expected %s to have valid EIP-55 checksum", addr)
		}
	}

	invalidAddresses := []string{
		"0x5aaeb6053F3e94C9b9A09f33669435E7Ef1BeAed", // mutated case
		"0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d358", // invalid character
		"0x123", // wrong length
	}

	for _, addr := range invalidAddresses {
		if ValidateEIP55Checksum(addr) {
			t.Errorf("expected %s to fail EIP-55 checksum", addr)
		}
	}
}
