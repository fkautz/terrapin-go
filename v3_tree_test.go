package terrapin

import (
	"testing"
)

// manualRoot reduces k<=FANOUT leaf blocks the way the spec does for a single
// layer: G over the concatenation of the per-block leaf hashes (independent of
// TreeRoot's own recursion bookkeeping).
func manualSingleLayerRoot(data []byte) [32]byte {
	if len(data) <= Block {
		return G(data)
	}
	var hashFile []byte
	for i := 0; i < len(data); i += Block {
		end := i + Block
		if end > len(data) {
			end = len(data)
		}
		h := G(data[i:end])
		hashFile = append(hashFile, h[:]...)
	}
	return G(hashFile) // valid only while #blocks <= FANOUT (one wrap)
}

// Verifies: REQ-TR-003
func TestTreeRootEmptyIsGEmpty(t *testing.T) {
	if TreeRoot(nil) != G(nil) {
		t.Error("TreeRoot(empty) must equal G(empty)")
	}
}

// Verifies: REQ-TR-004
func TestTreeRootMultiBlock(t *testing.T) {
	for _, n := range []int{2 * Block, 2*Block + 1, 3 * Block, 3*Block + 7} {
		d := fillBytes(n, int64(n))
		if TreeRoot(d) != manualSingleLayerRoot(d) {
			t.Errorf("TreeRoot(len=%d) != manual single-layer reduction", n)
		}
	}
}

// Verifies: REQ-TR-005
func TestTreeRootSingleLeafIsBare(t *testing.T) {
	for _, n := range []int{0, 1, 1000, Block - 1, Block} {
		d := fillBytes(n, int64(n+1))
		root := TreeRoot(d)
		if root != G(d) {
			t.Errorf("len=%d: single-leaf root must be the bare leaf G(data)", n)
		}
		leaf := G(d)
		if root == G(leaf[:]) {
			t.Errorf("len=%d: root must not be G(leaf) (double-wrap)", n)
		}
	}
}

// Verifies: REQ-TR-006
func TestTreeRootOrderMatters(t *testing.T) {
	a := fillBytes(Block, 100)
	b := fillBytes(Block, 200)
	ab := append(append([]byte{}, a...), b...)
	ba := append(append([]byte{}, b...), a...)
	if TreeRoot(ab) == TreeRoot(ba) {
		t.Error("block order must affect TreeRoot")
	}
}

// Verifies: REQ-TR-007
func TestTreeRootAvalanche(t *testing.T) {
	d := fillBytes(3*Block+5, 7)
	base := TreeRoot(d)
	e := append([]byte{}, d...)
	e[2*Block+1] ^= 0x80
	if TreeRoot(e) == base {
		t.Error("single-byte change must propagate to TreeRoot")
	}
}
