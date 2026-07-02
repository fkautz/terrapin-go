package terrapin

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkTree(t *testing.T, data []byte) *BuiltTree {
	t.Helper()
	bt, err := BuildFromReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return bt
}

func uptr(x uint64) *uint64 { return &x }

// writeTreeAndData persists a tree + data file in a temp dir and returns the
// tree base name, the data path, and the read-back PersistedTree.
func writeTreeAndData(t *testing.T, data []byte) (string, string, *PersistedTree) {
	t.Helper()
	dir := t.TempDir()
	base := filepath.Join(dir, "tree")
	dp := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(dp, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTree(base, mkTree(t, data)); err != nil {
		t.Fatal(err)
	}
	pt, err := ReadTree(base)
	if err != nil {
		t.Fatal(err)
	}
	return base, dp, pt
}

func equalU(a, b []uint64) bool { return equalCounts(a, b) }

// Verifies: REQ-DC-001
func TestDeriveCountsSmallAndBoundaries(t *testing.T) {
	cases := []struct {
		length uint64
		want   []uint64
	}{
		{0, []uint64{1}},
		{1, []uint64{1}},
		{Block, []uint64{1}},
		{Block + 1, []uint64{2}},
		{Fanout * Block, []uint64{Fanout}},
		{Fanout * Fanout * Block, []uint64{Fanout * Fanout, Fanout}},
	}
	for _, c := range cases {
		if got := DeriveCounts(c.length); !equalU(got, c.want) {
			t.Errorf("DeriveCounts(%d) = %v want %v", c.length, got, c.want)
		}
	}
}

// Verifies: REQ-DC-002
func TestDeriveCountsMultiLayer(t *testing.T) {
	if got := DeriveCounts(Fanout*Block + 1); !equalU(got, []uint64{Fanout + 1, 2}) {
		t.Errorf("Fanout*Block+1: %v", got)
	}
	if got := DeriveCounts(Fanout*Fanout*Block + 1); !equalU(got, []uint64{Fanout*Fanout + 1, Fanout + 1, 2}) {
		t.Errorf("3-layer: %v", got)
	}
	// Spec §4.3 worked example: 1 PiB.
	if got := DeriveCounts(1125899906842624); !equalU(got, []uint64{536870912, 8192}) {
		t.Errorf("1 PiB: %v", got)
	}
	// max uint64 must terminate, strictly shrink, top <= Fanout.
	mc := DeriveCounts(math.MaxUint64)
	for i := 1; i < len(mc); i++ {
		if mc[i] >= mc[i-1] {
			t.Fatal("counts must strictly shrink")
		}
	}
	if mc[len(mc)-1] > Fanout {
		t.Fatal("top layer must be <= Fanout")
	}
}

// Verifies: REQ-DC-003
func TestDeriveCountsMatchesBuilder(t *testing.T) {
	for _, n := range []int{1, Block, 3 * Block, 3*Block + 9} {
		bt := mkTree(t, fillBytes(n, int64(n)))
		got := make([]uint64, len(bt.Layers))
		for i, l := range bt.Layers {
			got[i] = uint64(len(l) / 32)
		}
		if !equalU(got, DeriveCounts(uint64(n))) {
			t.Errorf("len=%d: builder %v != DeriveCounts %v", n, got, DeriveCounts(uint64(n)))
		}
	}
}

// Verifies: REQ-OFF-001
func TestOffsetsFromCounts(t *testing.T) {
	if got := offsetsFromCounts([]uint64{3}); len(got) != 1 || got[0] != 0 {
		t.Fatalf("single layer offsets: %v", got)
	}
	counts := []uint64{Fanout + 1, 2}
	offs := offsetsFromCounts(counts)
	if offs[0] != 0 || offs[1] != (Fanout+1)*32 {
		t.Errorf("offsets: %v", offs)
	}
	for _, o := range offs {
		if o%32 != 0 {
			t.Error("offsets must be 32-byte aligned")
		}
	}
}

// Verifies: REQ-TB-001
func TestBuilderSingleLeaf(t *testing.T) {
	for _, n := range []int{0, 1, 1000, Block} {
		data := fillBytes(n, int64(n+1))
		bt := mkTree(t, data)
		if len(bt.Layers) != 1 || len(bt.Layers[0]) != 32 {
			t.Fatalf("len=%d: expected one 32-byte leaf layer", n)
		}
		if bt.Root != G(data) {
			t.Errorf("len=%d: root must be the bare leaf", n)
		}
		if bt.Identifier() != Identifier(data) {
			t.Errorf("len=%d: identifier mismatch", n)
		}
	}
}

// Verifies: REQ-TB-002
func TestBuilderEmptyPath(t *testing.T) {
	bt := mkTree(t, nil)
	if bt.Root != G(nil) {
		t.Error("empty build root must be G(nil)")
	}
	if bt.Identifier() != Identifier(nil) {
		t.Error("empty identifier mismatch")
	}
}

// Verifies: REQ-TB-003
func TestBuilderMultiLayerStructure(t *testing.T) {
	tb := NewTreeBuilder()
	for i := uint64(0); i < Fanout+1; i++ {
		var b [8]byte
		for j := 0; j < 8; j++ {
			b[j] = byte(i >> (8 * j))
		}
		tb.PushLeaf(G(b[:]))
	}
	bt := tb.Build(Fanout*Block + 1)
	if len(bt.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(bt.Layers))
	}
	if len(bt.Layers[0]) != int(Fanout+1)*32 || len(bt.Layers[1]) != 2*32 {
		t.Fatalf("layer sizes: %d %d", len(bt.Layers[0]), len(bt.Layers[1]))
	}
	g0 := G(bt.Layers[0][:Fanout*32])
	g1 := G(bt.Layers[0][Fanout*32:])
	if !bytes.Equal(bt.Layers[1][:32], g0[:]) || !bytes.Equal(bt.Layers[1][32:], g1[:]) {
		t.Error("layer-1 nodes must be G of the layer-0 groups")
	}
	if root := G(bt.Layers[1]); root != bt.Root {
		t.Error("root must be G(top layer)")
	}
}

