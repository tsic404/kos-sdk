package fixtures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"time"
)

// Entry is one file of a package archive, as an acceptance case observes it.
type Entry struct {
	Name    string
	Mode    int64
	UID     int
	GID     int
	Type    byte
	ModTime time.Time
	Content []byte
}

// ReadArchive returns the entries of a package archive in archive order.
func ReadArchive(archive []byte) ([]Entry, error) {
	decompressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open package archive: %w", err)
	}
	defer decompressed.Close()

	var entries []Entry
	reader := tar.NewReader(decompressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read package archive: %w", err)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read entry %q: %w", header.Name, err)
		}
		entries = append(entries, Entry{
			Name:    header.Name,
			Mode:    header.Mode,
			UID:     header.Uid,
			GID:     header.Gid,
			Type:    header.Typeflag,
			ModTime: header.ModTime,
			Content: content,
		})
	}
	return entries, nil
}

// Find returns the entry with the given name.
func Find(entries []Entry, name string) (Entry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return Entry{}, false
}

// WriteArchive builds an archive from entries, for cases that must present
// content the packer would never produce.
func WriteArchive(entries []Entry) ([]byte, error) {
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
			return nil, fmt.Errorf("write header for %q: %w", entry.Name, err)
		}
		if _, err := writer.Write(entry.Content); err != nil {
			return nil, fmt.Errorf("write entry %q: %w", entry.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := compressed.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}
	return buffer.Bytes(), nil
}
