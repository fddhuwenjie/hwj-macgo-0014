package domain

import "time"

type DeploymentTarget struct {
	ID          string
	Name        string
	Capacity    int
	Region      string
	CreatedAt   time.Time
	Version     int64
}
