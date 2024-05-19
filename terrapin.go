package terrapin

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/edwarnicke/gitoid"
	"io"
)

// Terrapin is a package for creating and verifying data attestations using SHA-256 hashes.
// The process involves reading data in chunks, hashing each chunk, and storing these hashes (attestations).
// The hashes can later be used to verify the integrity of the data by comparing computed hashes against the stored attestations.

type Terrapin struct {
	attestations []byte         // Byte slice to store SHA-256 hashes of data chunks
	buffer       []byte         // Buffer to hold data before hashing
	finalized    bool           // Boolean to indicate if the attestation process is finalized
	gid          *gitoid.GitOID // Pointer to the final gitoid representing the attested data
}

// BufferCapacity defines the maximum size of the buffer (2MB)
const BufferCapacity = 1024 * 1024 * 2 // 2MB buffer capacity

// NewTerrapin initializes and returns a new Terrapin instance with an empty buffer and attestations
func NewTerrapin() *Terrapin {
	return &Terrapin{
		attestations: []byte{},
		buffer:       make([]byte, 0, BufferCapacity),
		finalized:    false,
	}
}

// NewTerrapinWithAttestations initializes and returns a new Terrapin instance with provided attestations
func NewTerrapinWithAttestations(attestations []byte) (*Terrapin, error) {
	// Ensure the attestations length is a multiple of the SHA-256 size
	if len(attestations)%sha256.Size != 0 {
		return nil, errors.New("invalid attestations: length is not a multiple of SHA-256 size")
	}

	res := &Terrapin{
		attestations: attestations,
		buffer:       make([]byte, 0, BufferCapacity),
		finalized:    false,
	}

	// Finalize the Terrapin instance immediately
	_, _, _ = res.Finalize()

	return res, nil
}

// updateHashBuffer hashes the current buffer content, appends the hash to attestations, and resets the buffer
func (t *Terrapin) updateHashBuffer() error {
	// If buffer is empty, nothing to do
	if len(t.buffer) == 0 {
		return nil
	}

	// Create a new gitoid for the current buffer content
	gitoidHash, err := gitoid.New(bytes.NewReader(t.buffer), gitoid.WithSha256())
	if err != nil {
		return err
	}
	hash := gitoidHash.Bytes()

	// Append the hash to attestations
	t.attestations = append(t.attestations, hash...)

	// Reset the buffer for the next round
	t.buffer = t.buffer[:0]
	return nil
}

// Add adds data to the buffer, and processes the buffer if it reaches capacity
func (t *Terrapin) Add(data []byte) error {
	// Ensure the Terrapin instance is not finalized
	if t.finalized {
		return &AlreadyFinalizedError{}
	}

	// Copy data to the buffer in chunks, processing the buffer if it reaches capacity
	copied := 0
	for copied < len(data) {
		toCopy := min(len(data)-copied, BufferCapacity-len(t.buffer))
		t.buffer = append(t.buffer, data[copied:copied+toCopy]...)
		copied += toCopy

		// If buffer reaches capacity, update the hash buffer
		if len(t.buffer) >= BufferCapacity {
			if err := t.updateHashBuffer(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Finalize finalizes the attestation process by hashing any remaining buffer content
// Returns the gitoid URI, attestations, and any error encountered
func (t *Terrapin) Finalize() (string, []byte, error) {
	// Ensure the Terrapin instance is not already finalized
	if !t.finalized {
		// Update the hash buffer for any remaining data
		if err := t.updateHashBuffer(); err != nil {
			return "", nil, err
		}
		// Create a new gitoid for the final attestations
		gid, err := gitoid.New(bytes.NewReader(t.attestations), gitoid.WithSha256())
		if err != nil {
			return "", nil, fmt.Errorf("failed to hash terrapin: %w", err)
		}
		t.gid = gid
		t.finalized = true
	}
	// Return the gitoid URI and a copy of the attestations
	return t.gid.URI(), append([]byte(nil), t.attestations...), nil
}

// VerifyBuffer verifies the entire data stream from the reader against the attestations
// Returns true if verification succeeds, false otherwise
func (t *Terrapin) VerifyBuffer(reader io.Reader) (bool, error) {
	// Ensure the Terrapin instance is finalized
	if !t.finalized {
		return false, errors.New("terrapin not finalized")
	}

	// Buffer to read data in chunks
	buffer := make([]byte, BufferCapacity)
	offset := 0

	// Read data from the reader in chunks and verify against attestations
	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return false, err
		}
		if n == 0 {
			break
		}

		// Create a new gitoid for the current chunk of data
		gid, err := gitoid.New(bytes.NewReader(buffer[:n]), gitoid.WithSha256())
		if err != nil {
			return false, err
		}
		computedHash := gid.Bytes()
		attestationIndex := (offset / BufferCapacity) * sha256.Size
		expectedHash := t.attestations[attestationIndex : attestationIndex+sha256.Size]

		// Compare the computed hash with the expected hash
		if !bytes.Equal(computedHash, expectedHash) {
			return false, nil // Hash mismatch
		}

		offset += n
	}

	return true, nil // All hashes match
}

// VerifyBufferRange verifies a specific range of data from the reader against the attestations
// Returns true if verification succeeds, false otherwise
func (t *Terrapin) VerifyBufferRange(reader io.Reader, startOffset, endOffset int) (bool, error) {
	// Ensure the Terrapin instance is finalized
	if !t.finalized {
		return false, errors.New("terrapin not finalized")
	}

	// Validate the range
	if startOffset < 0 || endOffset <= startOffset {
		return false, errors.New("invalid range")
	}

	// Buffer to read data in chunks
	buffer := make([]byte, BufferCapacity)
	offset := startOffset

	// Align startOffset to BufferCapacity boundary
	startAlignedOffset := (startOffset / BufferCapacity) * BufferCapacity
	attestationStartIndex := (startAlignedOffset / BufferCapacity) * sha256.Size

	// Align endOffset to BufferCapacity boundary
	endAlignedOffset := ((endOffset + BufferCapacity - 1) / BufferCapacity) * BufferCapacity
	attestationEndIndex := (endAlignedOffset / BufferCapacity) * sha256.Size

	// Read data from the reader in chunks and verify against attestations
	for attestationIndex := attestationStartIndex; attestationIndex < attestationEndIndex; attestationIndex += sha256.Size {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return false, err
		}
		if n == 0 {
			break
		}

		// Create a new gitoid for the current chunk of data
		gid, err := gitoid.New(bytes.NewReader(buffer[:n]), gitoid.WithSha256())
		if err != nil {
			return false, err
		}
		computedHash := gid.Bytes()

		// Compare the computed hash with the expected hash
		expectedHash := t.attestations[attestationIndex : attestationIndex+sha256.Size]

		if !bytes.Equal(computedHash, expectedHash) {
			return false, nil // Hash mismatch
		}

		offset += n
	}

	return true, nil // All hashes match
}

// AlreadyFinalizedError is an error type for when the Terrapin instance is already finalized
type AlreadyFinalizedError struct{}

// Error implements the error interface for AlreadyFinalizedError
func (e *AlreadyFinalizedError) Error() string {
	return "terrapin attestor already finalized"
}
