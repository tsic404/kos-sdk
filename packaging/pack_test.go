package packaging_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/tsic404/kos-sdk/packaging"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
	"github.com/tsic404/kos-sdk/validate"
)

func systemAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/icon.svg", Content: []byte("<svg></svg>")},
		{Path: "assets/app.html", Content: []byte("<!doctype html>")},
	}
}

func hasIssue(issues []validate.Issue, path string, code validate.Code) bool {
	for _, issue := range issues {
		if issue.Path == path && issue.Code == code {
			return true
		}
	}
	return false
}

func TestPackRejectsInvalidDeclaration(t *testing.T) {
	declaration := fixtures.MinimalPageExtension()
	delete(declaration, "identifier")

	pkg, archive, err := packaging.Pack(declaration, nil)
	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("invalid declaration error = %v, want a rejection", err)
	}
	if archive != nil || pkg.Digest != "" {
		t.Fatal("a rejected declaration produced an artifact")
	}
	if !hasIssue(invalid.Issues, "$.identifier", validate.CodeMissingField) {
		t.Fatalf("expected a missing-field issue for $.identifier, got %v", invalid.Issues)
	}
}

func TestPackRejectsUnusableAssetPaths(t *testing.T) {
	cases := []struct {
		name   string
		assets []packaging.Asset
		index  int
	}{
		{name: "outside the asset namespace", assets: []packaging.Asset{{Path: "icon.svg"}}, index: 0},
		{name: "parent traversal", assets: []packaging.Asset{{Path: "assets/../icon.svg"}}, index: 0},
		{name: "namespace root only", assets: []packaging.Asset{{Path: "assets/"}}, index: 0},
		{
			name: "declared twice",
			assets: []packaging.Asset{
				{Path: "assets/notes.js", Content: []byte("v1")},
				{Path: "assets/notes.js", Content: []byte("v2")},
			},
			index: 1,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, archive, err := packaging.Pack(fixtures.ThirdPartyExtension(), testCase.assets)
			var invalid *validate.InvalidError
			if !errors.As(err, &invalid) {
				t.Fatalf("asset set error = %v, want a rejection", err)
			}
			if archive != nil {
				t.Fatal("a rejected asset set produced an artifact")
			}
			wantPath := fmt.Sprintf("$.assets[%d].path", testCase.index)
			if !hasIssue(invalid.Issues, wantPath, validate.CodeInvalidValue) {
				t.Fatalf("expected an invalid-value issue for %s, got %v", wantPath, invalid.Issues)
			}
		})
	}
}

func TestPackRejectsIconAssetThePackageDoesNotCarry(t *testing.T) {
	_, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), []packaging.Asset{
		{Path: "assets/app.html", Content: []byte("<!doctype html>")},
	})
	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("missing icon asset error = %v, want a rejection", err)
	}
	if archive != nil {
		t.Fatal("a missing icon asset produced an artifact")
	}
	if !hasIssue(invalid.Issues, "$.contributions.apps[0].icon", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for the icon, got %v", invalid.Issues)
	}
}

func TestPackIsDeterministic(t *testing.T) {
	declaration := fixtures.ThirdPartyExtension()
	first := []packaging.Asset{
		{Path: "assets/index.html", Content: []byte("<html></html>")},
		{Path: "assets/notes.js", Content: []byte("export {}")},
	}
	second := []packaging.Asset{first[1], first[0]}

	firstPackage, firstArchive, err := packaging.Pack(declaration, first)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	secondPackage, secondArchive, err := packaging.Pack(declaration, second)
	if err != nil {
		t.Fatalf("pack reordered assets: %v", err)
	}
	if firstPackage.Digest != secondPackage.Digest {
		t.Fatalf("digest depends on asset order: %s vs %s", firstPackage.Digest, secondPackage.Digest)
	}
	if !bytes.Equal(firstArchive, secondArchive) {
		t.Fatal("the same declaration and assets produced different archive bytes")
	}
}

