package services

import (
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
)

type FormService struct {
	repo *repositories.FormRepository
}

func NewFormService(repo *repositories.FormRepository) *FormService {
	return &FormService{repo: repo}
}

func (s *FormService) CreateForm(userID uint, input dto.CreateFormDTO) (*models.Form, error) {
	form := &models.Form{
		UserID:      userID,
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Description: input.Description,
		Status:      models.FormStatusPending,
	}
	return form, s.repo.Create(form)
}

func (s *FormService) GetAllForms() ([]models.Form, error) {
	return s.repo.FindAll()
}

func (s *FormService) GetUserForms(userID uint) ([]models.Form, error) {
	return s.repo.FindByUserID(userID)
}

func (s *FormService) UpdateFormStatus(id uint, status string) (*models.Form, error) {
	form, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	form.Status = models.FormStatus(status)
	return form, s.repo.Update(form)
}
