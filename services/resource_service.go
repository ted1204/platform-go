package services

import (
	"errors"

	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
)

type ResourceService struct {
	Repo *repositories.ResourceRepository
}

func (s *ResourceService) CreateResource(input *models.Resource) error {
	if input.Name == "" {
		return errors.New("resource name is required")
	}
	if input.Type == "" {
		return errors.New("resource type is required")
	}

	return s.Repo.CreateResource(input)
}

func (s *ResourceService) GetResourceByID(rid uint) (*models.Resource, error) {
	return s.Repo.GetResourceByID(rid)
}

func (s *ResourceService) UpdateResource(input *models.Resource) error {
	if input.RID == 0 {
		return errors.New("resource ID is required")
	}
	existing, err := s.Repo.GetResourceByID(input.RID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("resource not found")
	}
	return s.Repo.UpdateResource(input)
}

func (s *ResourceService) DeleteResource(rid uint) error {
	return s.Repo.DeleteResource(rid)
}

func (s *ResourceService) ListResourcesByProjectID(pid uint) ([]models.Resource, error) {
	return s.Repo.ListResourcesByProjectID(pid)
}
