package application

import (
	"github.com/linskybing/platform-go/internal/domain/audit"
	"github.com/linskybing/platform-go/internal/repository"
)

type AuditService struct {
	Repos *repository.Repos
}

func NewAuditService(repos *repository.Repos) *AuditService {
	return &AuditService{
		Repos: repos,
	}
}

func (s *AuditService) QueryAuditLogs(params repository.AuditQueryParams) ([]audit.AuditLog, error) {
	return s.Repos.Audit.GetAuditLogs(params)
}

func (s *AuditService) CleanupOldLogs(days int) error {
	return s.Repos.Audit.DeleteOldAuditLogs(days)
}
