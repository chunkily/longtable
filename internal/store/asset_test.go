package store

import (
	"errors"
	"strings"
	"testing"
)

func TestCreateAsset_FindByHash(t *testing.T) {
	s := newTestStore(t)

	asset, err := s.CreateAsset("abc123", "map.png", "image/png", 1024)
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	got, err := s.FindAssetByHash("abc123")
	if err != nil {
		t.Fatalf("FindAssetByHash: %v", err)
	}
	if got.ID != asset.ID {
		t.Fatalf("resolved asset %q, want %q", got.ID, asset.ID)
	}

	got2, err := s.GetAsset(asset.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got2.ContentHash != "abc123" {
		t.Fatalf("ContentHash = %q, want abc123", got2.ContentHash)
	}
}

func TestFindAssetByHash_NotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.FindAssetByHash("nosuch"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateAsset_DuplicateHashRejected(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateAsset("dup", "a.png", "image/png", 1); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	_, err := s.CreateAsset("dup", "b.png", "image/png", 2)
	if err == nil {
		t.Fatal("expected an error inserting a duplicate content hash")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Fatalf("err = %v, want a unique constraint error", err)
	}
}
