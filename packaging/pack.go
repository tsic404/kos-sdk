// Package packaging builds and verifies the extension artifact of KOSD-C-17:
// one manifest, its UI assets, and the digest that covers both.
//
// One packing path serves every author. The packer takes a declaration and the
// asset files supplied with it, writes exactly those, and reads no product
// identity or install source, so a first-party system application and a
// third-party extension produce the same structure (SDK-C-4).
package packaging

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/tsic404/kos-sdk/internal/framedhash"
	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/validate"
)

// The package structure is fixed: these two entries plus the assets, nothing
// else. An install path reads the declaration from ManifestEntry and checks the
// content against DigestEntry.
const (
	ManifestEntry = "manifest.json"
	DigestEntry   = "digest"
	AssetPrefix   = "assets/"
)

// epoch pins archive timestamps so the same input always yields the same bytes.
var epoch = time.Unix(0, 0).UTC()

// Digest is the package digest: the SHA-256 of the manifest and every asset,
// framed with each entry's name and length so a rename changes it as well. Its
// canonical wire form is lowercase "sha256:<hex>"; a digest supplied to Verify
// is parsed first, so its case does not matter.
type Digest string

// String returns the digest in its "sha256:<hex>" wire form.
func (d Digest) String() string { return string(d) }

// Asset is one UI asset file. Path is the package-relative entry name and must
// be a clean path under AssetPrefix.
type Asset struct {
	Path    string
	Content []byte
}

// Package is the logical content of one artifact.
type Package struct {
	Manifest manifest.ExtensionManifest
	Assets   []Asset
	Digest   Digest
}

// Pack validates declaration, checks assets against it, and returns the package
// together with its archive bytes.
//
// A rejected declaration or asset set yields a *validate.InvalidError and no
// package: an author must not hold an installable artifact from a failed call.
func Pack(declaration any, assets []Asset) (Package, []byte, error) {
	checked := manifest.ValidateExtensionManifest(declaration)
	if !checked.OK() {
		return Package{}, nil, &validate.InvalidError{
			What:   "extension declaration",
			Issues: checked.Issues,
		}
	}

	if issues := checkAssets(*checked.Manifest, assets); len(issues) > 0 {
		return Package{}, nil, &validate.InvalidError{
			What:   "package assets",
			Issues: issues,
		}
	}

	encoded, err := json.Marshal(checked.Manifest)
	if err != nil {
		return Package{}, nil, fmt.Errorf("encode manifest: %w", err)
	}

	ordered := sortedAssets(assets)
	pkg := Package{
		Manifest: *checked.Manifest,
		Assets:   ordered,
		Digest:   digestOf(encoded, ordered),
	}
	archive, err := writeArchive(pkg, encoded)
	if err != nil {
		return Package{}, nil, fmt.Errorf("write package archive: %w", err)
	}
	return pkg, archive, nil
}

// checkAssets rejects asset paths the package structure cannot carry, and an
// icon that names an asset the package does not carry. An icon that is an
// origin address is left to the manifest check (KOSD-C-73).
func checkAssets(declaration manifest.ExtensionManifest, assets []Asset) []validate.Issue {
	c := &validate.Checker{}
	carried := make(map[string]bool, len(assets))
	for index, asset := range assets {
		assetPath := validate.Join(validate.Index("$.assets", index), "path")
		switch {
		case !isAssetPath(asset.Path):
			c.Add(assetPath, validate.CodeInvalidValue,
				"asset path %q must be a clean path under %q", asset.Path, AssetPrefix)
		case carried[asset.Path]:
			c.Add(assetPath, validate.CodeInvalidValue,
				"asset path %q is declared twice", asset.Path)
		default:
			carried[asset.Path] = true
		}
	}

	for index, app := range declaration.Contributions.Apps {
		if !strings.HasPrefix(app.Icon, AssetPrefix) || carried[app.Icon] {
			continue
		}
		c.Add(validate.Join(validate.Index("$.contributions.apps", index), "icon"),
			validate.CodeInvalidValue,
			"icon %q names a package asset the package does not carry", app.Icon)
	}
	return c.Issues
}

// isAssetPath reports whether name is a clean path inside the asset namespace.
// A clean path keeps the digest framing unambiguous: the name carries no
// newline, and no "." or ".." segment can escape the namespace.
func isAssetPath(name string) bool {
	return strings.HasPrefix(name, AssetPrefix) &&
		len(name) > len(AssetPrefix) &&
		path.Clean(name) == name &&
		!strings.ContainsAny(name, "\x00\n\r")
}

func sortedAssets(assets []Asset) []Asset {
	ordered := append([]Asset(nil), assets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	return ordered
}

// digestOf hashes the manifest and every asset in path order.
func digestOf(manifestJSON []byte, assets []Asset) Digest {
	hasher := framedhash.New()
	hasher.Add(ManifestEntry, manifestJSON)
	for _, asset := range assets {
		hasher.Add(asset.Path, asset.Content)
	}
	return Digest(hasher.Sum())
}

func writeArchive(pkg Package, manifestJSON []byte) ([]byte, error) {
	var buffer bytes.Buffer
	compressed, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	compressed.Header.ModTime = epoch
	compressed.Header.OS = 255

	archive := tar.NewWriter(compressed)
	if err := writeEntry(archive, ManifestEntry, manifestJSON); err != nil {
		return nil, err
	}
	for _, asset := range pkg.Assets {
		if err := writeEntry(archive, asset.Path, asset.Content); err != nil {
			return nil, err
		}
	}
	if err := writeEntry(archive, DigestEntry, []byte(pkg.Digest+"\n")); err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	if err := compressed.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeEntry(archive *tar.Writer, name string, content []byte) error {
	header := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(content)),
		ModTime:  epoch,
	}
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	_, err := archive.Write(content)
	return err
}
