// Package blobstore stores uploaded files on disk, addressed by the
// sha256 hash of their content. Content addressing gives asset reuse
// for free: uploading identical bytes always resolves to the same
// path, whether that's the same room reusing a token image or a
// completely different room months later.
package blobstore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	dir string
}

// New ensures dir exists and returns a Store rooted there.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// path returns where a blob with the given content hash lives,
// sharded by the first two hex characters so the directory doesn't
// accumulate flat thousands of files.
func (s *Store) path(hash string) (string, error) {
	if len(hash) < 2 {
		return "", fmt.Errorf("invalid content hash %q", hash)
	}
	return filepath.Join(s.dir, hash[:2], hash), nil
}

// Write saves src under hash, creating parent directories as needed.
// If a blob with this hash already exists, it is left untouched (the
// content is by definition identical).
func (s *Store) Write(hash string, src io.Reader) error {
	path, err := s.path(hash)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil // already stored
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create blob directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("write blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close blob: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		// Somebody stored the same bytes between the Stat above and here.
		// POSIX rename would have replaced their file with our identical
		// one and said nothing; Windows refuses with "Access is denied",
		// which turned two people uploading the same picture at the same
		// moment into a 500 for whichever lost. The path *is* the content's
		// hash, so a file already sitting there is byte-for-byte what we
		// were about to write.
		if _, statErr := os.Stat(path); statErr == nil {
			return nil
		}
		return fmt.Errorf("store blob: %w", err)
	}
	return nil
}

// Open returns a reader for the blob with the given content hash.
func (s *Store) Open(hash string) (*os.File, error) {
	path, err := s.path(hash)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
