package main

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	terrapin "github.com/fkautz/terrapin-go"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "terrapin-cli")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "terrapin")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func run(args ...string) (stdout, stderr string, code int) {
	cmd := exec.Command(binPath, args...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return o.String(), e.String(), code
}

func fill(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	return b
}

// dataAndTree writes a data file and an attested tree, returning the data,
// data path, and tree base name.
func dataAndTree(t *testing.T, n int, seed int64) ([]byte, string, string) {
	t.Helper()
	dir := t.TempDir()
	data := fill(n, seed)
	dp := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(dp, data, 0644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "tree")
	if _, _, code := run("attest", "-out", base, dp); code != 0 {
		t.Fatalf("attest failed (code %d)", code)
	}
	return data, dp, base
}

// Verifies: REQ-CLI-001
func TestCLIID(t *testing.T) {
	data, dp, _ := dataAndTree(t, 3*terrapin.Block+10, 1)
	out, _, code := run("id", dp)
	if code != 0 {
		t.Fatalf("id exit %d", code)
	}
	if strings.TrimSpace(out) != terrapin.Identifier(data) {
		t.Errorf("id %q != lib %q", strings.TrimSpace(out), terrapin.Identifier(data))
	}
	if _, _, code := run("id", filepath.Join(t.TempDir(), "nope")); code == 0 {
		t.Error("id on missing file must exit non-zero")
	}
}

// Verifies: REQ-CLI-002
func TestCLIAttest(t *testing.T) {
	data, dp, base := dataAndTree(t, 3*terrapin.Block, 2)
	for _, ext := range []string{".head", ".blocks"} {
		if _, err := os.Stat(base + ext); err != nil {
			t.Errorf("attest did not write %s", ext)
		}
	}
	out, _, _ := run("attest", "-out", base, dp)
	if strings.TrimSpace(out) != terrapin.Identifier(data) {
		t.Error("attest id != lib identifier")
	}
	idOut, _, _ := run("id", dp)
	if strings.TrimSpace(out) != strings.TrimSpace(idOut) {
		t.Error("attest id != id command")
	}
}

// Verifies: REQ-CLI-003
func TestCLIValidateRangeAndBounds(t *testing.T) {
	data, dp, base := dataAndTree(t, 3*terrapin.Block+50, 3)
	if _, _, code := run("validate", "-tree", base, dp); code != 0 {
		t.Error("whole validate must exit 0")
	}
	if _, _, code := run("validate", "-tree", base, "-start", "10", "-end", "100", dp); code != 0 {
		t.Error("range validate must exit 0")
	}
	oob := strconv.Itoa(len(data) + 100)
	if _, _, code := run("validate", "-tree", base, "-end", oob, dp); code == 0 {
		t.Error("out-of-bounds end must exit non-zero")
	}
}

// Verifies: REQ-CLI-004
func TestCLITamperFails(t *testing.T) {
	_, dp, base := dataAndTree(t, 3*terrapin.Block, 4)
	raw, _ := os.ReadFile(dp)
	raw[terrapin.Block+5] ^= 0xff
	os.WriteFile(dp, raw, 0644)
	if _, _, code := run("validate", "-tree", base, dp); code == 0 {
		t.Error("validate on tamper must exit non-zero")
	}
	if _, _, code := run("cat", "-tree", base, dp); code == 0 {
		t.Error("cat on tamper must exit non-zero")
	}
}

// Verifies: REQ-CLI-005
func TestCLICatRange(t *testing.T) {
	data, dp, base := dataAndTree(t, 3*terrapin.Block+9, 5)
	s, e := terrapin.Block-3, 2*terrapin.Block+4
	out, _, code := run("cat", "-tree", base, "-start", strconv.Itoa(s), "-end", strconv.Itoa(e), dp)
	if code != 0 {
		t.Fatalf("cat exit %d", code)
	}
	if !bytes.Equal([]byte(out), data[s:e]) {
		t.Error("cat range != data slice")
	}
}

// Verifies: REQ-CLI-006
func TestCLIBadArgs(t *testing.T) {
	if _, _, code := run(); code == 0 {
		t.Error("no subcommand must exit non-zero")
	}
	if _, _, code := run("bogus"); code == 0 {
		t.Error("unknown subcommand must exit non-zero")
	}
	_, dp, _ := dataAndTree(t, 100, 6)
	if _, _, code := run("validate", dp); code == 0 {
		t.Error("validate without -tree must exit non-zero")
	}
}

// Verifies: REQ-CLI-007
func TestCLIValidateTrustedIdentifier(t *testing.T) {
	data, dp, base := dataAndTree(t, 3*terrapin.Block, 7)
	id := terrapin.Identifier(data)
	if _, _, code := run("validate", "-tree", base, "-identifier", id, dp); code != 0 {
		t.Error("correct trusted identifier must exit 0")
	}
	wrong := "terrapin-sha256:" + strings.Repeat("0", 64)
	if _, _, code := run("validate", "-tree", base, "-identifier", wrong, dp); code == 0 {
		t.Error("wrong trusted identifier must exit non-zero")
	}
}
