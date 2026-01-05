package application

import (
	"github.com/linskybing/platform-go/internal/domain/form"
	"github.com/linskybing/platform-go/internal/repository"
)

type FormService struct {
	repo *repository.FormRepository
}

func NewFormService(repo *repository.FormRepository) *FormService {
	return &FormService{repo: repo}
}

func (s *FormService) CreateForm(userID uint, input form.CreateFormDTO) (*form.Form, error) {
	f := &form.Form{
		UserID:      userID,
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Description: input.Description,
		Tag:         input.Tag, // TODO: enforce allowed tags from config
		Status:      form.FormStatusPending,
	}
	return f, s.repo.Create(f)
}

func (s *FormService) GetAllForms() ([]form.Form, error) {
	return s.repo.FindAll()
}

func (s *FormService) GetUserForms(userID uint) ([]form.Form, error) {
	return s.repo.FindByUserID(userID)
}

func (s *FormService) UpdateFormStatus(id uint, status string) (*form.Form, error) {
	f, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	f.Status = form.FormStatus(status)
	return f, s.repo.Update(f)
}

func (s *FormService) AddMessage(formID, userID uint, content string) (*form.FormMessage, error) {
	f, err := s.repo.FindByID(formID)
	if err != nil {
		return nil, err
	}
	// TODO: block messages when status is Completed; currently allow to unblock later if needed
	msg := &form.FormMessage{FormID: f.ID, UserID: userID, Content: content}
	return msg, s.repo.CreateMessage(msg)
}

func (s *FormService) ListMessages(formID uint) ([]form.FormMessage, error) {
	return s.repo.ListMessages(formID)
}
