// Package sdktc6 verifies that a first-party system application and a
// third-party extension travel the same packing path: the two artifacts carry
// the same structure, the digest covers the manifest and every UI asset, and
// both pass the checks an install path runs (SDK-AC-5, SDK-TC-6, P1).
package sdktc6

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/packaging"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

func systemAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/icon.svg", Content: []byte("<svg><!-- files --></svg>")},
		{Path: "assets/app.html", Content: []byte("<!doctype html><title>Files</title>")},
		{Path: "assets/app.js", Content: []byte("console.log('files')")},
	}
}

func thirdPartyAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/index.html", Content: []byte("<!doctype html><title>Notes</title>")},
		{Path: "assets/notes.js", Content: []byte("console.log('notes')")},
	}
}

func pack(t *testing.T, declaration map[string]any, assets []packaging.Asset) (packaging.Package, []byte) {
	t.Helper()
	pkg, archive, err := packaging.Pack(declaration, assets)
	if err != nil {
		t.Fatalf("pack %v: %v", declaration["id"], err)
	}
	return pkg, archive
}

func readEntries(t *testing.T, archive []byte) []fixtures.Entry {
	t.Helper()
	entries, err := fixtures.ReadArchive(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return entries
}

// roots returns the top-level names of an archive: the fixed structural entries
// plus the asset namespace.
func roots(entries []fixtures.Entry) []string {
	found := make(map[string]bool, len(entries))
	for _, entry := range entries {
		found[strings.SplitN(entry.Name, "/", 2)[0]] = true
	}
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// headerShapes returns the distinct ways the packer stamped an entry: every
// entry must use the same shape, whoever the author is.
func headerShapes(entries []fixtures.Entry) []string {
	found := make(map[string]bool, len(entries))
	for _, entry := range entries {
		found[fmt.Sprintf("%c mode=%o uid=%d gid=%d mtime=%d",
			entry.Type, entry.Mode, entry.UID, entry.GID, entry.ModTime.Unix())] = true
	}
	shapes := make([]string, 0, len(found))
	for shape := range found {
		shapes = append(shapes, shape)
	}
	sort.Strings(shapes)
	return shapes
}

func assetPaths(assets []packaging.Asset) []string {
	paths := make([]string, 0, len(assets))
	for _, asset := range assets {
		paths = append(paths, asset.Path)
	}
	sort.Strings(paths)
	return paths
}

func TestBothPackagesPassTheInstallPathChecks(t *testing.T) {
	systemPackage, systemArchive := pack(t, fixtures.SystemApp(), systemAssets())
	thirdPackage, thirdArchive := pack(t, fixtures.ThirdPartyExtension(), thirdPartyAssets())

	cases := []struct {
		name    string
		pkg     packaging.Package
		archive []byte
	}{
		{name: "first-party system application", pkg: systemPackage, archive: systemArchive},
		{name: "third-party extension", pkg: thirdPackage, archive: thirdArchive},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			loaded, err := packaging.Verify(testCase.archive, testCase.pkg.Digest)
			if err != nil {
				t.Fatalf("install path rejected the package: %v", err)
			}
			if loaded.Manifest.ID != testCase.pkg.Manifest.ID {
				t.Fatalf("loaded manifest id = %q, want %q", loaded.Manifest.ID, testCase.pkg.Manifest.ID)
			}
			if len(loaded.Assets) != len(testCase.pkg.Assets) {
				t.Fatalf("loaded %d assets, want %d", len(loaded.Assets), len(testCase.pkg.Assets))
			}
		})
	}

	if systemPackage.Digest == thirdPackage.Digest {
		t.Fatal("two different packages share one digest")
	}
}

func TestStructureDoesNotDependOnTheAuthor(t *testing.T) {
	_, systemArchive := pack(t, fixtures.SystemApp(), systemAssets())
	_, thirdArchive := pack(t, fixtures.ThirdPartyExtension(), thirdPartyAssets())

	systemEntries := readEntries(t, systemArchive)
	thirdEntries := readEntries(t, thirdArchive)

	wantRoots := []string{"assets", "digest", "manifest.json"}
	if got := roots(systemEntries); !slices.Equal(got, wantRoots) {
		t.Fatalf("system application roots = %v, want %v", got, wantRoots)
	}
	if got := roots(thirdEntries); !slices.Equal(got, wantRoots) {
		t.Fatalf("third-party roots = %v, want %v", got, wantRoots)
	}
	if got := headerShapes(systemEntries); !slices.Equal(got, headerShapes(thirdEntries)) {
		t.Fatalf("entry headers differ: %v vs %v", got, headerShapes(thirdEntries))
	}
}

func TestDigestChangesWhenAnAssetChanges(t *testing.T) {
	thirdPackage, _ := pack(t, fixtures.ThirdPartyExtension(), thirdPartyAssets())

	changedAssets := thirdPartyAssets()
	changedAssets[1].Content = []byte("console.log('notes v2')")
	changedPackage, changedArchive := pack(t, fixtures.ThirdPartyExtension(), changedAssets)

	if changedPackage.Digest == thirdPackage.Digest {
		t.Fatal("changing one UI asset left the digest unchanged")
	}
	if _, err := packaging.Verify(changedArchive, changedPackage.Digest); err != nil {
		t.Fatalf("the repacked artifact was rejected: %v", err)
	}
	if _, err := packaging.Verify(changedArchive, thirdPackage.Digest); !errors.Is(err, packaging.ErrDigestMismatch) {
		t.Fatalf("the replaced artifact matched the earlier digest: %v", err)
	}
}

func TestEntriesMatchTheDeclarationsExactly(t *testing.T) {
	cases := []struct {
		name        string
		declaration map[string]any
		assets      []packaging.Asset
	}{
		{name: "first-party system application", declaration: fixtures.SystemApp(), assets: systemAssets()},
		{name: "third-party extension", declaration: fixtures.ThirdPartyExtension(), assets: thirdPartyAssets()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pkg, archive := pack(t, testCase.declaration, testCase.assets)
			entries := readEntries(t, archive)

			declared := make(map[string]bool, len(testCase.assets)+2)
			declared[packaging.ManifestEntry] = true
			declared[packaging.DigestEntry] = true
			for _, asset := range testCase.assets {
				declared[asset.Path] = true
			}
			if len(entries) != len(declared) {
				t.Fatalf("archive carries %d entries, the declaration accounts for %d", len(entries), len(declared))
			}
			for _, entry := range entries {
				if !declared[entry.Name] {
					t.Fatalf("archive carries an undeclared entry %q", entry.Name)
				}
			}

			manifestEntry, ok := fixtures.Find(entries, packaging.ManifestEntry)
			if !ok {
				t.Fatal("archive carries no manifest")
			}
			checked := manifest.ValidateExtensionManifest(testCase.declaration)
			if !checked.OK() {
				t.Fatalf("fixture declaration rejected: %v", checked.Issues)
			}
			wantManifest, err := json.Marshal(checked.Manifest)
			if err != nil {
				t.Fatalf("encode declaration: %v", err)
			}
			if !bytes.Equal(manifestEntry.Content, wantManifest) {
				t.Fatal("the packaged manifest is not the declaration it was built from")
			}

			carried := make([]string, 0, len(pkg.Assets))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name, packaging.AssetPrefix) {
					carried = append(carried, entry.Name)
				}
			}
			sort.Strings(carried)
			if !slices.Equal(carried, assetPaths(pkg.Assets)) {
				t.Fatalf("packaged assets = %v, declared assets = %v", carried, assetPaths(pkg.Assets))
			}
		})
	}
}
