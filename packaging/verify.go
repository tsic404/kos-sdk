package packaging

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/validate"
)

var (
	// ErrMalformedPackage reports an archive that is not a package, including
	// one that exceeds the read budget.
	ErrMalformedPackage = errors.New("package archive is malformed")
	// ErrUndeclaredEntry reports content outside the fixed package structure.
	ErrUndeclaredEntry = errors.New("package carries an entry its structure does not declare")
	// ErrDigestMismatch reports content that does not match its digest.
	ErrDigestMismatch = errors.New("package digest does not match its content")
	// ErrInvalidDigest reports a digest that is not in the "sha256:<hex>" form.
	ErrInvalidDigest = errors.New("digest is not a sha256 digest")
)

const (
	// MaxEntries is the most entries Verify reads from one archive.
	MaxEntries = 4096
	// MaxUncompressedBytes is the most content Verify materializes from one
	// archive. A package is untrusted input, so the budget is enforced while
	// reading, before anything is materialized or hashed.
	MaxUncompressedBytes int64 = 256 << 20
)

// Limits bounds what Verify reads from an untrusted archive. A non-positive
// field falls back to MaxEntries or MaxUncompressedBytes.
type Limits struct {
	Entries           int
	UncompressedBytes int64
}

func (l Limits) resolved() Limits {
	if l.Entries <= 0 {
		l.Entries = MaxEntries
	}
	if l.UncompressedBytes <= 0 {
		l.UncompressedBytes = MaxUncompressedBytes
	}
	return l
}

// Verify parses a received package and runs the checks an install path runs
// before installing: the structure carries only declared entries, the digest
// covers the manifest and every asset, and the declaration is valid. It applies
// the default read budget.
//
// expected is the digest the caller was promised for this artifact, normally
// its catalog address; an empty expected checks only the package's own digest.
// The comparison is over the parsed digest, so the case of the wire form does
// not matter.
func Verify(archive []byte, expected Digest) (Package, error) {
	return VerifyWithLimits(archive, expected, Limits{})
}

// VerifyWithLimits is Verify with an explicit read budget.
func VerifyWithLimits(archive []byte, expected Digest, limits Limits) (Package, error) {
	pkg, err := readPackage(archive, limits.resolved())
	if err != nil {
		return Package{}, err
	}
	if expected == "" {
		return pkg, nil
	}
	promised, ok := parseDigest(string(expected))
	if !ok {
		return Package{}, fmt.Errorf("%w: expected %q", ErrInvalidDigest, expected)
	}
	if promised != pkg.Digest {
		return Package{}, fmt.Errorf("%w: promised %s, package carries %s",
			ErrDigestMismatch, promised, pkg.Digest)
	}
	return pkg, nil
}

func readPackage(archive []byte, limits Limits) (Package, error) {
	manifestJSON, digestText, assets, err := readEntries(archive, limits)
	if err != nil {
		return Package{}, err
	}

	declared, ok := parseDigest(digestText)
	if !ok {
		return Package{}, fmt.Errorf("%w: digest entry %q", ErrMalformedPackage, digestText)
	}
	ordered := sortedAssets(assets)
	computed := digestOf(manifestJSON, ordered)
	if declared != computed {
		return Package{}, fmt.Errorf("%w: entry says %s, content hashes to %s",
			ErrDigestMismatch, declared, computed)
	}

	var declaration map[string]any
	if err := json.Unmarshal(manifestJSON, &declaration); err != nil {
		return Package{}, fmt.Errorf("%w: decode manifest: %v", ErrMalformedPackage, err)
	}
	checked := manifest.ValidateExtensionManifest(declaration)
	if !checked.OK() {
		return Package{}, &validate.InvalidError{
			What:   "packaged declaration",
			Issues: checked.Issues,
		}
	}
	if issues := checkAssets(*checked.Manifest, ordered); len(issues) > 0 {
		return Package{}, &validate.InvalidError{
			What:   "package assets",
			Issues: issues,
		}
	}

	return Package{Manifest: *checked.Manifest, Assets: ordered, Digest: computed}, nil
}

// readEntries walks the archive once, rejecting anything outside the fixed
// structure: one manifest, one digest, and regular files under the asset
// namespace, each exactly once. Both budgets are charged before content is
// materialized, so a small archive that expands far beyond them is refused
// without being read.
func readEntries(archive []byte, limits Limits) ([]byte, string, []Asset, error) {
	decompressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: %v", ErrMalformedPackage, err)
	}
	defer decompressed.Close()

	var (
		manifestJSON []byte
		digestText   string
		hasManifest  bool
		hasDigest    bool
		assets       []Asset
		entries      int
		remaining    = limits.UncompressedBytes
	)
	seen := make(map[string]bool)
	reader := tar.NewReader(decompressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: %v", ErrMalformedPackage, err)
		}
		entries++
		if entries > limits.Entries {
			return nil, "", nil, fmt.Errorf("%w: more than %d entries", ErrMalformedPackage, limits.Entries)
		}
		if seen[header.Name] {
			return nil, "", nil, fmt.Errorf("%w: entry %q appears twice", ErrUndeclaredEntry, header.Name)
		}
		seen[header.Name] = true
		if header.Typeflag != tar.TypeReg {
			return nil, "", nil, fmt.Errorf("%w: entry %q is not a regular file", ErrUndeclaredEntry, header.Name)
		}

		// The tar header states the entry size, so the budget is decided before
		// any content is read: an entry that would overspend it is refused
		// without materializing a single byte.
		if header.Size > remaining {
			return nil, "", nil, fmt.Errorf("%w: content exceeds the %d byte budget",
				ErrMalformedPackage, limits.UncompressedBytes)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, "", nil, fmt.Errorf("%w: read %q: %v", ErrMalformedPackage, header.Name, err)
		}
		remaining -= int64(len(content))

		switch {
		case header.Name == ManifestEntry:
			manifestJSON, hasManifest = content, true
		case header.Name == DigestEntry:
			digestText, hasDigest = string(content), true
		case isAssetPath(header.Name):
			assets = append(assets, Asset{Path: header.Name, Content: content})
		default:
			return nil, "", nil, fmt.Errorf("%w: entry %q", ErrUndeclaredEntry, header.Name)
		}
	}
	if !hasManifest || !hasDigest {
		return nil, "", nil, fmt.Errorf("%w: manifest or digest entry is missing", ErrMalformedPackage)
	}
	return manifestJSON, digestText, assets, nil
}

// parseDigest accepts the wire form in any case and returns its canonical
// lowercase spelling, so a promised digest and a carried digest compare equal.
func parseDigest(text string) (Digest, bool) {
	value := strings.ToLower(strings.TrimSpace(text))
	if !strings.HasPrefix(value, "sha256:") {
		return "", false
	}
	sum := strings.TrimPrefix(value, "sha256:")
	if len(sum) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", false
	}
	return Digest("sha256:" + sum), true
}
