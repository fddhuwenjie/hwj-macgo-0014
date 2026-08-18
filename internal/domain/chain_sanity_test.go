package domain_test

import (
	"testing"

	"certarchive/internal/domain"
)

func TestChainVisibilitySanity(t *testing.T) {
	// 锁定 -> 签发：修复后必须合法，且与事件映射一致
	if err := domain.ValidateTransition(domain.ApplicationLocked, domain.ApplicationIssued); err != nil {
		t.Fatalf("LOCKED -> ISSUED should be valid: %v", err)
	}
	next, err := domain.NextStatusForEvent("issue", domain.ApplicationLocked)
	if err != nil || next != domain.ApplicationIssued {
		t.Fatalf("NextStatusForEvent(issue, LOCKED) = %v %v, want ISSUED", next, err)
	}
	// 草稿 / 已撤销直接签发仍被拒绝
	if domain.ValidateTransition(domain.ApplicationDraft, domain.ApplicationIssued) == nil {
		t.Fatal("DRAFT -> ISSUED must be rejected")
	}
	if domain.ValidateTransition(domain.ApplicationRevoked, domain.ApplicationIssued) == nil {
		t.Fatal("REVOKED -> ISSUED must be rejected")
	}
	// 锁定 -> 撤销 仍合法
	if domain.ValidateTransition(domain.ApplicationLocked, domain.ApplicationRevoked) != nil {
		t.Fatal("LOCKED -> REVOKED should be valid")
	}
	// 完整正向链 DRAFT -> LOCKED -> ISSUED -> DEPLOYING -> ACTIVE 全部合法
	chain := []domain.ApplicationStatus{
		domain.ApplicationDraft,
		domain.ApplicationLocked,
		domain.ApplicationIssued,
		domain.ApplicationDeploying,
		domain.ApplicationActive,
	}
	for i := 0; i+1 < len(chain); i++ {
		if err := domain.ValidateTransition(chain[i], chain[i+1]); err != nil {
			t.Fatalf("forward chain %s -> %s: %v", chain[i], chain[i+1], err)
		}
	}
}
