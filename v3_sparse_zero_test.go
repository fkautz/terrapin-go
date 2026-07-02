package terrapin

import "testing"

// buildSparseZeroObject builds the recursive tree for a pure-zero object of
// fullBlocks full 2 MiB blocks plus tailBytes trailing zero bytes, using G()
// and Build() without materializing the (multi-GiB) dataset. Every full block's
// leaf is Z_0 = G(2 MiB zeros); the final partial block's leaf is G(tailBytes
// zeros) -- identical to what BuildFromReader would push, without re-hashing
// 66560 identical zero blocks.
func buildSparseZeroObject(fullBlocks, tailBytes int) *BuiltTree {
	z0 := G(make([]byte, Block))
	tb := NewTreeBuilder()
	for i := 0; i < fullBlocks; i++ {
		tb.PushLeaf(z0)
	}
	length := uint64(fullBlocks) * uint64(Block)
	if tailBytes > 0 {
		tb.PushLeaf(G(make([]byte, tailBytes)))
		length += uint64(tailBytes)
	}
	return tb.Build(length)
}

// TestSparseZeroTreesReproduceHugeVectors rebuilds the large zero vectors from
// scratch through Build(). The conformance fixture only re-derives the manifest
// step for >8 MiB vectors from a committed tree root; this exercises the actual
// multi-level recurrence, including the 130 GiB + 1 partial-tail vector -- the
// blind cross-prediction reproducing the clean-room oracle's value.
func TestSparseZeroTreesReproduceHugeVectors(t *testing.T) {
	cases := []struct {
		name                 string
		fullBlocks, tailBytes int
		tree, id             string
	}{
		{"128GiB", 65536, 0,
			"9e7e7e12b71c2b008302a4e4f5abe5b012025a8bd59d9ea5aa187f187a165599",
			"8d03319328c6d6b3cd00566d894443b2a82d31437b580ee533c2021d82bdb5a4"},
		{"128GiB+1", 65536, 1,
			"73a1fd09b7b403e607c6ae58d0a4b0ac774d2b8519289ce3a1c85dbad6683316",
			"6f552f944f4995878c7facc92c29c3643aaafc2a5bff90e255bbf430210d551b"},
		{"130GiB+1", 66560, 1,
			"9e7c35ee337543af728d04b5d16fba6d12f8f2c6b814925d214c1eefdf09cb16",
			"266a590c4206a2edfc2b2200b872b515cb35a2bd9dabb7556f6450c7419c84c3"},
	}
	for _, c := range cases {
		bt := buildSparseZeroObject(c.fullBlocks, c.tailBytes)
		if bt.TreeHex() != c.tree {
			t.Errorf("%s tree: got %s want %s", c.name, bt.TreeHex(), c.tree)
		}
		if got := bt.Identifier(); got != "terrapin-sha256:"+c.id {
			t.Errorf("%s id: got %s want terrapin-sha256:%s", c.name, got, c.id)
		}
	}
}
