package validator

import (
	"encoding/binary"
	"encoding/hex"
	"math/bits"
	"strings"
)

// round constants for Keccak-f[1600]
var roundConstants = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808a, 0x8000000080008000,
	0x000000000000808b, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009,
	0x000000000000008a, 0x0000000000000088, 0x0000000080008009, 0x000000008000000a,
	0x000000008000808b, 0x800000000000008b, 0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080, 0x000000000000800a, 0x800000008000000a,
	0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

// rotation offsets for Keccak-f[1600]
var rotConstants = [5][5]int{
	{0, 36, 3, 41, 18},
	{1, 44, 10, 45, 2},
	{62, 6, 43, 15, 61},
	{28, 55, 25, 21, 56},
	{27, 20, 39, 8, 14},
}

// executes the 24-round permutation on the 1600-bit state
func keccakF1600(state *[25]uint64) {
	for round := 0; round < 24; round++ {
		// theta step
		var c [5]uint64
		for x := 0; x < 5; x++ {
			c[x] = state[x] ^ state[x+5] ^ state[x+10] ^ state[x+15] ^ state[x+20]
		}
		var d [5]uint64
		for x := 0; x < 5; x++ {
			d[x] = c[(x+4)%5] ^ bits.RotateLeft64(c[(x+1)%5], 1)
		}
		for x := 0; x < 5; x++ {
			for y := 0; y < 25; y += 5 {
				state[x+y] ^= d[x]
			}
		}

		// rho and pi steps
		var b [25]uint64
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				xPrime := y
				yPrime := (2*x + 3*y) % 5
				b[xPrime+5*yPrime] = bits.RotateLeft64(state[x+5*y], rotConstants[x][y])
			}
		}

		// chi step
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				state[x+5*y] = b[x+5*y] ^ ((^b[((x+1)%5)+5*y]) & b[((x+2)%5)+5*y])
			}
		}

		// iota step
		state[0] ^= roundConstants[round]
	}
}

// computes the standard Ethereum Keccak-256 hash
// NOTE: Rate is 1088 bits (136 bytes), domain separation delimiter is 0x01.
func Keccak256(data []byte) []byte {
	const rateBytes = 136
	var state [25]uint64

	// absorb full rate-sized blocks
	offset := 0
	for len(data)-offset >= rateBytes {
		for i := 0; i < 17; i++ {
			word := binary.LittleEndian.Uint64(data[offset+i*8 : offset+(i+1)*8])
			state[i] ^= word
		}
		keccakF1600(&state)
		offset += rateBytes
	}

	// pad final partial block with 0x01 ... 0x80
	remainder := data[offset:]
	padded := make([]byte, len(remainder), len(remainder)+rateBytes)
	copy(padded, remainder)
	padded = append(padded, 0x01)
	for len(padded)%rateBytes != (rateBytes - 1) {
		padded = append(padded, 0x00)
	}
	padded = append(padded, 0x80)

	for blockOffset := 0; blockOffset < len(padded); blockOffset += rateBytes {
		for i := 0; i < 17; i++ {
			word := binary.LittleEndian.Uint64(padded[blockOffset+i*8 : blockOffset+(i+1)*8])
			state[i] ^= word
		}
		keccakF1600(&state)
	}

	// squeeze out 32 bytes (256 bits)
	out := make([]byte, 32)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint64(out[i*8:(i+1)*8], state[i])
	}
	return out
}

// verifies Ethereum EIP-55 mixed-case address checksums
func ValidateEIP55Checksum(address string) bool {
	clean := strings.TrimPrefix(address, "0x")
	clean = strings.TrimPrefix(clean, "0X")
	if len(clean) != 40 {
		return false
	}

	lower := strings.ToLower(clean)
	hash := Keccak256([]byte(lower))
	hashHex := hex.EncodeToString(hash)

	for i := 0; i < 40; i++ {
		c := clean[i]
		nibble := hashHex[i]
		if nibble >= '8' {
			if c >= 'a' && c <= 'f' {
				return false
			}
		} else {
			if c >= 'A' && c <= 'F' {
				return false
			}
		}
	}
	return true
}
