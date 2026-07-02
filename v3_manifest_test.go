package terrapin

import (
	"math"
	"strings"
	"testing"
)

const goodTreeHex = "fee53a18d32820613c0527aa79be5cb30173c823a9b448fa4817767cc84c6f03"

// Verifies: REQ-MAN-001
func TestManifestBytesShape(t *testing.T) {
	m := string(ManifestBytes(11, goodTreeHex))
	want := "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + goodTreeHex + "\n"
	if m != want {
		t.Errorf("manifest shape:\n got %q\nwant %q", m, want)
	}
	if !strings.HasSuffix(m, "\n") {
		t.Error("manifest must end with LF")
	}
	if n := strings.Count(m, "\n"); n != 4 {
		t.Errorf("manifest must have 4 LF-terminated lines, got %d", n)
	}
}

// Verifies: REQ-MAN-002
func TestManifestValueVsDigestPrefix(t *testing.T) {
	m := string(ManifestBytes(11, goodTreeHex))
	if !strings.HasPrefix(m, "terrapin: sha256\n") {
		t.Error("manifest algorithm value must be the bare 'sha256'")
	}
	if strings.Contains(m, "terrapin-sha256") {
		t.Error("manifest must not contain the digest prefix 'terrapin-sha256:'")
	}
	if !strings.HasPrefix(Identifier(nil), "terrapin-sha256:") {
		t.Error("digest must carry the 'terrapin-sha256:' prefix")
	}
}

// Verifies: REQ-MAN-003
func TestManifestLengthAndBlockSize(t *testing.T) {
	m := string(ManifestBytes(1203942, goodTreeHex))
	if !strings.Contains(m, "\nlength: 1203942\n") {
		t.Error("length field must be the byte length")
	}
	if !strings.Contains(m, "\nblock_size: 2097152\n") {
		t.Error("block_size must be the literal 2097152")
	}
}

// Verifies: REQ-MAN-004
func TestParseManifestAcceptsAndRoundtrips(t *testing.T) {
	for _, n := range []uint64{0, 1, 11, math.MaxUint64} {
		b := ManifestBytes(n, goodTreeHex)
		gotN, tr, err := ParseManifest(b)
		if err != nil {
			t.Fatalf("length %d rejected: %v", n, err)
		}
		if gotN != n || tr != goodTreeHex {
			t.Errorf("roundtrip mismatch: n=%d tr=%s", gotN, tr)
		}
	}
}

// Verifies: REQ-MAN-006
func TestParseManifestRejectsSpacing(t *testing.T) {
	tr := goodTreeHex
	bad := map[string]string{
		"no-space":      "terrapin:sha256\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
		"double-space":  "terrapin: sha256\nblock_size: 2097152\nlength:  11\ntree: " + tr + "\n",
		"tab-after":     "terrapin:\tsha256\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
		"leading-ws":    " terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
		"trailing-ws":   "terrapin: sha256 \nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
	}
	for name, s := range bad {
		if _, _, err := ParseManifest([]byte(s)); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

// Verifies: REQ-MAN-007
func TestParseManifestRejectsValues(t *testing.T) {
	tr := goodTreeHex
	bad := map[string]string{
		"len-leading-zero": "terrapin: sha256\nblock_size: 2097152\nlength: 011\ntree: " + tr + "\n",
		"len-sign":         "terrapin: sha256\nblock_size: 2097152\nlength: +11\ntree: " + tr + "\n",
		"len-sep":          "terrapin: sha256\nblock_size: 2097152\nlength: 1,1\ntree: " + tr + "\n",
		"len-empty":        "terrapin: sha256\nblock_size: 2097152\nlength: \ntree: " + tr + "\n",
		"block-size":       "terrapin: sha256\nblock_size: 2000000\nlength: 11\ntree: " + tr + "\n",
		"tree-uppercase":   "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + strings.ToUpper(tr) + "\n",
		"tree-short":       "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: abcd\n",
		"tree-nonhex":      "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + strings.Repeat("g", 64) + "\n",
		"algo":             "terrapin: sha512\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
	}
	for name, s := range bad {
		if _, _, err := ParseManifest([]byte(s)); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

// Verifies: REQ-MAN-008
func TestParseManifestRejectsStructural(t *testing.T) {
	tr := goodTreeHex
	bad := map[string]string{
		"missing-final-lf": "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tr,
		"wrong-order":      "block_size: 2097152\nterrapin: sha256\nlength: 11\ntree: " + tr + "\n",
		"missing-key":      "terrapin: sha256\nblock_size: 2097152\ntree: " + tr + "\n",
		"extra-key":        "terrapin: sha256\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\nextra: x\n",
		"blank-line":       "terrapin: sha256\n\nblock_size: 2097152\nlength: 11\ntree: " + tr + "\n",
	}
	for name, s := range bad {
		if _, _, err := ParseManifest([]byte(s)); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

// Verifies: REQ-MAN-009
func TestParseManifestNeverNormalizes(t *testing.T) {
	// A defect that a normalizer might "fix" (extra space) must be rejected, not
	// silently accepted as the canonical (11, goodTreeHex).
	s := "terrapin: sha256\nblock_size: 2097152\nlength:  11\ntree: " + goodTreeHex + "\n"
	if _, _, err := ParseManifest([]byte(s)); err == nil {
		t.Error("non-canonical manifest must be rejected, never normalized")
	}
}
