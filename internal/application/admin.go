package application

import (
	"github.com/linskybing/platform-go/internal/repository"
)

type AdminService struct {
	Repos *repository.Repos
}

func NewAdminService(repos *repository.Repos) *AdminService {
	return &AdminService{Repos: repos}
}
