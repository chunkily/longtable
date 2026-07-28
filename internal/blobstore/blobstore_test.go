package blobstore

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWriteAndOpen(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content := []byte("hello, blob")
	if err := s.Write("abcd1234", bytes.NewReader(content)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	f, err := s.Open("abcd1234")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content = %q, want %q", got, content)
	}
}

func TestWrite_ExistingHashIsNoOp(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := s.Write("hash1", strings.NewReader("original")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Writing the same hash again with different bytes must not
	// overwrite the stored blob (content-addressed storage assumes the
	// same hash always means the same content).
	if err := s.Write("hash1", strings.NewReader("different")); err != nil {
		t.Fatalf("Write (again): %v", err)
	}

	f, err := s.Open("hash1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("content = %q, want original content preserved", got)
	}
}

func TestOpen_MissingBlob(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := s.Open("nosuchhash"); err == nil {
		t.Fatal("expected an error opening a blob that was never written")
	}
}

func TestWrite_RejectsShortHash(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := s.Write("a", strings.NewReader("x")); err == nil {
		t.Fatal("expected an error for a hash shorter than 2 characters")
	}
}
