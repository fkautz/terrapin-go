package terrapin

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

// Verifies: REQ-SB-001
func TestStreamMatchesInMemory(t *testing.T) {
	for _, n := range []int{0, 1, 1000, Block - 1, Block, Block + 1, 2 * Block} {
		d := fillBytes(n, int64(n+11))
		got, err := IdentifierFromReader(bytes.NewReader(d))
		if err != nil {
			t.Fatalf("len=%d: %v", n, err)
		}
		if want := Identifier(d); got != want {
			t.Errorf("len=%d: stream %s != in-memory %s", n, got, want)
		}
	}
}

// Verifies: REQ-SB-002
func TestStreamShortReads(t *testing.T) {
	d := fillBytes(2*Block+12345, 3)
	want := Identifier(d)
	readers := map[string]io.Reader{
		"one-byte": iotest.OneByteReader(bytes.NewReader(d)),
		"half":     iotest.HalfReader(bytes.NewReader(d)),
		"data-err": iotest.DataErrReader(bytes.NewReader(d)),
	}
	for name, r := range readers {
		got, err := IdentifierFromReader(r)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: %s != %s", name, got, want)
		}
	}
}

// Verifies: REQ-SB-003
func TestStreamEmpty(t *testing.T) {
	got, err := IdentifierFromReader(bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got != Identifier(nil) {
		t.Errorf("empty stream %s != Identifier(nil) %s", got, Identifier(nil))
	}
}

// Verifies: REQ-SB-004
func TestStreamSingleShortBlock(t *testing.T) {
	d := fillBytes(1234, 4)
	got, err := IdentifierFromReader(bytes.NewReader(d))
	if err != nil {
		t.Fatal(err)
	}
	if got != Identifier(d) {
		t.Error("single short block mismatch")
	}
}

// Verifies: REQ-SB-005
func TestStreamMultiBlockBoundaries(t *testing.T) {
	for _, n := range []int{Block, Block + 1, 2 * Block, 2*Block + 1, 3*Block + 7} {
		d := fillBytes(n, int64(n))
		got, err := IdentifierFromReader(bytes.NewReader(d))
		if err != nil {
			t.Fatalf("len=%d: %v", n, err)
		}
		if got != Identifier(d) {
			t.Errorf("len=%d mismatch", n)
		}
	}
}

// Verifies: REQ-SB-006
func TestStreamReaderError(t *testing.T) {
	boom := errors.New("boom")
	r := io.MultiReader(bytes.NewReader(fillBytes(Block+10, 5)), iotest.ErrReader(boom))
	got, err := IdentifierFromReader(r)
	if err == nil {
		t.Error("reader error must be surfaced")
	}
	if got != "" {
		t.Errorf("on error the identifier must be empty, got %q", got)
	}
}

// Verifies: REQ-SB-007
func TestStreamTwoLayerOracle(t *testing.T) {
	if testing.Short() {
		t.Skip("128 GiB streaming; run without -short")
	}
	n := uint64(Block)*(Block/32) + 1 // FANOUT+1 blocks -> 2 layers
	got, err := IdentifierFromReader(&zeroReader{remaining: int64(n)})
	if err != nil {
		t.Fatal(err)
	}
	want := idFromParts(n, treeRootZero(n))
	if got != want {
		t.Errorf("2-layer stream %s != oracle %s", got, want)
	}
}

// Verifies: REQ-SB-008
func TestStreamLargeMatchesInMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("64 MiB; run without -short")
	}
	d := fillBytes(64<<20, 8)
	got, err := IdentifierFromReader(bytes.NewReader(d))
	if err != nil {
		t.Fatal(err)
	}
	if got != Identifier(d) {
		t.Error("64 MiB stream mismatch")
	}
}
