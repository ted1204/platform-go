package application

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/utils"
)

var ErrResourceNotFound = errors.New("resource not found")

type ResourceService struct {
	Repos *repository.Repos
}

func NewResourceService(repos *repository.Repos) *ResourceService {
	return &ResourceService{
		Repos: repos,
	}
}

func (s *ResourceService) ListResourcesByProjectID(projectID uint) ([]resource.Resource, error) {
	return s.Repos.Resource.ListResourcesByProjectID(projectID)
}

func (s *ResourceService) ListResourcesByConfigFileID(cfID uint) ([]resource.Resource, error) {
	return s.Repos.Resource.ListResourcesByConfigFileID(cfID)
}

func (s *ResourceService) GetResource(rid uint) (*resource.Resource, error) {
	return s.Repos.Resource.GetResourceByID(rid)
}

func (s *ResourceService) CreateResource(c *gin.Context, res *resource.Resource) (*resource.Resource, error) {
	err := s.Repos.Resource.CreateResource(res)
	if err != nil {
		return nil, err
	}

	utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", res.RID), nil, *res, "", s.Repos.Audit)
	return res, nil
}

func (s *ResourceService) UpdateResource(c *gin.Context, rid uint, input resource.ResourceUpdateDTO) (*resource.Resource, error) {
	existing, err := s.Repos.Resource.GetResourceByID(rid)
	if err != nil || existing == nil {
		return nil, ErrResourceNotFound
	}

	oldResource := *existing

	if input.Type != nil {
		existing.Type = resource.ResourceType(*input.Type)
	}
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.ParsedYAML != nil {
		existing.ParsedYAML = *input.ParsedYAML
	}
	if input.Description != nil {
		existing.Description = input.Description
	}

	err = s.Repos.Resource.UpdateResource(existing)
	if err != nil {
		return nil, err
	}
	logFn := utils.LogAuditWithConsole
	go func(fn func(*gin.Context, string, string, string, interface{}, interface{}, string, repository.AuditRepo)) {
		fn(c, "update", "resource", fmt.Sprintf("r_id=%d", existing.RID), oldResource, *existing, "", s.Repos.Audit)
	}(logFn)

	return existing, nil
}

func (s *ResourceService) DeleteResource(c *gin.Context, rid uint) error {
	resource, err := s.Repos.Resource.GetResourceByID(rid)
	if err != nil || resource == nil {
		return ErrResourceNotFound
	}

	err = s.Repos.Resource.DeleteResource(rid)
	if err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "resource", fmt.Sprintf("r_id=%d", resource.RID), *resource, nil, "", s.Repos.Audit)

	return nil
}
