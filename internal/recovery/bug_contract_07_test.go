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
	"testing"

	"certarchive/internal/domain"
	"certarchive/internal/journal"
)

func TestBug07ValidJournalAndSnapshotRecover(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	if err = j.Append(domain.Event{Type: "issued", EntityID: "cert"}); err != nil {
		t.Fatal(err)
	}
	events, err := j.Recover()
	if err != nil || len(events) != 1 {
		t.Fatalf("journal recovery: %#v %v", events, err)
	}
	data := filepath.Join(dir, "data")
	for _, sub := range []string{"applications", "certificates", "targets", "deployments", "renewals", "revocations", "observations"} {
		if err = os.MkdirAll(filepath.Join(data, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(data, "applications", "a.json"), []byte(`{"ID":"a"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecoveryManager(data, j)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.TakeSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	snaps, err := r.listSnapshots()
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots: %#v %v", snaps, err)
	}
	if err = r.restoreSnapshot(snaps[0]); err != nil {
		t.Fatalf("restore valid snapshot: %v", err)
	}
}

// TestBug07CorruptedDataRejected 验证损坏的日志记录与快照仍被拒绝，而已恢复的
// 合法状态、查询数据与磁盘持久化数据保持一致（不留部分更新）。
func TestBug07CorruptedDataRejected(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	// 写入一条合法记录和一条被损坏的记录，恢复时应只得到合法的那一条。
	if err = j.Append(domain.Event{Type: "issued", EntityID: "good"}); err != nil {
		t.Fatal(err)
	}
	// 直接写入一条校验和不匹配的损坏记录。
	corruptRecord := fmt.Sprintf("%d:%s:%x\n", len(`{"Type":"issued","EntityID":"bad"}`), `{"Type":"issued","EntityID":"bad"}`, 0xff)
	if err = os.WriteFile(filepath.Join(dir, "journal", "corrupted.log"), []byte(corruptRecord), 0644); err != nil {
		t.Fatal(err)
	}
	events, err := j.Recover()
	if err != nil {
		t.Fatalf("journal recover error: %v", err)
	}
	if len(events) != 1 || events[0].EntityID != "good" {
		t.Fatalf("expected only the valid event to be recovered, got %#v", events)
	}

	// 准备数据目录并生成一个有效快照。
	data := filepath.Join(dir, "data")
	for _, sub := range []string{"applications", "certificates", "targets", "deployments", "renewals", "revocations", "observations"} {
		if err = os.MkdirAll(filepath.Join(data, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(data, "applications", "a.json"), []byte(`{"ID":"a"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecoveryManager(data, j)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.TakeSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	snaps, err := r.listSnapshots()
	if err != nil || len(snaps) != 1 {
		t.Fatalf("snapshots: %#v %v", snaps, err)
	}

	// 篡改快照中的实体内容但不更新 checksum，模拟磁盘损坏。
	snapPath := filepath.Join(r.snapshotDir, snaps[0])
	raw, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatal(err)
	}
	var snap struct {
		Metadata domain.SnapshotMetadata `json:"metadata"`
		Entities map[string]string       `json:"entities"`
	}
	if err = json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	for k := range snap.Entities {
		snap.Entities[k] = `{"ID":"tampered"}`
	}
	tampered, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(snapPath, tampered, 0644); err != nil {
		t.Fatal(err)
	}

	// 损坏快照必须被拒绝。
	if err = r.restoreSnapshot(snaps[0]); err == nil {
		t.Fatal("expected corrupted snapshot to be rejected, got nil error")
	}

	// 确认合法的实体文件未被损坏快照覆盖（无部分更新）。
	got, err := os.ReadFile(filepath.Join(data, "applications", "a.json"))
	if err != nil {
		t.Fatalf("original entity file missing after failed restore: %v", err)
	}
	if string(got) != `{"ID":"a"}` {
		t.Fatalf("original entity altered by rejected restore: %s", got)
	}
}

// TestSnapshotRestoreIsAtomic 验证当写入中途失败时，已落盘的实体保持完整，
// 不会留下“删了一半没写回去”的部分更新状态。
func TestSnapshotRestoreIsAtomic(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(dir, "data")
	for _, sub := range []string{"applications", "certificates"} {
		if err = os.MkdirAll(filepath.Join(data, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// 既有数据：applications/keep.json（不在即将构造的快照中，属于应被清理的旧文件）。
	if err = os.WriteFile(filepath.Join(data, "applications", "keep.json"), []byte(`{"ID":"keep"}`), 0644); err != nil {
		t.Fatal(err)
	}
	// 在 applications 下放置一个同名普通文件 zzz，使快照中 applications/zzz/inner.json
	// 在 MkdirAll 阶段可靠失败（路径组件不是目录），用于模拟写入中途出错。
	if err = os.WriteFile(filepath.Join(data, "applications", "zzz"), []byte(`not-a-dir`), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := NewRecoveryManager(data, j)
	if err != nil {
		t.Fatal(err)
	}

	// 构造一个校验和正确的快照：先写 aaa.json（会成功），再写 zzz/inner.json（会失败）。
	h := sha256.New()
	entities := map[string]string{
		"applications/aaa.json":       `{"ID":"aaa"}`,
		"applications/zzz/inner.json": `{"ID":"inner"}`,
	}
	keys := make([]string, 0, len(entities))
	for k := range entities {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(entities[k]))
	}
	snap := struct {
		Metadata domain.SnapshotMetadata `json:"metadata"`
		Entities map[string]string       `json:"entities"`
	}{
		Metadata: domain.SnapshotMetadata{Version: 1, Checksum: hex.EncodeToString(h.Sum(nil))},
		Entities: entities,
	}
	snapData, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(r.snapshotDir, "snap-1.json"), snapData, 0644); err != nil {
		t.Fatal(err)
	}

	// 写入阶段失败，应返回错误，且不应删除既有的 keep.json（无部分更新）。
	if err = r.restoreSnapshot("snap-1.json"); err == nil {
		t.Fatal("expected restore to fail on bad entity path, got nil")
	}
	if _, err = os.Stat(filepath.Join(data, "applications", "keep.json")); err != nil {
		t.Fatalf("pre-existing entity removed before all writes succeeded (partial update): %v", err)
	}
}
