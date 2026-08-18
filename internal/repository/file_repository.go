package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"certarchive/internal/domain"
)

type FileRepository struct {
	mu           sync.RWMutex
	baseDir      string
	Applications *fileApplicationRepo
	Certificates *fileCertificateRepo
	Targets      *fileTargetRepo
	Deployments  *fileDeploymentRepo
	Renewals     *fileRenewalRepo
	Revocations  *fileRevocationRepo
	Observations *fileObservationRepo
}

func NewFileRepository(baseDir string) (*FileRepository, error) {
	dirs := []string{
		filepath.Join(baseDir, "applications"),
		filepath.Join(baseDir, "certificates"),
		filepath.Join(baseDir, "targets"),
		filepath.Join(baseDir, "deployments"),
		filepath.Join(baseDir, "renewals"),
		filepath.Join(baseDir, "revocations"),
		filepath.Join(baseDir, "observations"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}
	repo := &FileRepository{baseDir: baseDir}
	repo.Applications = &fileApplicationRepo{dir: dirs[0]}
	repo.Certificates = &fileCertificateRepo{dir: dirs[1]}
	repo.Targets = &fileTargetRepo{dir: dirs[2]}
	repo.Deployments = &fileDeploymentRepo{dir: dirs[3]}
	repo.Renewals = &fileRenewalRepo{dir: dirs[4]}
	repo.Revocations = &fileRevocationRepo{dir: dirs[5]}
	repo.Observations = &fileObservationRepo{dir: dirs[6]}
	if err := repo.seedDefaults(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *FileRepository) seedDefaults() error {
	ctx := context.Background()
	t1 := &domain.DeploymentTarget{
		ID:        "target-1",
		Name:      "web-server-eu",
		Capacity:  100,
		Region:    "eu-west",
		CreatedAt: now(),
		Version:   1,
	}
	if _, err := os.Stat(filepath.Join(r.Targets.dir, t1.ID+".json")); os.IsNotExist(err) {
		if err := r.Targets.Create(ctx, t1); err != nil {
			return err
		}
	}
	t2 := &domain.DeploymentTarget{
		ID:        "target-2",
		Name:      "api-server-us",
		Capacity:  200,
		Region:    "us-east",
		CreatedAt: now(),
		Version:   1,
	}
	if _, err := os.Stat(filepath.Join(r.Targets.dir, t2.ID+".json")); os.IsNotExist(err) {
		if err := r.Targets.Create(ctx, t2); err != nil {
			return err
		}
	}
	return nil
}

func now() time.Time {
	return time.Now().UTC()
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type fileApplicationRepo struct {
	dir string
}

func (r *fileApplicationRepo) Create(ctx context.Context, app *domain.Application) error {
	if app == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, app.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, app)
}

func (r *fileApplicationRepo) Update(ctx context.Context, app *domain.Application) error {
	if app == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, app.ID+".json")
	var existing domain.Application
	if err := readJSON(path, &existing); err != nil {
		return err
	}
	if existing.Version != app.Version-1 {
		return domain.ErrOptimisticConflict
	}
	return writeJSON(path, app)
}

func (r *fileApplicationRepo) Get(ctx context.Context, id string) (*domain.Application, error) {
	path := filepath.Join(r.dir, id+".json")
	var app domain.Application
	if err := readJSON(path, &app); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &app, nil
}

func (r *fileApplicationRepo) List(ctx context.Context) ([]*domain.Application, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.Application
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var app domain.Application
			if err := readJSON(filepath.Join(r.dir, f.Name()), &app); err == nil {
				result = append(result, &app)
			}
		}
	}
	return result, nil
}

type fileCertificateRepo struct {
	dir string
}

func (r *fileCertificateRepo) Create(ctx context.Context, cert *domain.CertificateVersion) error {
	if cert == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, cert.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, cert)
}

func (r *fileCertificateRepo) Update(ctx context.Context, cert *domain.CertificateVersion) error {
	if cert == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, cert.ID+".json")
	var existing domain.CertificateVersion
	if err := readJSON(path, &existing); err != nil {
		return err
	}
	if existing.Version != cert.Version-1 {
		return domain.ErrOptimisticConflict
	}
	return writeJSON(path, cert)
}

func (r *fileCertificateRepo) Get(ctx context.Context, id string) (*domain.CertificateVersion, error) {
	path := filepath.Join(r.dir, id+".json")
	var cert domain.CertificateVersion
	if err := readJSON(path, &cert); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &cert, nil
}

func (r *fileCertificateRepo) ListByApplication(ctx context.Context, appID string) ([]*domain.CertificateVersion, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.CertificateVersion
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var c domain.CertificateVersion
			if readJSON(filepath.Join(r.dir, f.Name()), &c) == nil && c.ApplicationID == appID {
				result = append(result, &c)
			}
		}
	}
	return result, nil
}

type fileTargetRepo struct {
	dir string
}

func (r *fileTargetRepo) Create(ctx context.Context, t *domain.DeploymentTarget) error {
	if t == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, t.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, t)
}

func (r *fileTargetRepo) Update(ctx context.Context, t *domain.DeploymentTarget) error {
	if t == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, t.ID+".json")
	var existing domain.DeploymentTarget
	if err := readJSON(path, &existing); err != nil {
		return err
	}
	if existing.Version != t.Version-1 {
		return domain.ErrOptimisticConflict
	}
	return writeJSON(path, t)
}