func TestDigestCoversManifestAndEveryAsset(t *testing.T) {
	declaration := fixtures.SystemApp()
	base, _, err := packaging.Pack(declaration, systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	changedAsset := systemAssets()
	changedAsset[1].Content = []byte("<!doctype html><title>Files</title>")
	if pkg, _, err := packaging.Pack(declaration, changedAsset); err != nil {
		t.Fatalf("pack changed asset: %v", err)
	} else if pkg.Digest == base.Digest {
		t.Fatal("changing an asset left the digest unchanged")
	}

	changedDeclaration := fixtures.SystemApp()
	changedDeclaration["version"] = "1.0.1"
	if pkg, _, err := packaging.Pack(changedDeclaration, systemAssets()); err != nil {
		t.Fatalf("pack changed declaration: %v", err)
	} else if pkg.Digest == base.Digest {
		t.Fatal("changing the manifest left the digest unchanged")
	}

	moreAssets := append(systemAssets(), packaging.Asset{Path: "assets/extra.css", Content: []byte("body{}")})
	if pkg, _, err := packaging.Pack(declaration, moreAssets); err != nil {
		t.Fatalf("pack added asset: %v", err)
	} else if pkg.Digest == base.Digest {
		t.Fatal("adding an asset left the digest unchanged")
	}
}

func TestVerifyAcceptsTheBuiltPackage(t *testing.T) {
	built, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	loaded, err := packaging.Verify(archive, built.Digest)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if loaded.Digest != built.Digest {
		t.Fatalf("digest = %s, want %s", loaded.Digest, built.Digest)
	}
	if loaded.Manifest.Identifier != built.Manifest.Identifier || len(loaded.Assets) != len(built.Assets) {
		t.Fatalf("loaded package = %+v, want %+v", loaded, built)
	}
}

func TestVerifyRejectsAnotherDigest(t *testing.T) {
	_, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	unrelated := packaging.Digest("sha256:" + strings.Repeat("0", 64))
	if _, err := packaging.Verify(archive, unrelated); !errors.Is(err, packaging.ErrDigestMismatch) {
		t.Fatalf("verify against another digest = %v, want ErrDigestMismatch", err)
	}
}

func TestVerifyRejectsTamperedContent(t *testing.T) {
	_, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	tampered := rewriteArchive(t, archive, func(entries []fixtures.Entry) []fixtures.Entry {
		for index, entry := range entries {
			if entry.Name == "assets/app.html" {
				entries[index].Content = []byte("<script>alert(1)</script>")
			}
		}
		return entries
	})
	if _, err := packaging.Verify(tampered, ""); !errors.Is(err, packaging.ErrDigestMismatch) {
		t.Fatalf("verify tampered package = %v, want ErrDigestMismatch", err)
	}
}

func TestVerifyRejectsUndeclaredEntry(t *testing.T) {
	_, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	extra := rewriteArchive(t, archive, func(entries []fixtures.Entry) []fixtures.Entry {
		return append(entries, fixtures.Entry{Name: "notes.txt", Content: []byte("undeclared")})
	})
	if _, err := packaging.Verify(extra, ""); !errors.Is(err, packaging.ErrUndeclaredEntry) {
		t.Fatalf("verify package with an undeclared entry = %v, want ErrUndeclaredEntry", err)
	}
}

func TestVerifyAcceptsThePromisedDigestInAnyCase(t *testing.T) {
	built, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	upper := packaging.Digest(strings.ToUpper(string(built.Digest)))
	if _, err := packaging.Verify(archive, upper); err != nil {
		t.Fatalf("verify with an uppercase promised digest: %v", err)
	}
}

func TestVerifyRejectsMalformedPromisedDigest(t *testing.T) {
	_, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), systemAssets())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	if _, err := packaging.Verify(archive, "sha256:not-a-digest"); !errors.Is(err, packaging.ErrInvalidDigest) {
		t.Fatalf("verify with a malformed promised digest = %v, want ErrInvalidDigest", err)
	}
}

func packageWithAssets(t *testing.T, extra ...packaging.Asset) (packaging.Package, []byte) {
	t.Helper()
	assets := append(systemAssets(), extra...)
	built, archive, err := packaging.Pack(fixtures.MinimalWindowApp(), assets)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	return built, archive
}

func TestVerifyRefusesAValidPackageOverTheByteBudget(t *testing.T) {
	built, archive := packageWithAssets(t, packaging.Asset{
		Path:    "assets/bundle.bin",
		Content: make([]byte, 4<<20),
	})

	limits := packaging.Limits{Entries: 64, UncompressedBytes: 1 << 20}
	pkg, err := packaging.VerifyWithLimits(archive, built.Digest, limits)
	if !errors.Is(err, packaging.ErrMalformedPackage) {
		t.Fatalf("verify over the byte budget = %v, want ErrMalformedPackage", err)
	}
	if !strings.Contains(err.Error(), "1048576") {
		t.Fatalf("the refusal does not name the byte budget: %v", err)
	}
	if pkg.Digest != "" || pkg.Manifest.Identifier != "" || len(pkg.Assets) != 0 {
		t.Fatal("a refused package returned partial content")
	}

	// Only the budget stands between this artifact and acceptance, so removing
	// the budget logic would make this test red.
	if _, err := packaging.Verify(archive, built.Digest); err != nil {
		t.Fatalf("the package is not valid at the default budget: %v", err)
	}
}

func TestVerifyRefusesAValidPackageOverTheEntryBudget(t *testing.T) {
	extra := make([]packaging.Asset, 0, 7)
	for index := range 7 {
		extra = append(extra, packaging.Asset{
			Path:    fmt.Sprintf("assets/file-%d", index),
			Content: []byte("x"),
		})
	}
	built, archive := packageWithAssets(t, extra...)

	limits := packaging.Limits{Entries: 8, UncompressedBytes: 16 << 20}
	pkg, err := packaging.VerifyWithLimits(archive, built.Digest, limits)
	if !errors.Is(err, packaging.ErrMalformedPackage) {
		t.Fatalf("verify over the entry budget = %v, want ErrMalformedPackage", err)
	}
	if !strings.Contains(err.Error(), "8 entries") {
		t.Fatalf("the refusal does not name the entry budget: %v", err)
	}
	if pkg.Digest != "" || pkg.Manifest.Identifier != "" || len(pkg.Assets) != 0 {
		t.Fatal("a refused package returned partial content")
	}

	if _, err := packaging.Verify(archive, built.Digest); err != nil {
		t.Fatalf("the package is not valid at the default budget: %v", err)
	}
}

func TestVerifyAcceptsAValidPackageUnderAnUnboundedBudget(t *testing.T) {
	built, archive := packageWithAssets(t)

	limits := packaging.Limits{Entries: math.MaxInt32, UncompressedBytes: math.MaxInt64}
	if _, err := packaging.VerifyWithLimits(archive, built.Digest, limits); err != nil {
		t.Fatalf("verify under an unbounded budget: %v", err)
	}
}

// rewriteArchive rebuilds an archive from its entries so a case can present the
// install path with content the packer would never produce.
func rewriteArchive(t *testing.T, archive []byte, mutate func([]fixtures.Entry) []fixtures.Entry) []byte {
	t.Helper()
	entries, err := fixtures.ReadArchive(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	rebuilt, err := fixtures.WriteArchive(mutate(entries))
	if err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return rebuilt
}