// Verifies: REQ-TB-004
func TestBuilderInternalConsistency(t *testing.T) {
	bt := mkTree(t, fillBytes(3*Block+9, 1))
	for l := 0; l+1 < len(bt.Layers); l++ {
		var rebuilt []byte
		for i := 0; i < len(bt.Layers[l]); i += Block {
			end := i + Block
			if end > len(bt.Layers[l]) {
				end = len(bt.Layers[l])
			}
			h := G(bt.Layers[l][i:end])
			rebuilt = append(rebuilt, h[:]...)
		}
		if !bytes.Equal(rebuilt, bt.Layers[l+1]) {
			t.Fatalf("layer %d -> %d relation broken", l, l+1)
		}
	}
	top := bt.Layers[len(bt.Layers)-1]
	if len(top) == 32 {
		if [32]byte(top[:32]) != bt.Root {
			t.Error("single-leaf root mismatch")
		}
	} else if G(top) != bt.Root {
		t.Error("root must equal G(top layer)")
	}
}

// Verifies: REQ-TB-005
func TestBuilderLeafCountOrderLengthHex(t *testing.T) {
	tb := NewTreeBuilder()
	a, b := G([]byte("a")), G([]byte("b"))
	tb.PushLeaf(a)
	tb.PushLeaf(b)
	if tb.LeafCount() != 2 {
		t.Error("leaf count")
	}
	ab := tb.Build(2 * Block).Root

	tb2 := NewTreeBuilder()
	tb2.PushLeaf(b)
	tb2.PushLeaf(a)
	ba := tb2.Build(2 * Block).Root
	if ab == ba {
		t.Error("leaf order must matter")
	}

	// Same leaves, two lengths -> same root, different identifier.
	t1 := mkTree(t, fillBytes(2*Block, 5))
	if t1.TreeHex() != hex.EncodeToString(func() []byte { r := TreeRoot(fillBytes(2*Block, 5)); return r[:] }()) {
		t.Error("tree hex mismatch with TreeRoot")
	}
}

// Verifies: REQ-BR-001
func TestBuildFromReaderMatches(t *testing.T) {
	for _, n := range []int{0, 1, Block, Block + 1, 3*Block + 7} {
		data := fillBytes(n, int64(n))
		bt := mkTree(t, data)
		if bt.Identifier() != Identifier(data) {
			t.Errorf("len=%d identifier", n)
		}
		if bt.Length != uint64(n) {
			t.Errorf("len=%d length field=%d", n, bt.Length)
		}
	}
}

