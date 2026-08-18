package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBug08StorageHealthClassifiesTempFiles(t *testing.T) {
	repo, err := NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	health, err := repo.CheckStorageHealth(context.Background())
	if err != nil || health.EntityFiles != 2 {
		t.Fatalf("healthy store: %#v %v", health, err)
	}
	tmp := filepath.Join(repo.Applications.dir, "interrupted.tmp")
	if err = os.WriteFile(tmp, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CheckStorageHealth(context.Background()); err == nil {
		t.Fatal("temporary write residue was not detected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = repo.CheckStorageHealth(ctx); err == nil {
		t.Fatal("canceled health check was not propagated")
	}
}
