package terrapin

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"
	"testing/iotest"
)

// Verifies: REQ-PR-001
func TestPropStreamEqualsInMemory(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 40; i++ {
		n := r.Intn(3 * Block)
		d := fillBytes(n, int64(i))
		got, err := IdentifierFromReader(bytes.NewReader(d))
		if err != nil {
			t.Fatalf("i=%d n=%d: %v", i, n, err)
		}
		if got != Identifier(d) {
			t.Fatalf("i=%d n=%d: stream != in-memory", i, n)
		}
	}
}

// Verifies: REQ-PR-002
func TestPropChunkingInvariant(t *testing.T) {
	for i := 0; i < 30; i++ {
		n := Block/2 + i*7919 // span sub-block to multi-block
		d := fillBytes(n, int64(100+i))
		want := Identifier(d)
		for _, r := range []interface {
			Read([]byte) (int, error)
		}{
			iotest.OneByteReader(bytes.NewReader(d)),
			iotest.HalfReader(bytes.NewReader(d)),
			bytes.NewReader(d),
		} {
			got, err := IdentifierFromReader(r)
			if err != nil {
				t.Fatalf("i=%d: %v", i, err)
			}
			if got != want {
				t.Fatalf("i=%d: chunking changed the identifier", i)
			}
		}
	}
}

// Verifies: REQ-PR-003
func TestPropByteFlipChangesIdentifier(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 40; i++ {
		n := 1 + r.Intn(2*Block)
		d := fillBytes(n, int64(i))
		base := Identifier(d)
		e := append([]byte{}, d...)
		pos := r.Intn(n)
		e[pos] ^= 1 << uint(r.Intn(8))
		if Identifier(e) == base {
			t.Fatalf("i=%d: byte flip at %d did not change the identifier", i, pos)
		}
	}
}

// Verifies: REQ-PR-004
func TestPropManifestRoundtripAndMutation(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := 0; i < 30; i++ {
		length := r.Uint64()
		tree := TreeRoot(fillBytes(r.Intn(64), int64(i)))
		treeHex := hex.EncodeToString(tree[:])
		m := ManifestBytes(length, treeHex)

		gotLen, gotTree, err := ParseManifest(m)
		if err != nil || gotLen != length || gotTree != treeHex {
			t.Fatalf("i=%d: roundtrip failed: %v", i, err)
		}
		// Mutate one byte: it is either rejected or parses to a genuinely
		// different (length, tree) — never silently the original meaning.
		mut := append([]byte{}, m...)
		pos := r.Intn(len(mut))
		mut[pos] ^= 1 << uint(r.Intn(8))
		if bytes.Equal(mut, m) {
			continue
		}
		if l2, t2, err := ParseManifest(mut); err == nil && l2 == length && t2 == treeHex {
			t.Fatalf("i=%d: mutated manifest normalized back to the original", i)
		}
	}
}
