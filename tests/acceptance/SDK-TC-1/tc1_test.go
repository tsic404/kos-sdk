// Package sdktc1 verifies that the smallest declarations an author can write —
// an extension contributing one page and one contributing a window application
// — pack and verify through the SDK: the produced manifest carries exactly the
// contract fields, the digest is verifiable, and a loader can take the artifact
// (SDK-AC-1, SDK-TC-1, P1).
package sdktc1

import (
	"encoding/json"
	"slices"
	"sort"
	"testing"

	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/packaging"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

func pageAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/reports.html", Content: []byte("<!doctype html><title>Reports</title>")},
	}
}

func windowAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/icon.svg", Content: []byte("<svg><!-- notes --></svg>")},
		{Path: "assets/index.html", Content: []byte("<!doctype html><title>Notes</title>")},
	}
}

func keys(record map[string]any) []string {
	found := make([]string, 0, len(record))
	for key := range record {
		found = append(found, key)
	}
	sort.Strings(found)
	return found
}

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

func decode(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(content, &record); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return record
}

func TestMinimalDeclarationsPackAndVerify(t *testing.T) {
	cases := []struct {
		name        string
		declaration map[string]any
		assets      []packaging.Asset
	}{
		{name: "minimal page extension", declaration: fixtures.MinimalPageExtension(), assets: pageAssets()},
		{name: "minimal window application", declaration: fixtures.MinimalWindowApp(), assets: windowAssets()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			built, archive, err := packaging.Pack(testCase.declaration, testCase.assets)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}

			// The loader is what a system component runs before it takes the
			// artifact: structure, digest, and declaration are checked there.
			loaded, err := packaging.Verify(archive, built.Digest)
			if err != nil {
				t.Fatalf("load package: %v", err)
			}
			if loaded.Manifest.ID != testCase.declaration["id"] {
				t.Fatalf("loaded id = %q, want %v", loaded.Manifest.ID, testCase.declaration["id"])
			}
			if len(loaded.Assets) != len(testCase.assets) {
				t.Fatalf("loaded %d assets, want %d", len(loaded.Assets), len(testCase.assets))
			}
			if !slices.Equal(assetPaths(loaded.Assets), assetPaths(testCase.assets)) {
				t.Fatalf("loaded assets = %v, want %v", assetPaths(loaded.Assets), assetPaths(testCase.assets))
			}
		})
	}
}

func TestPackagedManifestCarriesExactlyTheContractFields(t *testing.T) {
	cases := []struct {
		name        string
		declaration map[string]any
		assets      []packaging.Asset
	}{
		{name: "minimal page extension", declaration: fixtures.MinimalPageExtension(), assets: pageAssets()},
		{name: "minimal window application", declaration: fixtures.MinimalWindowApp(), assets: windowAssets()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, archive, err := packaging.Pack(testCase.declaration, testCase.assets)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}
			entries, err := fixtures.ReadArchive(archive)
			if err != nil {
				t.Fatalf("read archive: %v", err)
			}
			manifestEntry, ok := fixtures.Find(entries, packaging.ManifestEntry)
			if !ok {
				t.Fatal("archive carries no manifest")
			}

			produced := decode(t, manifestEntry.Content)
			if got := keys(produced); !slices.Equal(got, sortedCopy(manifest.ExtensionManifestFields)) {
				t.Fatalf("extension field set = %v, want %v", got, manifest.ExtensionManifestFields)
			}

			contributions, ok := produced["contributions"].(map[string]any)
			if !ok {
				t.Fatalf("contributions is not an object: %T", produced["contributions"])
			}
			for _, key := range keys(contributions) {
				if !slices.Contains(manifest.ContributionClasses, key) {
					t.Fatalf("contributions carries an undefined class %q", key)
				}
			}

			apps, ok := contributions["apps"].([]any)
			if !ok {
				return
			}
			requiredAppFields := []string{"id", "name", "version", "icon", "entry", "window", "permissions"}
			for index, app := range apps {
				record, ok := app.(map[string]any)
				if !ok {
					t.Fatalf("apps[%d] is not an object: %T", index, app)
				}
				for _, field := range requiredAppFields {
					if _, ok := record[field]; !ok {
						t.Fatalf("apps[%d] is missing the required field %q", index, field)
					}
				}
				for _, field := range keys(record) {
					if !slices.Contains(manifest.AppManifestFields, field) {
						t.Fatalf("apps[%d] carries a field outside the contract: %q", index, field)
					}
				}
				window, ok := record["window"].(map[string]any)
				if !ok {
					t.Fatalf("apps[%d].window is not an object: %T", index, record["window"])
				}
				if got := keys(window); !slices.Equal(got, sortedCopy(manifest.AppWindowFields)) {
					t.Fatalf("window field set = %v, want %v", got, manifest.AppWindowFields)
				}
			}
		})
	}
}

func TestDigestIsVerifiable(t *testing.T) {
	built, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), windowAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if len(built.Digest) != len("sha256:")+64 || built.Digest[:len("sha256:")] != "sha256:" {
		t.Fatalf("digest %q is not a sha256 digest", built.Digest)
	}

	entries, err := fixtures.ReadArchive(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	digestEntry, ok := fixtures.Find(entries, packaging.DigestEntry)
	if !ok {
		t.Fatal("archive carries no digest entry")
	}
	if string(digestEntry.Content) != string(built.Digest)+"\n" {
		t.Fatalf("digest entry = %q, want %q", digestEntry.Content, built.Digest)
	}
	if _, err := packaging.Verify(archive, built.Digest); err != nil {
		t.Fatalf("verify digest: %v", err)
	}
}

func assetPaths(assets []packaging.Asset) []string {
	paths := make([]string, 0, len(assets))
	for _, asset := range assets {
		paths = append(paths, asset.Path)
	}
	sort.Strings(paths)
	return paths
}