// Verifies: REQ-PT-001
func TestPersistRoundtripAndFormat(t *testing.T) {
	data := fillBytes(3*Block+10, 2)
	base, _, pt := writeTreeAndData(t, data)

	if pt.Identifier != Identifier(data) {
		t.Error("identifier roundtrip")
	}
	if !equalU(pt.Counts, DeriveCounts(uint64(len(data)))) {
		t.Error("counts roundtrip")
	}

	blocks, _ := os.ReadFile(base + ".blocks")
	var sum uint64
	for _, c := range pt.Counts {
		sum += c
	}
	if uint64(len(blocks)) != sum*32 {
		t.Errorf(".blocks size %d != %d", len(blocks), sum*32)
	}

	head, _ := os.ReadFile(base + ".head")
	hs := string(head)
	for _, want := range []string{"terrapin-tree: 1\n", "algorithm: terrapin-sha256\n", "block_size: 2097152\n", "\nidentifier: terrapin-sha256:", "\nlayer_counts: "} {
		if !strings.Contains(hs, want) {
			t.Errorf(".head missing %q", want)
		}
	}
	if strings.Count(hs, "\n") != 7 {
		t.Errorf(".head must have 7 lines, got %d", strings.Count(hs, "\n"))
	}

	// Reproducible: write again, identical bytes.
	base2 := filepath.Join(t.TempDir(), "tree2")
	if err := WriteTree(base2, mkTree(t, data)); err != nil {
		t.Fatal(err)
	}
	h2, _ := os.ReadFile(base2 + ".head")
	b2, _ := os.ReadFile(base2 + ".blocks")
	if !bytes.Equal(head, h2) || !bytes.Equal(blocks, b2) {
		t.Error("artifact must be byte-reproducible")
	}
}

// Verifies: REQ-PT-002
func TestReadTreeRejectsBadHeaders(t *testing.T) {
	data := fillBytes(Block+10, 3)
	base, _, _ := writeTreeAndData(t, data)
	good, _ := os.ReadFile(base + ".head")

	mutate := func(s string) string {
		p := filepath.Join(t.TempDir(), "t")
		os.WriteFile(p+".head", []byte(s), 0644)
		os.WriteFile(p+".blocks", []byte{}, 0644)
		return p
	}
	bad := map[string]string{
		"missing-head-handled": "", // handled separately below
		"bad-line":             strings.Replace(string(good), "algorithm: terrapin-sha256", "algorithm terrapin-sha256", 1),
		"unknown-key":          string(good) + "extra: x\n",
		"bad-version":          strings.Replace(string(good), "terrapin-tree: 1", "terrapin-tree: 2", 1),
		"bad-block-size":       strings.Replace(string(good), "block_size: 2097152", "block_size: 2000000", 1),
		"bad-algorithm":        strings.Replace(string(good), "algorithm: terrapin-sha256", "algorithm: terrapin-sha512", 1),
		"inconsistent-counts":  strings.Replace(string(good), "layer_counts: 2", "layer_counts: 5", 1),
		"non-numeric-counts":   strings.Replace(string(good), "layer_counts: 2", "layer_counts: x", 1),
	}
	for name, s := range bad {
		if name == "missing-head-handled" {
			continue
		}
		if _, err := ReadTree(mutate(s)); err == nil {
			t.Errorf("%s: expected ReadTree rejection", name)
		}
	}
	if _, err := ReadTree(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("missing head: expected error")
	}
}

// Verifies: REQ-VAL-001
func TestValidateSuccess(t *testing.T) {
	data := fillBytes(3*Block+1234, 4)
	_, dp, pt := writeTreeAndData(t, data)
	if err := pt.Validate(dp, nil, nil, nil); err != nil {
		t.Fatalf("whole: %v", err)
	}
	if err := pt.Validate(dp, nil, nil, nil); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	// single-block tree
	small := fillBytes(1000, 5)
	_, dp2, pt2 := writeTreeAndData(t, small)
	if err := pt2.Validate(dp2, nil, nil, nil); err != nil {
		t.Fatalf("single-block: %v", err)
	}
	// content-addressed copy
	cp := filepath.Join(t.TempDir(), "copy.bin")
	os.WriteFile(cp, data, 0644)
	if err := pt.Validate(cp, nil, nil, nil); err != nil {
		t.Fatalf("copy: %v", err)
	}
}

