package terrapin

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Fanout is the number of 32-byte child hashes that fill one Block-sized hash
// file (Block / 32 = 65536).
const Fanout = Block / 32

// DeriveCounts returns the number of hashes at each layer, a total function of
// length and the fixed block size (spec §6 step 3). counts[0] is the leaf count.
func DeriveCounts(length uint64) []uint64 {
	var nblocks uint64
	if length == 0 {
		nblocks = 1 // empty dataset is one empty leaf
	} else {
		nblocks = (length + Block - 1) / Block
	}
	counts := []uint64{nblocks}
	for counts[len(counts)-1] > Fanout {
		prev := counts[len(counts)-1]
		counts = append(counts, (prev+Fanout-1)/Fanout)
	}
	return counts
}

func offsetsFromCounts(counts []uint64) []uint64 {
	offs := make([]uint64, len(counts))
	var acc uint64
	for i, c := range counts {
		offs[i] = acc
		acc += c * 32
	}
	return offs
}

// groupStart is the first hash index of the Fanout-sized group containing idx.
func groupStart(idx uint64) uint64 { return (idx / Fanout) * Fanout }

func identifierFromParts(length uint64, tree [32]byte) string {
	m := ManifestBytes(length, hex.EncodeToString(tree[:]))
	id := G(m)
	return "terrapin-sha256:" + hex.EncodeToString(id[:])
}

// BuiltTree is a fully built recursive tree: every layer's hash file plus root.
type BuiltTree struct {
	Length uint64
	Layers [][]byte // Layers[0] = leaf hash file; Layers[k] = topmost
	Root   [32]byte
}

// TreeHex returns the tree root as 64 lowercase hex.
func (b *BuiltTree) TreeHex() string { return hex.EncodeToString(b.Root[:]) }

// Identifier returns the terrapin-sha256 identifier (spec §5.3).
func (b *BuiltTree) Identifier() string { return identifierFromParts(b.Length, b.Root) }

// TreeBuilder accumulates leaf hashes and builds the recursive tree, retaining
// every layer so the full tree can be persisted.
type TreeBuilder struct{ leaves []byte }

func NewTreeBuilder() *TreeBuilder { return &TreeBuilder{} }

// PushLeaf appends one leaf hash (G of a data block), in block order.
func (t *TreeBuilder) PushLeaf(h [32]byte) { t.leaves = append(t.leaves, h[:]...) }

// LeafCount returns the number of leaf hashes pushed so far.
func (t *TreeBuilder) LeafCount() uint64 { return uint64(len(t.leaves) / 32) }

// Build finishes the tree for a dataset of length bytes. Requires at least one
// leaf (an empty dataset contributes one empty leaf, G("")).
func (t *TreeBuilder) Build(length uint64) *BuiltTree {
	layers := [][]byte{t.leaves}
	var root [32]byte
	if len(layers[0]) == 32 {
		// Single leaf (dataset <= Block, incl. empty): the bare leaf is the root.
		copy(root[:], layers[0][:32])
	} else {
		cur := 0
		for {
			if len(layers[cur]) <= Block {
				root = G(layers[cur])
				break
			}
			var next []byte
			for i := 0; i < len(layers[cur]); i += Block {
				end := min(i+Block, len(layers[cur]))
				h := G(layers[cur][i:end])
				next = append(next, h[:]...)
			}
			layers = append(layers, next)
			cur++
		}
	}
	return &BuiltTree{Length: length, Layers: layers, Root: root}
}

// BuildFromReader streams the input in Block-sized chunks and builds the full
// tree (every layer), never holding the dataset in memory.
func BuildFromReader(r io.Reader) (*BuiltTree, error) {
	buf := make([]byte, Block)
	tb := NewTreeBuilder()
	var total uint64
	blocks := 0
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			tb.PushLeaf(G(buf[:n]))
			total += uint64(n)
			blocks++
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if blocks == 0 {
		tb.PushLeaf(G(nil)) // empty dataset -> one empty leaf
	}
	return tb.Build(total), nil
}

const headVersion = "1"

// PersistedTree is a read handle for a published two-file tree artifact.
type PersistedTree struct {
	Length     uint64
	TreeHex    string
	Identifier string
	Counts     []uint64
	offsets    []uint64
	blocksPath string
}

