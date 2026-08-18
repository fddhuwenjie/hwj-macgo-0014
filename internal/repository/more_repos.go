package repository

import (
	"context"
	"os"
	"path/filepath"

	"certarchive/internal/domain"
)

// 为 FileRepository 添加 ListAll 方法用于查询
func (r *FileRepository) ListCertificatesAll(ctx context.Context) ([]*domain.CertificateVersion, error) {
	files, err := os.ReadDir(r.Certificates.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.CertificateVersion
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var c domain.CertificateVersion
			if readJSON(filepath.Join(r.Certificates.dir, f.Name()), &c) == nil {
				result = append(result, &c)
			}
		}
	}
	return result, nil
}

// 将 ListCertificatesAll 暴露给 query 使用
func (r *FileRepository) CertAll(ctx context.Context) ([]*domain.CertificateVersion, error) {
	return r.ListCertificatesAll(ctx)
}

func (r *FileRepository) ListTargetsAll(ctx context.Context) ([]*domain.DeploymentTarget, error) {
	files, err := os.ReadDir(r.Targets.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.DeploymentTarget
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var t domain.DeploymentTarget
			if readJSON(filepath.Join(r.Targets.dir, f.Name()), &t) == nil {
				result = append(result, &t)
			}
		}
	}
	return result, nil
}

// 为证书仓库加入 ListAll 方法（不影响接口）
func (r *fileCertificateRepo) ListAll(ctx context.Context) ([]*domain.CertificateVersion, error) {
	return r.listAll(ctx)
}

func (r *fileCertificateRepo) listAll(ctx context.Context) ([]*domain.CertificateVersion, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.CertificateVersion
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var c domain.CertificateVersion
			if readJSON(filepath.Join(r.dir, f.Name()), &c) == nil {
				result = append(result, &c)
			}
		}
	}
	return result, nil
}

// 其它辅助函数
func (r *FileRepository) Close() error {
	return nil
}
