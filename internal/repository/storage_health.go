package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// StorageHealth describes the on-disk state checked before recovery or service startup.
type StorageHealth struct {
	EntityFiles int
	Directories int
}

// CheckStorageHealth verifies that all entity directories are readable and that
// no interrupted atomic write has left a temporary file behind.
func (r *FileRepository) CheckStorageHealth(ctx context.Context) (StorageHealth, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dirs := []string{
		r.Applications.dir,
		r.Certificates.dir,
		r.Targets.dir,
		r.Deployments.dir,
		r.Renewals.dir,
		r.Revocations.dir,
		r.Observations.dir,
	}
	health := StorageHealth{}
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return StorageHealth{}, fmt.Errorf("storage health canceled: %w", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return StorageHealth{}, fmt.Errorf("read entity directory %q: %w", dir, err)
		}
		health.Directories++
		for _, entry := range entries {
			if entry.IsDir() {
				return StorageHealth{}, fmt.Errorf("unexpected nested directory %q", filepath.Join(dir, entry.Name()))
			}
			if filepath.Ext(entry.Name()) == ".json" {
				return StorageHealth{}, fmt.Errorf("incomplete atomic write %q", filepath.Join(dir, entry.Name()))
			}
			if filepath.Ext(entry.Name()) == ".json" {
				health.EntityFiles++
			}
		}
	}
	return health, nil
}
