// Package sdktc8 verifies that reading a received package is bounded: a
// complete, valid package that only exceeds the entry or byte budget is refused
// as unreadable, the refusal happens at the bound instead of after the content
// is read, and a package inside the budget still passes (SDK-AC-7, SDK-TC-8,
// P1).
package sdktc8

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tsic404/kos-sdk/packaging"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

func systemAssets() []packaging.Asset {
	return []packaging.Asset{
		{Path: "assets/icon.svg", Content: []byte("<svg><!-- files --></svg>")},
		{Path: "assets/app.html", Content: []byte("<!doctype html><title>Files</title>")},
	}
}

func packWith(t *testing.T, extra ...packaging.Asset) (packaging.Package, []byte) {
	t.Helper()
	assets := append(systemAssets(), extra...)
	built, archive, err := packaging.Pack(fixtures.SystemApp(), assets)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	return built, archive
}

func TestPackageOverTheByteBudgetIsRefused(t *testing.T) {
	built, archive := packWith(t, packaging.Asset{
		Path:    "assets/bundle.bin",
		Content: make([]byte, 4<<20),
	})

	limits := packaging.Limits{Entries: 64, UncompressedBytes: 1 << 20}
	refused, err := packaging.VerifyWithLimits(archive, built.Digest, limits)
	if !errors.Is(err, packaging.ErrMalformedPackage) {
		t.Fatalf("verify over the byte budget = %v, want ErrMalformedPackage", err)
	}
	if !strings.Contains(err.Error(), "1048576") {
		t.Fatalf("the refusal does not point at the byte budget: %v", err)
	}
	if errors.Is(err, packaging.ErrDigestMismatch) {
		t.Fatalf("the package was reported as a digest mismatch instead of unreadable: %v", err)
	}
	if refused.Digest != "" || refused.Manifest.Identifier != "" || len(refused.Assets) != 0 {
		t.Fatal("a refused package returned partial content")
	}

	// The same artifact is a valid package: only the budget separates it from
	// acceptance, so removing the budget logic would make this case green.
	if _, err := packaging.Verify(archive, built.Digest); err != nil {
		t.Fatalf("the package is not valid at the default budget: %v", err)
	}
}

func TestPackageOverTheEntryBudgetIsRefused(t *testing.T) {
	extra := make([]packaging.Asset, 0, 7)
	for index := range 7 {
		extra = append(extra, packaging.Asset{
			Path:    fmt.Sprintf("assets/file-%d", index),
			Content: []byte("x"),
		})
	}
	built, archive := packWith(t, extra...)

	limits := packaging.Limits{Entries: 8, UncompressedBytes: 16 << 20}
	if _, err := packaging.VerifyWithLimits(archive, built.Digest, limits); !errors.Is(err, packaging.ErrMalformedPackage) {
		t.Fatalf("verify over the entry budget = %v, want ErrMalformedPackage", err)
	} else if !strings.Contains(err.Error(), "8 entries") {
		t.Fatalf("the refusal does not point at the entry budget: %v", err)
	}

	if _, err := packaging.Verify(archive, built.Digest); err != nil {
		t.Fatalf("the package is not valid at the default budget: %v", err)
	}
}

func TestRefusalHappensAtTheBoundNotAfterTheRead(t *testing.T) {
	built, archive := packWith(t, packaging.Asset{
		Path:    "assets/bundle.bin",
		Content: make([]byte, 4<<20),
	})

	// Cut the stream inside the over-budget entry: a reader that materializes
	// the entry fails with a short read, while a reader that stops at the bound
	// reports the budget before touching it.
	truncated := truncateEntry(t, archive, "assets/bundle.bin", 1<<10)
	limits := packaging.Limits{Entries: 64, UncompressedBytes: 1 << 20}

	_, err := packaging.VerifyWithLimits(truncated, built.Digest, limits)
	if !errors.Is(err, packaging.ErrMalformedPackage) {
		t.Fatalf("verify a truncated over-budget package = %v, want ErrMalformedPackage", err)
	}
	if !strings.Contains(err.Error(), "1048576") {
		t.Fatalf("the refusal did not happen at the bound: %v", err)
	}
}

func TestPackageInsideTheBudgetStillPasses(t *testing.T) {
	built, archive := packWith(t)

	loaded, err := packaging.VerifyWithLimits(archive, built.Digest, packaging.Limits{
		Entries:           64,
		UncompressedBytes: 16 << 20,
	})
	if err != nil {
		t.Fatalf("verify a package inside the budget: %v", err)
	}
	if loaded.Manifest.Identifier != built.Manifest.Identifier || len(loaded.Assets) != len(built.Assets) {
		t.Fatalf("loaded package = %+v, want %+v", loaded, built)
	}
}

// truncateEntry rewrites an archive so the named entry keeps its declared size
// while the stream ends after keep bytes of its content.
func truncateEntry(t *testing.T, archive []byte, name string, keep int) []byte {
	t.Helper()
	entries, err := fixtures.ReadArchive(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}

	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     entry.Name,
			Mode:     0o644,
			Size:     int64(len(entry.Content)),
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write header for %q: %v", entry.Name, err)
		}
		if entry.Name == name {
			if _, err := writer.Write(entry.Content[:keep]); err != nil {
				t.Fatalf("write short entry %q: %v", entry.Name, err)
			}
			break
		}
		if _, err := writer.Write(entry.Content); err != nil {
			t.Fatalf("write entry %q: %v", entry.Name, err)
		}
	}
	// The tar writer is deliberately left open: the stream must end inside the
	// truncated entry.
	if err := compressed.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}
