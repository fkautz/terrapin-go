package terrapin

import (
	"crypto/sha256"
	"io"
	"math/rand"
	"strconv"
)

// fillBytes returns deterministic pseudo-random bytes for reproducible tests.
func fillBytes(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	return b
}

// framedSHA256 is an INDEPENDENT expression of G's spec formula
// (sha256("blob " + decimal(len) + "\0" + data)), used as an oracle so the G
// tests do not merely restate the implementation.
func framedSHA256(data []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("blob " + strconv.Itoa(len(data)) + "\x00"))
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// zeroReader streams `remaining` zero bytes without allocating the dataset.
type zeroReader struct{ remaining int64 }

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.remaining {
		n = z.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 0
	}
	z.remaining -= n
	return int(n), nil
}
