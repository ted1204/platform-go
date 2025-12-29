package services

import (
	"errors"
	"time"

	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
)

type GPURequestService struct {
	Repos *repositories.Repos
}

func NewGPURequestService(repos *repositories.Repos) *GPURequestService {
	return &GPURequestService{
		Repos: repos,
	}
}

func (s *GPURequestService) CreateRequest(projectID uint, userID uint, input dto.CreateGPURequestDTO) (models.GPURequest, error) {
	req := models.GPURequest{
		ProjectID:   projectID,
		RequesterID: userID,
		Type:        models.GPURequestType(input.Type),
		Reason:      input.Reason,
		Status:      models.GPURequestStatusPending,
	}

	if input.RequestedQuota != nil {
		req.RequestedQuota = *input.RequestedQuota
	}
	if input.RequestedAccessType != nil {
		req.RequestedAccessType = *input.RequestedAccessType
	}

	err := s.Repos.GPURequest.Create(&req)
	return req, err
}

func (s *GPURequestService) ListByProject(projectID uint) ([]models.GPURequest, error) {
	return s.Repos.GPURequest.ListByProjectID(projectID)
}

func (s *GPURequestService) ListPending() ([]models.GPURequest, error) {
	return s.Repos.GPURequest.ListPending()
}

func (s *GPURequestService) ProcessRequest(requestID uint, status string) (models.GPURequest, error) {
	req, err := s.Repos.GPURequest.GetByID(requestID)
	if err != nil {
		return models.GPURequest{}, err
	}

	if req.Status != models.GPURequestStatusPending {
		return req, errors.New("request is not pending")
	}

	req.Status = models.GPURequestStatus(status)
	req.UpdatedAt = time.Now()

	if status == string(models.GPURequestStatusApproved) {
		// Update Project
		project, err := s.Repos.Project.GetProjectByID(req.ProjectID)
		if err != nil {
			return req, err
		}

		if req.Type == models.GPURequestTypeQuota {
			project.GPUQuota = req.RequestedQuota
		} else if req.Type == models.GPURequestTypeAccess {
			project.GPUAccess = req.RequestedAccessType
		}

		if err := s.Repos.Project.UpdateProject(&project); err != nil {
			return req, err
		}
	}

	if err := s.Repos.GPURequest.Update(&req); err != nil {
		return req, err
	}

	return req, nil
}
