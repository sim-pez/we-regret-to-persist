package entity

import "time"

type ApplicationStatus string

const (
	ApplicationStatusApplied   ApplicationStatus = "applied"
	ApplicationStatusRejected  ApplicationStatus = "rejected"
	ApplicationStatusAdvanced  ApplicationStatus = "advanced"
	ApplicationStatusUnrelated ApplicationStatus = "unrelated"
)

type Application struct {
	Company    string
	AppliedAt  *time.Time
	RejectedAt *time.Time
	Status     ApplicationStatus
}
