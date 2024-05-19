package terrapin

import (
	"bytes"
	"github.com/edwarnicke/gitoid"
	"testing"
)

// Tests

func TestNewTerrapin(t *testing.T) {
	terrapin := NewTerrapin()
	if len(terrapin.attestations) != 0 {
		t.Errorf("Expected empty attestations, got %d", len(terrapin.attestations))
	}
	if len(terrapin.buffer) != 0 {
		t.Errorf("Expected empty buffer, got %d", len(terrapin.buffer))
	}
	if terrapin.finalized {
		t.Error("Expected finalized to be false")
	}
}

func TestAddData(t *testing.T) {
	terrapin := NewTerrapin()
	data := []byte{1, 2, 3, 4, 5}
	if err := terrapin.Add(data); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(terrapin.buffer) != len(data) {
		t.Errorf("Expected buffer length %d, got %d", len(data), len(terrapin.buffer))
	}
}

func TestAddDataWhenFinalized(t *testing.T) {
	terrapin := NewTerrapin()
	terrapin.Finalize()
	data := []byte{1, 2, 3, 4, 5}
	err := terrapin.Add(data)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if err.Error() != "terrapin attestor already finalized" {
		t.Errorf("Expected finalized error, got %v", err)
	}
}

func TestFinalize(t *testing.T) {
	terrapin := NewTerrapin()
	data := []byte{1, 2, 3, 4, 5}
	gitoidHash, _ := gitoid.New(bytes.NewReader(data), gitoid.WithSha256())
	hash := gitoidHash.Bytes()
	if err := terrapin.Add(data); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	gid, attestation, err := terrapin.Finalize()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if gid == "" {
		t.Errorf("expected non-empty gid")
	}
	if len(attestation) != len(hash) {
		t.Errorf("Expected hash length %d, got %d", len(hash), len(attestation))
	}
	if !bytes.Equal(attestation, hash) {
		t.Errorf("Expected hash %v, got %v", hash, attestation)
	}
}

func TestFinalizeWhenAlreadyFinalized(t *testing.T) {
	terrapin := NewTerrapin()
	gid1, attestation1, _ := terrapin.Finalize()
	gid2, attestation2, _ := terrapin.Finalize()
	if gid1 != gid2 {
		t.Errorf("Expected same gid, got %s and %s", gid1, gid2)
	}
	if !bytes.Equal(attestation1, attestation2) {
		t.Errorf("Expected same attestations, got %v and %v", attestation1, attestation2)
	}
}