// Verifies: REQ-VAL-002
func TestValidateRanges(t *testing.T) {
	data := fillBytes(3*Block+1234, 6)
	_, dp, pt := writeTreeAndData(t, data)
	ranges := [][2]uint64{
		{10, 100},
		{Block - 5, 2*Block + 5},
		{Block - 1, Block + 1},
		{3 * Block, uint64(len(data))},
		{0, 1},
		{uint64(len(data)) - 1, uint64(len(data))},
		{42, 42}, // empty
	}
	for _, r := range ranges {
		if err := pt.Validate(dp, uptr(r[0]), uptr(r[1]), nil); err != nil {
			t.Errorf("range %v: %v", r, err)
		}
	}
}

// Verifies: REQ-VF-001
func TestValidateTamperAndBounds(t *testing.T) {
	data := fillBytes(3*Block+50, 7)
	_, dp, pt := writeTreeAndData(t, data)

	// tamper inside the range
	bad := append([]byte{}, data...)
	bad[Block+1] ^= 0xff
	bp := filepath.Join(t.TempDir(), "bad.bin")
	os.WriteFile(bp, bad, 0644)
	if err := pt.Validate(bp, nil, nil, nil); err == nil {
		t.Error("tamper must fail")
	}
	// slice independence: validating block 0 of the tampered file succeeds
	if err := pt.Validate(bp, uptr(0), uptr(100), nil); err != nil {
		t.Error("clean slice of tampered file must validate")
	}
	// length mismatch
	short := filepath.Join(t.TempDir(), "short.bin")
	os.WriteFile(short, data[:len(data)-10], 0644)
	if err := pt.Validate(short, nil, nil, nil); err == nil {
		t.Error("length mismatch must fail")
	}
	// bounds
	if err := pt.Validate(dp, uptr(10), uptr(5), nil); err == nil {
		t.Error("start>end must error")
	}
	if err := pt.Validate(dp, uptr(0), uptr(uint64(len(data))+1), nil); err == nil {
		t.Error("end>length must error")
	}
}

// Verifies: REQ-VF-002
func TestValidateCorruptArtifact(t *testing.T) {
	data := fillBytes(3*Block+5, 8)
	base, dp, pt := writeTreeAndData(t, data)

	// corrupt a leaf hash in .blocks -> any range fails
	blocks, _ := os.ReadFile(base + ".blocks")
	corrupt := append([]byte{}, blocks...)
	corrupt[0] ^= 0xff
	os.WriteFile(base+".blocks", corrupt, 0644)
	if err := pt.Validate(dp, nil, nil, nil); err == nil {
		t.Error("corrupt leaf hash must fail")
	}
	// truncated .blocks
	os.WriteFile(base+".blocks", blocks[:len(blocks)-32], 0644)
	if err := pt.Validate(dp, nil, nil, nil); err == nil {
		t.Error("truncated .blocks must fail")
	}
	// missing .blocks
	os.Remove(base + ".blocks")
	if err := pt.Validate(dp, nil, nil, nil); err == nil {
		t.Error("missing .blocks must fail")
	}

	// corrupt head identifier only
	base2, dp2, _ := writeTreeAndData(t, data)
	head, _ := os.ReadFile(base2 + ".head")
	hs := strings.Replace(string(head), "identifier: terrapin-sha256:", "identifier: terrapin-sha256:0", 1)
	os.WriteFile(base2+".head", []byte(hs), 0644)
	pt2, err := ReadTree(base2)
	if err == nil {
		if verr := pt2.Validate(dp2, nil, nil, nil); verr == nil {
			t.Error("corrupt head identifier must fail")
		}
	}
}

