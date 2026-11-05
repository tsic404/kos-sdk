// Package framedhash computes the "sha256:<hex>" digest over named records.
//
// Each record is framed as "<name> <length>\n<content>", so neither a renamed
// record nor a shifted boundary can reproduce an earlier digest. The package
// digest and the case-set digest share this framing, so the SDK carries one
// digest scheme rather than two.
package framedhash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
)

// Hasher accumulates named records into one digest.
type Hasher struct {
	sum hash.Hash
}

// New returns a hasher with no record.
func New() *Hasher {
	return &Hasher{sum: sha256.New()}
}

// Add appends one record. The order of calls is part of the digest.
func (h *Hasher) Add(name string, content []byte) {
	fmt.Fprintf(h.sum, "%s %d\n", name, len(content))
	h.sum.Write(content)
}

// Sum returns the digest in its lowercase "sha256:<hex>" wire form.
func (h *Hasher) Sum() string {
	return "sha256:" + hex.EncodeToString(h.sum.Sum(nil))
}
