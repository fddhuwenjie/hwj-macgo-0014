package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"certarchive/internal/application"
	"certarchive/internal/audit"
	"certarchive/internal/journal"
	"certarchive/internal/query"
	"certarchive/internal/recovery"
	"certarchive/internal/repository"
	"certarchive/internal/scheduler"
)

func main() {
	selfCheck := flag.Bool("self-check", false, "run self-check and exit")
	flag.Parse()

	if *selfCheck {
		if err := runSelfCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "self-check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("self-check passed")
		return
	}

	fmt.Println("certarchive: use --self-check to validate installation")
}

func runSelfCheck() error {
	dir, err := os.MkdirTemp("", "certarchive-selfcheck")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	storeDir := filepath.Join(dir, "store")
	repo, err := repository.NewFileRepository(storeDir)
	if err != nil {
		return err
	}

	jrnl, err := journal.NewFileJournal(filepath.Join(dir, "journal"))
	if err != nil {
		return err
	}
	rec, err := recovery.NewRecoveryManager(storeDir, jrnl)
	if err != nil {
		return err
	}
	auditLog := audit.NewInMemoryAuditLog()
	app := application.NewCertificateService(repo, rec, jrnl, auditLog)
	qry := query.NewQueryService(repo)
	sched := scheduler.NewScheduler(app, qry, rec)

	ctx := context.Background()
	// The storage health check is the audit gate for on-disk integrity: a
	// lingering ".tmp" is evidence of an interrupted atomic write. Discarding
	// its verdict here would let a broken link pass as success, so the error
	// must propagate up to the self-check exit code.
	if _, err := repo.CheckStorageHealth(ctx); err != nil {
		return err
	}
	// 创建一个申请并走几步流程
	appID, err := app.SubmitApplication(ctx, "example.com", "RSA-2048", "Org1")
	if err != nil {
		return err
	}
	if err := app.LockApplication(ctx, appID); err != nil {
		return err
	}
	if err := app.RegisterIssuance(ctx, appID); err != nil {
		return err
	}

	// 启动调度器（无实际操作）
	sched.Start(ctx)
	defer sched.Stop()

	// 查询
	_, err = qry.ListExpiringSoon(ctx, 30)
	if err != nil {
		return err
	}
	return nil
}
