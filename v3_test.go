package terrapin

import (
	"encoding/hex"
	"testing"
)

// Golden vectors from the LLIFS conformance oracle
// (llifs/conformance/vectors-terrapin.json). These ARE the v0.3 ground truth.

type vec struct {
	name     string
	length   uint64
	treeHex  string
	idHex    string // without the "terrapin-sha256:" prefix
}

var vectors = []vec{
	{"empty", 0,
		"473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813",
		"f4b8abc1cfd6ffec75b4070be5440706286b3a7af937ef5d020ca2c0c1210458"},
	{"one-zero-byte", 1,
		"449e9b795420cd16fe60ad5298cf680f15a7cd2ac9b44adaf7ed3edc0d08dd78",
		"dce39f984d9c140e4ad8f4b448a2ae6ae5398ed1adbb4d07ed8bedbc5b3b4598"},
	{"block-minus-1", Block - 1,
		"1024ef65054fcdb76a56b6fe00712dbc0007be8c65ee3902fa6c6b8c2fd7f09f",
		"dc7f0a33cf02e7a84fc380a41d396b451c96325a633a87528ebf797621befad7"},
	{"exactly-one-block", Block,
		"67cbed9b97ddabde2863f4daefa4f57176567a7c3ccfa1560c1065f9c8af74d6",
		"6fbd6447c2d8d70a83ae159461847a1a410679900702433dd2b04d063a3b2f9b"},
	{"block-plus-1", Block + 1,
		"18010af5fe70aa45e486608a97516f30410dc75c934c2486f985494990b54602",
		"5ba8049ae8f68a47acd4fad265c8a963aa82735e90f209dd79ff8d6d2188fdc5"},
}

// Verifies: REQ-TR-001
func TestVectorsZeroData(t *testing.T) {
	for _, v := range vectors {
		if v.name == "empty" {
			continue // covered by TestExplicit
		}
		data := make([]byte, v.length)
		gotTree := TreeRoot(data)
		if hex.EncodeToString(gotTree[:]) != v.treeHex {
			t.Errorf("%s tree: got %x want %s", v.name, gotTree, v.treeHex)
		}
		if got := Identifier(data); got != "terrapin-sha256:"+v.idHex {
			t.Errorf("%s id: got %s want terrapin-sha256:%s", v.name, got, v.idHex)
		}
	}
}

// Verifies: REQ-ID-001
func TestExplicit(t *testing.T) {
	// empty
	if got := Identifier(nil); got != "terrapin-sha256:"+vectors[0].idHex {
		t.Errorf("empty id: got %s", got)
	}
	// G(empty) must equal the SHA-256 git empty-blob hash (cross-check).
	ge := G(nil)
	if hex.EncodeToString(ge[:]) != "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813" {
		t.Errorf("G(empty) mismatch: %x", ge)
	}
	// "hello world"
	if got := Identifier([]byte("hello world")); got !=
		"terrapin-sha256:7bc0163f32e5f6082308ae0dff3dc7c9b0488e5aa652d9de01418df5ec800c8c" {
		t.Errorf("hello-world id: got %s", got)
	}
}

// treeRootZero computes T(n zero bytes) without materializing n bytes, so the
// 128 GiB recursion-boundary vectors are testable. The leaf G(zero-block) it
// relies on is itself validated by the exactly-one-block vector.
func treeRootZero(n uint64) [32]byte {
	if n <= Block {
		return TreeRoot(make([]byte, n))
	}
	full := n / Block
	rem := n % Block
	leaf := G(make([]byte, Block))
	hf := make([]byte, 0, (full+1)*32)
	for i := uint64(0); i < full; i++ {
		hf = append(hf, leaf[:]...)
	}
	if rem > 0 {
		r := G(make([]byte, rem))
		hf = append(hf, r[:]...)
	}
	return TreeRoot(hf)
}

// Verifies: REQ-TR-002
func TestRecursionBoundaryVectors(t *testing.T) {
	cases := []vec{
		{"65536-full-blocks", 65536 * Block,
			"9e7e7e12b71c2b008302a4e4f5abe5b012025a8bd59d9ea5aa187f187a165599",
			"8d03319328c6d6b3cd00566d894443b2a82d31437b580ee533c2021d82bdb5a4"},
		{"65536-full-blocks-plus-1-byte", 65536*Block + 1,
			"73a1fd09b7b403e607c6ae58d0a4b0ac774d2b8519289ce3a1c85dbad6683316",
			"6f552f944f4995878c7facc92c29c3643aaafc2a5bff90e255bbf430210d551b"},
	}
	for _, v := range cases {
		tree := treeRootZero(v.length)
		if hex.EncodeToString(tree[:]) != v.treeHex {
			t.Errorf("%s tree: got %x want %s", v.name, tree, v.treeHex)
		}
		m := ManifestBytes(v.length, hex.EncodeToString(tree[:]))
		id := G(m)
		if hex.EncodeToString(id[:]) != v.idHex {
			t.Errorf("%s id: got %x want %s", v.name, id, v.idHex)
		}
	}
}

// Verifies: REQ-MAN-005
func TestManifestAcceptReject(t *testing.T) {
	good := ManifestBytes(11, "fee53a18d32820613c0527aa79be5cb30173c823a9b448fa4817767cc84c6f03")
	if n, tr, err := ParseManifest(good); err != nil || n != 11 || len(tr) != 64 {
		t.Fatalf("canonical manifest rejected: n=%d err=%v", n, err)
	}
	tree := "fee53a18d32820613c0527aa79be5cb30173c823a9b448fa4817767cc84c6f03"
	rejects := map[string][]byte{
		"uppercase-hex":    ManifestBytes(11, "FEE53A18d32820613c0527aa79be5cb30173c823a9b448fa4817767cc84c6f03"),
		"missing-final-lf": []byte("terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tree),
		"wrong-order":      []byte("block_size: 2097152\nterrapin: sha256\nlength: 11\ntree: " + tree + "\n"),
		"leading-zero-len": []byte("terrapin: sha256\nblock_size: 2097152\nlength: 011\ntree: " + tree + "\n"),
		"double-space":     []byte("terrapin: sha256\nblock_size: 2097152\nlength:  11\ntree: " + tree + "\n"),
		"bad-block-size":   []byte("terrapin: sha256\nblock_size: 2000000\nlength: 11\ntree: " + tree + "\n"),
		"short-tree":       ManifestBytes(11, "abcd"),
		"extra-key":        []byte("terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tree + "\nextra: x\n"),
	}
	for name, b := range rejects {
		if _, _, err := ParseManifest(b); err == nil {
			t.Errorf("%s: expected rejection, got accept", name)
		}
	}
}
