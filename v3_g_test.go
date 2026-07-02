package terrapin

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Verifies: REQ-G-001
func TestGEqualsBlobFramedSHA256(t *testing.T) {
	for _, in := range [][]byte{nil, []byte("hello world"), []byte("abc"), fillBytes(1000, 1)} {
		got := G(in)
		want := framedSHA256(in)
		if got != want {
			t.Errorf("G(len=%d): got %x want %x", len(in), got, want)
		}
	}
	// Anchor to git's known empty-blob SHA-256.
	ge := G(nil)
	if hex.EncodeToString(ge[:]) != "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813" {
		t.Errorf("G(empty) mismatch: %x", ge)
	}
}

// Verifies: REQ-G-002
func TestGIs32BytesAndDeterministic(t *testing.T) {
	d := fillBytes(123, 2)
	a, b := G(d), G(d)
	if a != b {
		t.Error("G not deterministic")
	}
	if len([32]byte(a)) != 32 {
		t.Error("G not 32 bytes")
	}
}

// Verifies: REQ-G-003
func TestGBindsLength(t *testing.T) {
	for _, d := range [][]byte{[]byte("x"), fillBytes(64, 3)} {
		raw := sha256.Sum256(d) // bare hash, no blob framing
		if G(d) == raw {
			t.Errorf("G must differ from raw sha256 (framing not applied) for len=%d", len(d))
		}
	}
}

// Verifies: REQ-G-004
func TestGAcrossBlockBoundary(t *testing.T) {
	for _, n := range []int{Block - 1, Block, Block + 1} {
		d := fillBytes(n, int64(n))
		if G(d) != framedSHA256(d) {
			t.Errorf("G(len=%d) != framed oracle", n)
		}
	}
}

// Verifies: REQ-G-005
func TestGAvalanche(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		d := fillBytes(50, seed)
		base := G(d)
		e := make([]byte, len(d))
		copy(e, d)
		e[seed%50] ^= 1 << uint(seed%8)
		if G(e) == base {
			t.Errorf("seed %d: single-bit flip did not change G", seed)
		}
	}
}
