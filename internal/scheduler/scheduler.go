package scheduler

import (
	"context"
	"sync"
	"time"

	"certarchive/internal/application"
	"certarchive/internal/query"
	"certarchive/internal/recovery"
)

type Scheduler struct {
	app *application.CertificateService
	qry *query.QueryService
	rec *recovery.RecoveryManager
	mu  sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func NewScheduler(app *application.CertificateService, qry *query.QueryService, rec *recovery.RecoveryManager) *Scheduler {
	return &Scheduler{app: app, qry: qry, rec: rec, stop: make(chan struct{}), done: make(chan struct{})}
}

func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				// 执行续期扫描和快照轮换
				_ = s.runTasks(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scheduler) runTasks(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 续期扫描：查找即将到期证书并创建续期计划（简化）
	_, _ = s.qry.ListExpiringSoon(ctx, 30)
	// 快照轮换
	_ = s.rec.RotateSnapshots(ctx, 5)
	return nil
}
