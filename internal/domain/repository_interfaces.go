package domain

import "context"

type ApplicationRepository interface {
	Create(ctx context.Context, app *Application) error
	Update(ctx context.Context, app *Application) error
	Get(ctx context.Context, id string) (*Application, error)
	List(ctx context.Context) ([]*Application, error)
}

type CertificateVersionRepository interface {
	Create(ctx context.Context, cert *CertificateVersion) error
	Update(ctx context.Context, cert *CertificateVersion) error
	Get(ctx context.Context, id string) (*CertificateVersion, error)
	ListByApplication(ctx context.Context, appID string) ([]*CertificateVersion, error)
}

type DeploymentRepository interface {
	CreateBatch(ctx context.Context, batch *DeploymentBatch) error
	UpdateBatch(ctx context.Context, batch *DeploymentBatch) error
	GetBatch(ctx context.Context, id string) (*DeploymentBatch, error)
	ListBatches(ctx context.Context) ([]*DeploymentBatch, error)
	CreateConfirmation(ctx context.Context, conf *ActivationConfirmation) error
	ListConfirmations(ctx context.Context, certID string) ([]*ActivationConfirmation, error)
	DeleteConfirmation(ctx context.Context, id string) error
}

type RenewalRepository interface {
	CreatePlan(ctx context.Context, plan *RenewalPlan) error
	UpdatePlan(ctx context.Context, plan *RenewalPlan) error
	GetPlan(ctx context.Context, id string) (*RenewalPlan, error)
	ListPlans(ctx context.Context) ([]*RenewalPlan, error)
}

type RevocationRepository interface {
	CreateCredential(ctx context.Context, cred *RevocationCredential) error
	GetCredential(ctx context.Context, id string) (*RevocationCredential, error)
	ListByCertificate(ctx context.Context, certID string) ([]*RevocationCredential, error)
}

type ObservationRepository interface {
	CreateObservation(ctx context.Context, obs *ValidityObservation) error
	ListObservations(ctx context.Context) ([]*ValidityObservation, error)
}
