package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
)

var ErrResourceNotFound = errors.New("resource not found")

type ResourceService struct {
	Repos *repositories.Repos
}

func NewResourceService(repos *repositories.Repos) *ResourceService {
	return &ResourceService{
		Repos: repos,
	}
}

func (s *ResourceService) ListResourcesByProjectID(projectID uint) ([]models.Resource, error) {
	return s.Repos.Resource.ListResourcesByProjectID(projectID)
}

func (s *ResourceService) ListResourcesByConfigFileID(cfID uint) ([]models.Resource, error) {
	return s.Repos.Resource.ListResourcesByConfigFileID(cfID)
}

func (s *ResourceService) GetResource(rid uint) (*models.Resource, error) {
	return s.Repos.Resource.GetResourceByID(rid)
}

func (s *ResourceService) CreateResource(c *gin.Context, resource *models.Resource) (*models.Resource, error) {
	err := s.Repos.Resource.CreateResource(resource)
	if err != nil {
		return nil, err
	}

	go utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", resource.RID), nil, *resource, "", s.Repos.Audit)
	return resource, nil
}

func (s *ResourceService) UpdateResource(c *gin.Context, rid uint, input dto.ResourceUpdateDTO) (*models.Resource, error) {
	existing, err := s.Repos.Resource.GetResourceByID(rid)
	if err != nil || existing == nil {
		return nil, ErrResourceNotFound
	}

	oldResource := *existing

	if input.Type != nil {
		existing.Type = *input.Type
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
	go utils.LogAuditWithConsole(c, "update", "resource", fmt.Sprintf("r_id=%d", existing.RID), oldResource, *existing, "", s.Repos.Audit)

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

	go utils.LogAuditWithConsole(c, "delete", "resource", fmt.Sprintf("r_id=%d", resource.RID), *resource, nil, "", s.Repos.Audit)

	return nil
}
