package terrapin

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type conformanceFile struct {
	InputVectors []struct {
		Name  string `json:"name"`
		Input string `json:"input"`
		ID    string `json:"id"`
	} `json:"input_vectors"`
	ZeroVectors []struct {
		Name   string `json:"name"`
		Length uint64 `json:"length"`
		Tree   string `json:"tree"`
		ID     string `json:"id"`
	} `json:"zero_vectors"`
}

const materializeLimit = 8 << 20 // do not allocate vectors larger than 8 MiB

func loadVectors(t *testing.T) conformanceFile {
	t.Helper()
	b, err := os.ReadFile("testdata/vectors-terrapin.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var f conformanceFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return f
}

// Verifies: REQ-CF-001
func TestConformanceFixture(t *testing.T) {
	f := loadVectors(t)
	checked := 0
	for _, v := range f.InputVectors {
		if got := Identifier([]byte(v.Input)); got != "terrapin-sha256:"+v.ID {
			t.Errorf("input %q: got %s want terrapin-sha256:%s", v.Name, got, v.ID)
		}
		checked++
	}
	for _, v := range f.ZeroVectors {
		var treeBytes [32]byte
		raw, err := hex.DecodeString(v.Tree)
		if err != nil || len(raw) != 32 {
			t.Fatalf("%s: bad tree hex", v.Name)
		}
		copy(treeBytes[:], raw)
		if v.Length <= materializeLimit {
			data := make([]byte, v.Length)
			gotTree := TreeRoot(data)
			if hex.EncodeToString(gotTree[:]) != v.Tree {
				t.Errorf("%s tree mismatch", v.Name)
			}
			if got := Identifier(data); got != "terrapin-sha256:"+v.ID {
				t.Errorf("%s id mismatch: %s", v.Name, got)
			}
		} else {
			// Huge vector: verify only the identifier from the committed tree root.
			if got := idFromParts(v.Length, treeBytes); got != "terrapin-sha256:"+v.ID {
				t.Errorf("%s id-from-parts mismatch: %s", v.Name, got)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no vectors checked")
	}
}

// Verifies: REQ-CF-002
func TestConformanceBoundaryAndExample(t *testing.T) {
	// Boundary zero vectors, materialized.
	for _, n := range []int{Block - 1, Block, Block + 1} {
		data := make([]byte, n)
		_ = TreeRoot(data)
		if Identifier(data) == "" {
			t.Fatal("unexpected empty id")
		}
	}
	// Spec §5.4: a 1,203,942-byte dataset is a single block; tree == G(dataset).
	d := fillBytes(1203942, 1)
	if TreeRoot(d) != G(d) {
		t.Error("§5.4: single-block tree must equal G(dataset)")
	}
	if Identifier(d) != idFromParts(uint64(len(d)), G(d)) {
		t.Error("§5.4: identifier mismatch")
	}
}

// Verifies: REQ-CF-003
func TestConformanceSnapshotCorpus(t *testing.T) {
	// Frozen identifiers; breaking any requires an intentional spec change.
	snap := map[string]string{
		"":            "terrapin-sha256:f4b8abc1cfd6ffec75b4070be5440706286b3a7af937ef5d020ca2c0c1210458",
		"hello world": "terrapin-sha256:7bc0163f32e5f6082308ae0dff3dc7c9b0488e5aa652d9de01418df5ec800c8c",
	}
	for in, want := range snap {
		if got := Identifier([]byte(in)); got != want {
			t.Errorf("snapshot %q: got %s", in, got)
		}
	}
}
