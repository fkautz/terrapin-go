package terrapin

// Terrapin v0.3 (profile terrapin-sha256). See docs: the identifier is
// G(canonical root manifest), NOT the bare recursive tree root. Block size is
// pinned to exactly 2,097,152 bytes and layering is a total function of length
// (no optional layer-skipping). This is a breaking change from the v0.2 API in
// terrapin.go, which is retained for migration only.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Block is the exact Terrapin block size (2 MiB, not 2,000,000).
const Block = 2097152

// G computes GitOID SHA-256: sha256("blob " + decimal(len) + "\0" + data).
func G(data []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("blob "))
	h.Write([]byte(strconv.Itoa(len(data))))
	h.Write([]byte{0})
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// TreeRoot computes the recursive tree root T(data). The recursion is total:
// every layer is full, none skipped. The result is an intermediate value, not
// an identifier.
func TreeRoot(data []byte) [32]byte {
	if len(data) <= Block {
		return G(data)
	}
	hashFile := make([]byte, 0, ((len(data)+Block-1)/Block)*32)
	for i := 0; i < len(data); i += Block {
		end := i + Block
		if end > len(data) {
			end = len(data)
		}
		h := G(data[i:end])
		hashFile = append(hashFile, h[:]...)
	}
	return TreeRoot(hashFile)
}

// ManifestBytes returns the canonical root manifest bytes for the given dataset
// length and tree root (64 lowercase hex). Every line, including the last, is
// LF-terminated.
func ManifestBytes(length uint64, treeHex string) []byte {
	return []byte(fmt.Sprintf("terrapin: sha256\nblock_size: %d\nlength: %d\ntree: %s\n",
		Block, length, treeHex))
}

// Identifier returns the Terrapin v0.3 identifier "terrapin-sha256:<64 hex>" =
// G(canonical manifest).
func Identifier(data []byte) string {
	tree := TreeRoot(data)
	m := ManifestBytes(uint64(len(data)), hex.EncodeToString(tree[:]))
	id := G(m)
	return "terrapin-sha256:" + hex.EncodeToString(id[:])
}

// IdentifierFromReader streams the input in Block-sized chunks so very large
// datasets (e.g. 128 GiB) need not be held in memory. Only the level-0 hash file
// (one G per 2 MiB block) is buffered; reduction reuses TreeRoot. The result is
// byte-identical to Identifier over the same bytes.
func IdentifierFromReader(r io.Reader) (string, error) {
	buf := make([]byte, Block)
	hashFile := make([]byte, 0, 1<<20)
	var total uint64
	var single [32]byte
	blocks := 0
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			h := G(buf[:n])
			if blocks == 0 {
				single = h
			}
			hashFile = append(hashFile, h[:]...)
			total += uint64(n)
			blocks++
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	var tree [32]byte
	switch {
	case blocks == 0: // empty input: one empty block, tree = G(empty)
		tree = G(nil)
	case blocks == 1: // single (possibly short) block: tree = G(block)
		tree = single
	default: // multi-block: reduce the level-0 hash file exactly as TreeRoot does
		tree = TreeRoot(hashFile)
	}
	m := ManifestBytes(total, hex.EncodeToString(tree[:]))
	id := G(m)
	return "terrapin-sha256:" + hex.EncodeToString(id[:]), nil
}

// ParseManifest validates and parses a canonical root manifest. A manifest that
// is not in exactly canonical form is rejected (not normalized).
func ParseManifest(b []byte) (length uint64, treeHex string, err error) {
	s := string(b)
	if !strings.HasSuffix(s, "\n") {
		return 0, "", errors.New("manifest: missing final LF")
	}
	lines := strings.Split(s, "\n")
	if len(lines) != 5 || lines[4] != "" {
		return 0, "", errors.New("manifest: must be exactly 4 LF-terminated lines")
	}
	keys := [4]string{"terrapin", "block_size", "length", "tree"}
	vals := [4]string{}
	for i := 0; i < 4; i++ {
		prefix := keys[i] + ": "
		if !strings.HasPrefix(lines[i], prefix) {
			return 0, "", fmt.Errorf("manifest: line %d not %q-prefixed", i, prefix)
		}
		v := lines[i][len(prefix):]
		if v != strings.TrimSpace(v) {
			return 0, "", fmt.Errorf("manifest: extra whitespace in line %d", i)
		}
		vals[i] = v
	}
	if vals[0] != "sha256" {
		return 0, "", errors.New("manifest: algorithm must be sha256")
	}
	if vals[1] != strconv.Itoa(Block) {
		return 0, "", errors.New("manifest: block_size must be 2097152")
	}
	if !isCanonicalDecimal(vals[2]) {
		return 0, "", errors.New("manifest: length not canonical decimal")
	}
	if !isLowerHex64(vals[3]) {
		return 0, "", errors.New("manifest: tree must be 64 lowercase hex")
	}
	n, perr := strconv.ParseUint(vals[2], 10, 64)
	if perr != nil {
		return 0, "", perr
	}
	return n, vals[3], nil
}

func isCanonicalDecimal(s string) bool {
	if s == "" {
		return false
	}
	if s == "0" {
		return true
	}
	if s[0] == '0' { // no leading zeros
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
