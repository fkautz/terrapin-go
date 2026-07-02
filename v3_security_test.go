package terrapin

import (
	"encoding/hex"
	"testing"
)

// Verifies: REQ-SEC-001
func TestSecLengthReinterpretation(t *testing.T) {
	// The identifier commits length, so datasets of different length cannot share
	// an identifier even with a related tree root.
	small := make([]byte, 1)
	big := make([]byte, Block+1)
	if Identifier(small) == Identifier(big) {
		t.Error("different-length datasets must not share an identifier")
	}
	// Same tree root, different committed length -> different identifier.
	tree := TreeRoot(small)
	if idFromParts(1, tree) == idFromParts(1<<20, tree) {
		t.Error("length must be committed into the identifier")
	}
}

// Verifies: REQ-SEC-002
func TestSecBareRootNotIdentifier(t *testing.T) {
	d := fillBytes(2*Block+3, 1)
	tree := TreeRoot(d)
	id := Identifier(d)
	if id == hex.EncodeToString(tree[:]) || id == "terrapin-sha256:"+hex.EncodeToString(tree[:]) {
		t.Error("the bare tree root must not be accepted as the identifier")
	}
}

// Verifies: REQ-SEC-003
func TestSecNonCanonicalManifestRejected(t *testing.T) {
	tr := goodTreeHex
	for _, s := range []string{
		"terrapin: sha256\nblock_size: 2097152\nlength: 011\ntree: " + tr + "\n", // leading zero
		"terrapin: sha256\nblock_size: 2000000\nlength: 11\ntree: " + tr + "\n",  // wrong block size
		"terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tr,         // no final LF
	} {
		if _, _, err := ParseManifest([]byte(s)); err == nil {
			t.Errorf("non-canonical manifest accepted: %q", s)
		}
	}
}

// Verifies: REQ-SEC-004
func TestSecLengthFramingDetectsTruncation(t *testing.T) {
	block := fillBytes(Block, 2)
	// The blob "<len>\0" framing binds length: a truncated block hashes differently.
	if G(block) == G(block[:Block-1]) {
		t.Error("length framing must make a truncated block hash differently")
	}
	// A manifest claiming the wrong (truncated) length reproduces a different id.
	tree := TreeRoot(block)
	if idFromParts(Block, tree) == idFromParts(Block-1, tree) {
		t.Error("a truncated length must not reproduce the identifier")
	}
}

// Verifies: REQ-WE-001
func TestWorkedExampleSingleBlock(t *testing.T) {
	// Spec §5.4: a 1,203,942-byte dataset is a single block (<= Block), so the
	// tree root is G(dataset) and the identifier wraps it in the manifest.
	d := fillBytes(1203942, 3)
	if len(d) > Block {
		t.Fatal("precondition: example dataset must be a single block")
	}
	if TreeRoot(d) != G(d) {
		t.Error("single-block tree root must equal G(dataset)")
	}
	if Identifier(d) != idFromParts(uint64(len(d)), G(d)) {
		t.Error("identifier must be G(manifest(len, G(dataset)))")
	}
}