// WriteTree writes the two-file artifact name.head + name.blocks.
func WriteTree(name string, tree *BuiltTree) error {
	bf, err := os.Create(name + ".blocks")
	if err != nil {
		return err
	}
	for _, layer := range tree.Layers {
		if _, err := bf.Write(layer); err != nil {
			bf.Close()
			return err
		}
	}
	if err := bf.Close(); err != nil {
		return err
	}

	counts := make([]string, len(tree.Layers))
	for i, l := range tree.Layers {
		counts[i] = strconv.FormatUint(uint64(len(l)/32), 10)
	}
	head := fmt.Sprintf(
		"terrapin-tree: %s\nalgorithm: terrapin-sha256\nblock_size: %d\nlength: %d\ntree: %s\nidentifier: %s\nlayer_counts: %s\n",
		headVersion, Block, tree.Length, tree.TreeHex(), tree.Identifier(), strings.Join(counts, " "),
	)
	return os.WriteFile(name+".head", []byte(head), 0644)
}

// ReadTree opens a persisted tree by base name.
func ReadTree(name string) (*PersistedTree, error) {
	text, err := os.ReadFile(name + ".head")
	if err != nil {
		return nil, fmt.Errorf("cannot read %s.head: %w", name, err)
	}
	pt := &PersistedTree{blocksPath: name + ".blocks"}
	var version, blockSize string
	var haveLen, haveTree, haveID, haveCounts bool
	for _, line := range strings.Split(string(text), "\n") {
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			return nil, fmt.Errorf("head: bad line %q", line)
		}
		switch k {
		case "terrapin-tree":
			version = v
		case "algorithm":
			if v != "terrapin-sha256" {
				return nil, fmt.Errorf("head: unsupported algorithm %s", v)
			}
		case "block_size":
			blockSize = v
		case "length":
			n, perr := strconv.ParseUint(v, 10, 64)
			if perr != nil {
				return nil, errors.New("head: bad length")
			}
			pt.Length = n
			haveLen = true
		case "tree":
			pt.TreeHex = v
			haveTree = true
		case "identifier":
			pt.Identifier = v
			haveID = true
		case "layer_counts":
			for _, f := range strings.Fields(v) {
				n, perr := strconv.ParseUint(f, 10, 64)
				if perr != nil {
					return nil, errors.New("head: bad layer_counts")
				}
				pt.Counts = append(pt.Counts, n)
			}
			haveCounts = true
		default:
			return nil, fmt.Errorf("head: unknown key %s", k)
		}
	}
	if version != headVersion {
		return nil, errors.New("head: unsupported terrapin-tree version")
	}
	if blockSize != strconv.Itoa(Block) {
		return nil, errors.New("head: block_size must be 2097152")
	}
	if !haveLen || !haveTree || !haveID || !haveCounts {
		return nil, errors.New("head: missing required key")
	}
	if !equalCounts(pt.Counts, DeriveCounts(pt.Length)) {
		return nil, errors.New("head: layer_counts inconsistent with length")
	}
	pt.offsets = offsetsFromCounts(pt.Counts)
	return pt, nil
}

func equalCounts(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (pt *PersistedTree) root() ([32]byte, error) {
	raw, err := hex.DecodeString(pt.TreeHex)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, errors.New("head: tree not 64 hex")
	}
	var r [32]byte
	copy(r[:], raw)
	return r, nil
}

func (pt *PersistedTree) checkIdentifier() ([32]byte, error) {
	root, err := pt.root()
	if err != nil {
		return [32]byte{}, err
	}
	if identifierFromParts(pt.Length, root) != pt.Identifier {
		return [32]byte{}, errors.New("tree: identifier does not match manifest")
	}
	return root, nil
}

// CheckAgainst asserts the tree's identifier equals a trusted one obtained
// out-of-band (spec §6 step 1). A tree forged for different data is rejected.
func (pt *PersistedTree) CheckAgainst(trusted string) error {
	if pt.Identifier != trusted {
		return fmt.Errorf("identifier mismatch: tree is %s, expected %s", pt.Identifier, trusted)
	}
	return nil
}

func (pt *PersistedTree) readGroup(bf *os.File, layer int, gstart uint64) ([]byte, error) {
	remaining := pt.Counts[layer] - gstart
	lenHashes := min(remaining, Fanout)
	off := pt.offsets[layer] + gstart*32
	grp := make([]byte, lenHashes*32)
	if _, err := bf.ReadAt(grp, int64(off)); err != nil {
		return nil, fmt.Errorf("blocks truncated: %w", err)
	}
	return grp, nil
}

