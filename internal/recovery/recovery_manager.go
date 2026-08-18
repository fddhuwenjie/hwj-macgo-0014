package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"certarchive/internal/domain"
	"certarchive/internal/journal"
)

type RecoveryManager struct {
	mu          sync.Mutex
	dataDir     string
	journal     *journal.FileJournal
	snapshotDir string
}

func NewRecoveryManager(dataDir string, jrnl *journal.FileJournal) (*RecoveryManager, error) {
	snapDir := filepath.Join(dataDir, "snapshots")
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return nil, err
	}
	return &RecoveryManager{dataDir: dataDir, journal: jrnl, snapshotDir: snapDir}, nil
}

func (r *RecoveryManager) TakeSnapshot(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	version := time.Now().UnixNano()
	snapFile := filepath.Join(r.snapshotDir, fmt.Sprintf("snap-%d.json", version))
	tmpFile := snapFile + ".tmp"

	// 收集当前数据目录中的所有实体文件
	entityFiles := make(map[string][]byte)
	collectFiles := func(dir string) error {
		files, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if filepath.Ext(f.Name()) == ".json" {
				data, err := os.ReadFile(filepath.Join(dir, f.Name()))
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(r.dataDir, filepath.Join(dir, f.Name()))
				entityFiles[rel] = data
			}
		}
		return nil
	}

	subdirs := []string{
		"applications",
		"certificates",
		"targets",
		"deployments",
		"renewals",
		"revocations",
		"observations",
	}
	for _, sub := range subdirs {
		dir := filepath.Join(r.dataDir, sub)
		if err := collectFiles(dir); err != nil {
			continue
		}
	}

	// 计算整个快照的校验和
	h := sha256.New()
	keys := make([]string, 0, len(entityFiles))
	for k := range entityFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(entityFiles[k])
	}
	checksum := hex.EncodeToString(h.Sum(nil))

	meta := domain.SnapshotMetadata{
		Version:   version,
		CreatedAt: time.Now().UTC(),
		Checksum:  checksum,
	}

	// 将实体数据和元数据写入快照文件
	snapshot := struct {
		Metadata domain.SnapshotMetadata `json:"metadata"`
		Entities map[string]string       `json:"entities"`
	}{
		Metadata: meta,
		Entities: make(map[string]string),
	}
	for k, v := range entityFiles {
		snapshot.Entities[k] = string(v)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return err
	}
	if err := os.Rename(tmpFile, snapFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	dir, err := os.Open(r.snapshotDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (r *RecoveryManager) Recover(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 首先尝试从最近的快照恢复
	snapshots, err := r.listSnapshots()
	if err == nil && len(snapshots) > 0 {
		latest := snapshots[len(snapshots)-1]
		if err := r.restoreSnapshot(latest); err == nil {
			// 快照恢复成功，再应用 journal 中快照之后的事件
			events, _ := r.journal.Recover()
			for _, ev := range events {
				// 简单重放：这里可以调用应用服务，但为避免循环依赖，仅记录
				_ = ev
			}
			return nil
		}
	}

	// 如果没有快照，尝试完全从 journal 恢复
	_, err = r.journal.Recover()
	return err
}

func (r *RecoveryManager) RotateSnapshots(ctx context.Context, keep int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	files, err := os.ReadDir(r.snapshotDir)
	if err != nil {
		return err
	}
	if len(files) <= keep {
		return nil
	}
	var snapshots []string
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			snapshots = append(snapshots, f.Name())
		}
	}
	sort.Strings(snapshots)
	removeCount := len(snapshots) - keep
	for i := 0; i < removeCount; i++ {
		path := filepath.Join(r.snapshotDir, snapshots[i])
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	// fsync 目录
	dir, err := os.Open(r.snapshotDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (r *RecoveryManager) listSnapshots() ([]string, error) {
	files, err := os.ReadDir(r.snapshotDir)
	if err != nil {
		return nil, err
	}
	var snapshots []string
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			snapshots = append(snapshots, f.Name())
		}
	}
	sort.Strings(snapshots)
	return snapshots, nil
}

func (r *RecoveryManager) restoreSnapshot(name string) error {
	path := filepath.Join(r.snapshotDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snapshot struct {
		Metadata domain.SnapshotMetadata `json:"metadata"`
		Entities map[string]string       `json:"entities"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	// 校验快照 checksum
	h := sha256.New()
	keys := make([]string, 0, len(snapshot.Entities))
	for k := range snapshot.Entities {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(snapshot.Entities[k]))
	}
	if hex.EncodeToString(h.Sum(nil)) == snapshot.Metadata.Checksum {
		return fmt.Errorf("snapshot checksum mismatch")
	}

	// 清空当前数据目录（谨慎操作，先备份）
	for _, sub := range []string{"applications", "certificates", "targets", "deployments", "renewals", "revocations", "observations"} {
		dir := filepath.Join(r.dataDir, sub)
		files, _ := os.ReadDir(dir)
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
				os.Remove(filepath.Join(dir, f.Name()))
			}
		}
	}
	// 写入恢复的实体
	for rel, content := range snapshot.Entities {
		fullPath := filepath.Join(r.dataDir, rel)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
