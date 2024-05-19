package terrapin

import (
	"bytes"
	"io"
	"testing"
)

func setupTerrapinWithData(t *testing.T, data []byte) (*Terrapin, io.Reader) {
	terrapin := NewTerrapin()
	err := terrapin.Add(data)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}
	_, _, err = terrapin.Finalize()
	if err != nil {
		t.Fatalf("Failed to finalize terrapin: %v", err)
	}
	return terrapin, bytes.NewReader(data)
}

func TestVerifyBuffer_MatchingData(t *testing.T) {
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	terrapin, reader := setupTerrapinWithData(t, data)

	match, err := terrapin.VerifyBuffer(reader)
	if err != nil {
		t.Fatalf("VerifyBuffer returned an error: %v", err)
	}
	if !match {
		t.Fatalf("VerifyBuffer expected to match, but it didn't")
	}
}

func TestVerifyBuffer_MismatchedData(t *testing.T) {
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	terrapin, _ := setupTerrapinWithData(t, data)

	// Alter the data
	data[100] = 255
	reader := bytes.NewReader(data)

	match, err := terrapin.VerifyBuffer(reader)
	if err != nil {
		t.Fatalf("VerifyBuffer returned an error: %v", err)
	}
	if match {
		t.Fatalf("VerifyBuffer expected to mismatch, but it matched")
	}
}

func TestVerifyBufferRange_MatchingData(t *testing.T) {
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	terrapin, reader := setupTerrapinWithData(t, data)

	// Align startOffset and endOffset to BufferCapacity boundary
	startOffset := BufferCapacity
	endOffset := 2 * BufferCapacity
	reader = bytes.NewBuffer(data[startOffset:endOffset])
	match, err := terrapin.VerifyBufferRange(reader, startOffset, endOffset)
	if err != nil {
		t.Fatalf("VerifyBufferRange returned an error: %v", err)
	}
	if !match {
		t.Fatalf("VerifyBufferRange expected to match, but it didn't")
	}
}

func TestVerifyBufferRange_MismatchedData(t *testing.T) {
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	terrapin, _ := setupTerrapinWithData(t, data)

	// Alter the data within the range
	data[BufferCapacity+100] = 255

	// Align startOffset and endOffset to BufferCapacity boundary
	startOffset := BufferCapacity
	endOffset := 2 * BufferCapacity
	reader := bytes.NewReader(data[startOffset:endOffset])
	match, err := terrapin.VerifyBufferRange(reader, startOffset, endOffset)
	if err != nil {
		t.Fatalf("VerifyBufferRange returned an error: %v", err)
	}
	if match {
		t.Fatalf("VerifyBufferRange expected to mismatch, but it matched")
	}
}

func TestVerifyBufferRange_InvalidRange(t *testing.T) {
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	terrapin, reader := setupTerrapinWithData(t, data)

	startOffset := 3 * BufferCapacity / 2
	endOffset := BufferCapacity / 2
	_, err := terrapin.VerifyBufferRange(reader, startOffset, endOffset)
	if err == nil {
		t.Fatalf("VerifyBufferRange expected to return an error for invalid range, but it didn't")
	}
}

func TestVerifyBuffer_BeforeFinalization(t *testing.T) {
	terrapin := NewTerrapin()
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	reader := bytes.NewReader(data)

	match, err := terrapin.VerifyBuffer(reader)
	if err == nil || match {
		t.Fatalf("VerifyBuffer expected to return an error and not match before finalization, but it didn't")
	}
}

func TestVerifyBufferRange_BeforeFinalization(t *testing.T) {
	terrapin := NewTerrapin()
	data := make([]byte, 4*BufferCapacity)
	for i := range data {
		data[i] = byte(i % 256)
	}
	reader := bytes.NewReader(data)

	startOffset := BufferCapacity
	endOffset := 2 * BufferCapacity
	match, err := terrapin.VerifyBufferRange(reader, startOffset, endOffset)
	if err == nil || match {
		t.Fatalf("VerifyBufferRange expected to return an error and not match before finalization, but it didn't")
	}
}
