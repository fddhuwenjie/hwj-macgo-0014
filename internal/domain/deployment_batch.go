package domain

import "time"

// DeploymentBatchStatus 表示部署批次在仓储层的生命周期状态枚举。
//
// 状态语义：
//   - BatchPending：批次已创建但尚未有任何激活确认（已激活计数为 0）。
//   - BatchPartial：批次已部分激活（已激活计数小于总数，且大于 0）。
//   - BatchConfirmed：批次已全部激活并确认生效（已激活计数等于总数）。
//   - BatchFailed：批次激活失败，不再继续推进。
//
// 该类型是 string 的具名类型，JSON 序列化与原始字符串形式完全一致，
// 因此历史快照与磁盘上的批次文件可在不迁移数据的前提下重建。
type DeploymentBatchStatus string

const (
	BatchPending   DeploymentBatchStatus = "PENDING"
	BatchPartial   DeploymentBatchStatus = "PARTIAL"
	BatchConfirmed DeploymentBatchStatus = "CONFIRMED"
	BatchFailed    DeploymentBatchStatus = "FAILED"
)

// IsFinished 报告批次是否已进入终态（已确认或失败）。
// 终态批次不再构成部署缺口，也不会继续推进激活计数。
func (s DeploymentBatchStatus) IsFinished() bool {
	return s == BatchConfirmed || s == BatchFailed
}

// IsGap 报告该批次是否构成部署缺口：
// 未完成（已激活计数严格小于总数）且非已确认、非失败。
//
// 该口径刻意覆盖待处理与部分激活两类未完成批次，并排除已确认和失败两类终态批次：
//   - 待处理（0/N）与部分激活（k/N, k<N）均为缺口；
//   - 已确认（N/N）虽已激活完毕但因进入终态而被排除；
//   - 失败批次按统计口径排除，其处置由撤销/重新签发流程处理，不计入部署缺口。
//
// 注意：口径以“未完成且非失败”为正向识别，再排除已确认与失败；
// 因此单纯比较 ActivatedCount < TotalCount 并不充分（已确认批次计数恰好相等，
// 失败批次计数也可能小于总数），必须同时校验状态枚举。
func (b *DeploymentBatch) IsGap() bool {
	if b == nil {
		return false
	}
	return b.ActivatedCount < b.TotalCount &&
		b.Status != BatchConfirmed &&
		b.Status != BatchFailed
}

type DeploymentBatch struct {
	ID             string
	CertificateID  string
	TargetID       string
	Generation     int64
	Status         DeploymentBatchStatus
	ActivatedCount int
	TotalCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}