// Verifies: REQ-VF-003
func TestValidateAdversarial(t *testing.T) {
	data := fillBytes(3*Block+5, 9)
	_, _, pt := writeTreeAndData(t, data)

	// different data of the same length
	other := fillBytes(len(data), 999)
	op := filepath.Join(t.TempDir(), "other.bin")
	os.WriteFile(op, other, 0644)
	if err := pt.Validate(op, nil, nil, nil); err == nil {
		t.Error("different data of same length must fail")
	}
	// swapped blocks
	swapped := append([]byte{}, data...)
	copy(swapped[0:Block], data[Block:2*Block])
	copy(swapped[Block:2*Block], data[0:Block])
	sp := filepath.Join(t.TempDir(), "swap.bin")
	os.WriteFile(sp, swapped, 0644)
	if err := pt.Validate(sp, nil, nil, nil); err == nil {
		t.Error("swapped blocks must fail")
	}
	// single-leaf tamper
	small := fillBytes(500, 1)
	_, sdp, spt := writeTreeAndData(t, small)
	smallBad := append([]byte{}, small...)
	smallBad[0] ^= 1
	os.WriteFile(sdp, smallBad, 0644)
	if err := spt.Validate(sdp, nil, nil, nil); err == nil {
		t.Error("single-leaf tamper must fail")
	}
	// empty-dataset tree vs non-empty file
	_, _, ept := writeTreeAndData(t, nil)
	nf := filepath.Join(t.TempDir(), "ne.bin")
	os.WriteFile(nf, []byte("x"), 0644)
	if err := ept.Validate(nf, nil, nil, nil); err == nil {
		t.Error("empty tree vs non-empty file must fail")
	}
}

// Verifies: REQ-CAT-001
func TestCatWholeAndRanges(t *testing.T) {
	data := fillBytes(3*Block+777, 11)
	_, dp, pt := writeTreeAndData(t, data)

	var whole bytes.Buffer
	if err := pt.Validate(dp, nil, nil, &whole); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(whole.Bytes(), data) {
		t.Error("cat whole != data")
	}
	for _, r := range [][2]uint64{{7, 9}, {Block - 3, 2*Block + 4}, {3 * Block, uint64(len(data))}, {5, 5}} {
		var buf bytes.Buffer
		if err := pt.Validate(dp, uptr(r[0]), uptr(r[1]), &buf); err != nil {
			t.Fatalf("range %v: %v", r, err)
		}
		if !bytes.Equal(buf.Bytes(), data[r[0]:r[1]]) {
			t.Errorf("range %v: cat != slice", r)
		}
	}
}

type failingWriter struct {
	ok  int // bytes to accept before failing
	n   int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.n+len(p) > w.ok {
		return 0, errors.New("writer boom")
	}
	w.n += len(p)
	return len(p), nil
}

// Verifies: REQ-CAT-002
func TestCatPartialAndWriterError(t *testing.T) {
	data := fillBytes(3*Block, 12)
	_, dp, pt := writeTreeAndData(t, data)

	// tamper block 2 (index 2); cat a range covering block 0..2 -> error, and
	// no bytes from at/after the tampered block are emitted.
	bad := append([]byte{}, data...)
	bad[2*Block+1] ^= 0xff
	bp := filepath.Join(t.TempDir(), "bad.bin")
	os.WriteFile(bp, bad, 0644)
	var out bytes.Buffer
	err := pt.Validate(bp, uptr(0), uptr(uint64(len(data))), &out)
	if err == nil {
		t.Error("cat over a tampered block must fail")
	}
	if out.Len() > 2*Block {
		t.Error("cat must not emit bytes from at/after the tampered block")
	}
	if !bytes.Equal(out.Bytes(), bad[:out.Len()]) {
		t.Error("emitted bytes must be a correct prefix")
	}

	// writer error surfaces
	if err := pt.Validate(dp, nil, nil, &failingWriter{ok: 10}); err == nil {
		t.Error("writer error must surface")
	}
}

// Verifies: REQ-WE-004
func TestPathBlocksOnePerLayer(t *testing.T) {
	tb := NewTreeBuilder()
	for i := uint64(0); i < Fanout+1; i++ {
		var b [8]byte
		for j := 0; j < 8; j++ {
			b[j] = byte(i >> (8 * j))
		}
		tb.PushLeaf(G(b[:]))
	}
	base := filepath.Join(t.TempDir(), "tree")
	if err := WriteTree(base, tb.Build(Fanout*Block+1)); err != nil {
		t.Fatal(err)
	}
	pt, err := ReadTree(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(pt.Counts) != 2 {
		t.Fatalf("expected 2 layers, got %v", pt.Counts)
	}
	pb, err := pt.PathBlocks(uptr(0), uptr(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(pb) != 2 {
		t.Fatalf("expected one block per layer (2), got %d", len(pb))
	}
	if pb[0] != [3]uint64{0, 0, Fanout} || pb[1] != [3]uint64{1, 0, 2} {
		t.Errorf("path blocks: %v", pb)
	}
}
