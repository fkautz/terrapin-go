package terrapin

import (
	"encoding/hex"
	"strings"
	"testing"
)

// idFromParts reconstructs the identifier from a known length and tree root,
// mirroring §5.3 (G of the canonical manifest).
func idFromParts(length uint64, tree [32]byte) string {
	m := ManifestBytes(length, hex.EncodeToString(tree[:]))
	id := G(m)
	return "terrapin-sha256:" + hex.EncodeToString(id[:])
}

// Verifies: REQ-ID-002
func TestIdentifierZeroVectors(t *testing.T) {
	for _, v := range vectors {
		if v.name == "empty" {
			continue
		}
		got := Identifier(make([]byte, v.length))
		if got != "terrapin-sha256:"+v.idHex {
			t.Errorf("%s id: got %s want terrapin-sha256:%s", v.name, got, v.idHex)
		}
	}
}

// Verifies: REQ-ID-003
func TestIdentifierEqualsManifestGitoid(t *testing.T) {
	for _, n := range []int{0, 1, Block, 2*Block + 9} {
		d := fillBytes(n, int64(n+5))
		if Identifier(d) != idFromParts(uint64(len(d)), TreeRoot(d)) {
			t.Errorf("len=%d: Identifier != G(manifest(len, TreeRoot))", n)
		}
	}
}

// Verifies: REQ-ID-004
func TestIdentifierStructure(t *testing.T) {
	id := Identifier([]byte("hello world"))
	if !strings.HasPrefix(id, "terrapin-sha256:") {
		t.Fatal("missing prefix")
	}
	rest := strings.TrimPrefix(id, "terrapin-sha256:")
	if len(rest) != 64 {
		t.Fatalf("hex part must be 64 chars, got %d", len(rest))
	}
	if rest != strings.ToLower(rest) || !isLowerHex64(rest) {
		t.Error("hex part must be 64 lowercase hex")
	}
}

// Verifies: REQ-ID-005
func TestIdentifierIsNotBareTreeRoot(t *testing.T) {
	d := fillBytes(3*Block, 9)
	tree := TreeRoot(d)
	id := Identifier(d)
	if id == hex.EncodeToString(tree[:]) {
		t.Error("identifier must not be the bare tree root hex")
	}
	if id == "gitoid:blob:sha256:"+hex.EncodeToString(tree[:]) {
		t.Error("identifier must not be an OmniBOR gitoid of the tree root")
	}
}

// Verifies: REQ-ID-006
func TestIdentifierCommitsLength(t *testing.T) {
	tree := TreeRoot(fillBytes(100, 1))
	if idFromParts(100, tree) == idFromParts(101, tree) {
		t.Error("identical tree root with different length must yield different identifiers")
	}
}

// Verifies: REQ-ID-007
func TestIdentifierDistinctInputs(t *testing.T) {
	ids := map[string]bool{}
	for _, d := range [][]byte{nil, {0}, fillBytes(Block, 1), fillBytes(Block+1, 1)} {
		id := Identifier(d)
		if ids[id] {
			t.Errorf("collision for len=%d", len(d))
		}
		ids[id] = true
	}
}

// Verifies: REQ-ID-008
func TestIdentifierSnapshot(t *testing.T) {
	snap := map[string]string{
		"":            "terrapin-sha256:f4b8abc1cfd6ffec75b4070be5440706286b3a7af937ef5d020ca2c0c1210458",
		"hello world": "terrapin-sha256:7bc0163f32e5f6082308ae0dff3dc7c9b0488e5aa652d9de01418df5ec800c8c",
	}
	for in, want := range snap {
		if got := Identifier([]byte(in)); got != want {
			t.Errorf("snapshot %q: got %s want %s", in, got, want)
		}
	}
}
