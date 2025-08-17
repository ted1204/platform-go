package repositories

import (
	"time"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type AuditQueryParams struct {
	UserID       *uint
	ResourceType *string
	Action       *string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

type AuditRepo interface {
	GetAuditLogs(params AuditQueryParams) ([]models.AuditLog, error)
	CreateAuditLog(audit *models.AuditLog) error
}

type DBAuditRepo struct{}

func (r *DBAuditRepo) GetAuditLogs(params AuditQueryParams) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := db.DB.Model(&models.AuditLog{})

	if params.UserID != nil {
		query = query.Where("user_id = ?", *params.UserID)
	}
	if params.ResourceType != nil {
		query = query.Where("resource_type = ?", *params.ResourceType)
	}
	if params.Action != nil {
		query = query.Where("action = ?", *params.Action)
	}
	if params.StartTime != nil {
		query = query.Where("created_at >= ?", *params.StartTime)
	}
	if params.EndTime != nil {
		query = query.Where("created_at <= ?", *params.EndTime)
	}

	query = query.Order("created_at DESC")
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	err := query.Find(&logs).Error
	return logs, err
}

func (r *DBAuditRepo) CreateAuditLog(audit *models.AuditLog) error {
	return db.DB.Create(audit).Error
}
