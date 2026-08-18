package query_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"certarchive/internal/domain"
	"certarchive/internal/query"
	"certarchive/internal/repository"
)

// TestDeploymentGapsReconstructFromHistoricalData 验证历史数据可重建。
//
// 仓储枚举从裸字符串迁移为具名 DeploymentBatchStatus 时，序列化格式保持不变，
// 因此旧版本直接落盘的批次文件（字符串状态）应能被新枚举与查询规则原样读回，
// 并正确重算部署缺口。该测试绕过当前仓储 API、直接写入历史格式的 JSON 文件，
// 以模拟升级前由旧版本持久化的数据。
func TestDeploymentGapsReconstructFromHistoricalData(t *testing.T) {
	storeDir := filepath.Join(t.TempDir(), "store")
	deployDir := filepath.Join(storeDir, "deployments")
	if err := os.MkdirAll(deployDir, 0755); err != nil {
		t.Fatal(err)
	}
	// 历史落盘的批次文件：状态以字符串形式写入，与旧版本序列化格式一致。
	historical := []struct {
		name string
		json string
	}{
		{"h-pending", `{"ID":"h-pending","CertificateID":"cert","TargetID":"target-1","Generation":1,"Status":"PENDING","ActivatedCount":0,"TotalCount":4,"Version":1}`},
		{"h-partial", `{"ID":"h-partial","CertificateID":"cert","TargetID":"target-1","Generation":1,"Status":"PARTIAL","ActivatedCount":2,"TotalCount":5,"Version":1}`},
		{"h-confirmed", `{"ID":"h-confirmed","CertificateID":"cert","TargetID":"target-1","Generation":1,"Status":"CONFIRMED","ActivatedCount":3,"TotalCount":3,"Version":1}`},
		{"h-failed", `{"ID":"h-failed","CertificateID":"cert","TargetID":"target-1","Generation":1,"Status":"FAILED","ActivatedCount":1,"TotalCount":3,"Version":1}`},
	}
	for _, h := range historical {
		if err := os.WriteFile(filepath.Join(deployDir, h.name+".json"), []byte(h.json), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewFileRepository(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	got, err := query.NewQueryService(repo).FindDeploymentGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 待处理与部分激活两类未完成且非失败批次应被识别为缺口；
	// 已确认与失败批次被排除。os.ReadDir 按文件名排序遍历，
	// 因此缺口顺序为 h-partial, h-pending。
	want := []string{"h-partial", "h-pending"}
	if len(got) != len(want) {
		t.Fatalf("gap count = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gap[%d] = %q, want %q (got=%v)", i, got[i], want[i], got)
		}
	}

	// 校验历史批次被重建为正确的枚举值与终态判定。
	batches, err := repo.Deployments.ListBatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]*domain.DeploymentBatch, len(batches))
	for _, b := range batches {
		byID[b.ID] = b
	}
	if b := byID["h-confirmed"]; b == nil || b.Status != domain.BatchConfirmed || !b.Status.IsFinished() {
		t.Fatalf("historical CONFIRMED batch not reconstructed: %+v", b)
	}
	if b := byID["h-failed"]; b == nil || b.Status != domain.BatchFailed || !b.Status.IsFinished() {
		t.Fatalf("historical FAILED batch not reconstructed: %+v", b)
	}
	if b := byID["h-pending"]; b == nil || b.Status != domain.BatchPending || b.Status.IsFinished() {
		t.Fatalf("historical PENDING batch not reconstructed: %+v", b)
	}
	if b := byID["h-partial"]; b == nil || b.Status != domain.BatchPartial || b.Status.IsFinished() {
		t.Fatalf("historical PARTIAL batch not reconstructed: %+v", b)
	}
}