func (r *fileTargetRepo) Get(ctx context.Context, id string) (*domain.DeploymentTarget, error) {
	path := filepath.Join(r.dir, id+".json")
	var t domain.DeploymentTarget
	if err := readJSON(path, &t); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *fileTargetRepo) List(ctx context.Context) ([]*domain.DeploymentTarget, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.DeploymentTarget
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var t domain.DeploymentTarget
			if readJSON(filepath.Join(r.dir, f.Name()), &t) == nil {
				result = append(result, &t)
			}
		}
	}
	return result, nil
}

type fileDeploymentRepo struct {
	dir string
}

func (r *fileDeploymentRepo) CreateBatch(ctx context.Context, b *domain.DeploymentBatch) error {
	if b == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, b.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, b)
}

func (r *fileDeploymentRepo) UpdateBatch(ctx context.Context, b *domain.DeploymentBatch) error {
	if b == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, b.ID+".json")
	var existing domain.DeploymentBatch
	if err := readJSON(path, &existing); err != nil {
		return err
	}
	if existing.Version != b.Version-1 {
		return domain.ErrOptimisticConflict
	}
	return writeJSON(path, b)
}

func (r *fileDeploymentRepo) GetBatch(ctx context.Context, id string) (*domain.DeploymentBatch, error) {
	path := filepath.Join(r.dir, id+".json")
	var b domain.DeploymentBatch
	if err := readJSON(path, &b); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *fileDeploymentRepo) ListBatches(ctx context.Context) ([]*domain.DeploymentBatch, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.DeploymentBatch
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var b domain.DeploymentBatch
			if readJSON(filepath.Join(r.dir, f.Name()), &b) == nil {
				result = append(result, &b)
			}
		}
	}
	return result, nil
}

func (r *fileDeploymentRepo) CreateConfirmation(ctx context.Context, conf *domain.ActivationConfirmation) error {
	if conf == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, "confirmations", conf.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, conf)
}

func (r *fileDeploymentRepo) ListConfirmations(ctx context.Context, certID string) ([]*domain.ActivationConfirmation, error) {
	dir := filepath.Join(r.dir, "confirmations")
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.ActivationConfirmation{}, nil
		}
		return nil, err
	}
	var result []*domain.ActivationConfirmation
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var c domain.ActivationConfirmation
			if readJSON(filepath.Join(dir, f.Name()), &c) == nil && c.CertificateID == certID {
				result = append(result, &c)
			}
		}
	}
	return result, nil
}

type fileRenewalRepo struct {
	dir string
}

func (r *fileRenewalRepo) CreatePlan(ctx context.Context, plan *domain.RenewalPlan) error {
	if plan == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, plan.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, plan)
}

func (r *fileRenewalRepo) UpdatePlan(ctx context.Context, plan *domain.RenewalPlan) error {
	if plan == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, plan.ID+".json")
	var existing domain.RenewalPlan
	if err := readJSON(path, &existing); err != nil {
		return err
	}
	if existing.Version != plan.Version-1 {
		return domain.ErrOptimisticConflict
	}
	return writeJSON(path, plan)
}

func (r *fileRenewalRepo) GetPlan(ctx context.Context, id string) (*domain.RenewalPlan, error) {
	path := filepath.Join(r.dir, id+".json")
	var plan domain.RenewalPlan
	if err := readJSON(path, &plan); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &plan, nil
}

func (r *fileRenewalRepo) ListPlans(ctx context.Context) ([]*domain.RenewalPlan, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.RenewalPlan
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var p domain.RenewalPlan
			if readJSON(filepath.Join(r.dir, f.Name()), &p) == nil {
				result = append(result, &p)
			}
		}
	}
	return result, nil
}

type fileRevocationRepo struct {
	dir string
}

func (r *fileRevocationRepo) CreateCredential(ctx context.Context, cred *domain.RevocationCredential) error {
	if cred == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, cred.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, cred)
}

func (r *fileRevocationRepo) GetCredential(ctx context.Context, id string) (*domain.RevocationCredential, error) {
	path := filepath.Join(r.dir, id+".json")
	var cred domain.RevocationCredential
	if err := readJSON(path, &cred); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &cred, nil
}

func (r *fileRevocationRepo) ListByCertificate(ctx context.Context, certID string) ([]*domain.RevocationCredential, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.RevocationCredential
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var c domain.RevocationCredential
			if readJSON(filepath.Join(r.dir, f.Name()), &c) == nil && c.CertificateID == certID {
				result = append(result, &c)
			}
		}
	}
	return result, nil
}

type fileObservationRepo struct {
	dir string
}

func (r *fileObservationRepo) CreateObservation(ctx context.Context, obs *domain.ValidityObservation) error {
	if obs == nil {
		return domain.ErrInvalidArgument
	}
	path := filepath.Join(r.dir, obs.ID+".json")
	if fileExists(path) {
		return domain.ErrAlreadyExists
	}
	return writeJSON(path, obs)
}

func (r *fileObservationRepo) ListObservations(ctx context.Context) ([]*domain.ValidityObservation, error) {
	files, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var result []*domain.ValidityObservation
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			var o domain.ValidityObservation
			if readJSON(filepath.Join(r.dir, f.Name()), &o) == nil {
				result = append(result, &o)
			}
		}
	}
	return result, nil
}
