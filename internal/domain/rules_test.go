package domain_test

import (
	"testing"
	"time"
	"certarchive/internal/domain"
)

func TestValidityOverlap(t *testing.T) {
	a1 := time.Now()
	a2 := a1.Add(time.Hour)
	b1 := a1.Add(30 * time.Minute)
	b2 := a1.Add(2 * time.Hour)
	if !domain.CheckValidityOverlap(a1, a2, b1, b2) {
		t.Fatal("expected overlap")
	}
	if domain.CheckValidityOverlap(a1, a1.Add(time.Hour), a1.Add(2*time.Hour), a1.Add(3*time.Hour)) {
		t.Fatal("did not expect overlap")
	}
}
