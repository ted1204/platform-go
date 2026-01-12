package repository

import (
	"time"

	"github.com/linskybing/platform-go/internal/domain/audit"
	"gorm.io/gorm"
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
	GetAuditLogs(params AuditQueryParams) ([]audit.AuditLog, error)
	CreateAuditLog(audit *audit.AuditLog) error
	DeleteOldAuditLogs(retentionDays int) error
	WithTx(tx *gorm.DB) AuditRepo
}

type DBAuditRepo struct {
	db *gorm.DB
}

func NewAuditRepo(db *gorm.DB) *DBAuditRepo {
	return &DBAuditRepo{
		db: db,
	}
}

func (r *DBAuditRepo) DeleteOldAuditLogs(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return r.db.Where("created_at < ?", cutoff).Delete(&audit.AuditLog{}).Error
}

func (r *DBAuditRepo) GetAuditLogs(params AuditQueryParams) ([]audit.AuditLog, error) {
	var logs []audit.AuditLog
	query := r.db.Model(&audit.AuditLog{})

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

func (r *DBAuditRepo) CreateAuditLog(audit *audit.AuditLog) error {
	return r.db.Create(audit).Error
}

func (r *DBAuditRepo) WithTx(tx *gorm.DB) AuditRepo {
	if tx == nil {
		return r
	}
	return &DBAuditRepo{
		db: tx,
	}
}