type groupCache struct {
	start uint64
	bytes []byte
	node  [32]byte
	valid bool
}

// Validate checks the byte range [start, end) of dataPath against the tree,
// optionally streaming the verified bytes to w. nil start/end means the whole
// dataset. It implements the spec §6 path-based validation: only the touched
// data blocks plus one hash-file block per layer are read.
func (pt *PersistedTree) Validate(dataPath string, start, end *uint64, w io.Writer) error {
	root, err := pt.checkIdentifier()
	if err != nil {
		return err
	}
	s := uint64(0)
	if start != nil {
		s = *start
	}
	e := pt.Length
	if end != nil {
		e = *end
	}
	if s > e || e > pt.Length {
		return fmt.Errorf("range %d..%d out of bounds for length %d", s, e, pt.Length)
	}

	df, err := os.Open(dataPath)
	if err != nil {
		return err
	}
	defer df.Close()
	fi, err := df.Stat()
	if err != nil {
		return err
	}
	if uint64(fi.Size()) != pt.Length {
		return fmt.Errorf("data length %d != tree length %d", fi.Size(), pt.Length)
	}

	if pt.Length == 0 {
		if G(nil) != root {
			return errors.New("validation failed: empty dataset root mismatch")
		}
		return nil
	}
	if e == s {
		return nil
	}

	singleLeaf := pt.Counts[0] == 1
	nlayers := len(pt.Counts)
	caches := make([]groupCache, nlayers)

	bf, err := os.Open(pt.blocksPath)
	if err != nil {
		return err
	}
	defer bf.Close()

	bLo := s / Block
	bHi := (e - 1) / Block
	for i := bLo; i <= bHi; i++ {
		blockOff := i * Block
		blockLen := min(pt.Length-blockOff, Block)
		buf := make([]byte, blockLen)
		if _, err := df.ReadAt(buf, int64(blockOff)); err != nil {
			return fmt.Errorf("data read: %w", err)
		}
		h := G(buf)

		if singleLeaf {
			if h != root {
				return fmt.Errorf("validation failed at block %d", i)
			}
		} else {
			idx := i
			for l := 0; l < nlayers; l++ {
				gstart := groupStart(idx)
				posn := idx - gstart
				c := &caches[l]
				if !c.valid || c.start != gstart {
					grp, err := pt.readGroup(bf, l, gstart)
					if err != nil {
						return err
					}
					c.start, c.bytes, c.node, c.valid = gstart, grp, G(grp), true
				}
				if !bytes.Equal(c.bytes[posn*32:posn*32+32], h[:]) {
					return fmt.Errorf("validation failed at block %d (layer %d)", i, l)
				}
				h = c.node
				idx /= Fanout
			}
			if h != root {
				return fmt.Errorf("validation failed at block %d (root)", i)
			}
		}

		if w != nil {
			lo := max(s, blockOff)
			hi := min(e, blockOff+blockLen)
			if hi > lo {
				if _, err := w.Write(buf[lo-blockOff : hi-blockOff]); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			}
		}
	}
	return nil
}

// PathBlocks reports the hash-file blocks Validate reads for the range
// [start, end) — one per layer along each touched leaf's path. Structural (no
// data, no .blocks I/O); shares groupStart with Validate. Returns entries of
// {layer, groupStartIndex, groupLenHashes}.
func (pt *PersistedTree) PathBlocks(start, end *uint64) ([][3]uint64, error) {
	s := uint64(0)
	if start != nil {
		s = *start
	}
	e := pt.Length
	if end != nil {
		e = *end
	}
	if s > e || e > pt.Length {
		return nil, fmt.Errorf("range %d..%d out of bounds for length %d", s, e, pt.Length)
	}
	if pt.Length == 0 || e == s || pt.Counts[0] == 1 {
		return nil, nil
	}
	var out [][3]uint64
	bLo := s / Block
	bHi := (e - 1) / Block
	for i := bLo; i <= bHi; i++ {
		idx := i
		for l := 0; l < len(pt.Counts); l++ {
			gstart := groupStart(idx)
			length := min(pt.Counts[l]-gstart, Fanout)
			entry := [3]uint64{uint64(l), gstart, length}
			found := false
			for _, x := range out {
				if x == entry {
					found = true
					break
				}
			}
			if !found {
				out = append(out, entry)
			}
			idx /= Fanout
		}
	}
	return out, nil
}
