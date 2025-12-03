package services

import (
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
)

type TicketService struct {
	repo *repositories.TicketRepository
}

func NewTicketService(repo *repositories.TicketRepository) *TicketService {
	return &TicketService{repo: repo}
}

func (s *TicketService) CreateTicket(userID uint, input dto.CreateTicketDTO) (*models.Ticket, error) {
	ticket := &models.Ticket{
		UserID:      userID,
		ProjectID:   input.ProjectID,
		Title:       input.Title,
		Description: input.Description,
		Status:      models.TicketStatusPending,
	}
	return ticket, s.repo.Create(ticket)
}

func (s *TicketService) GetAllTickets() ([]models.Ticket, error) {
	return s.repo.FindAll()
}

func (s *TicketService) GetUserTickets(userID uint) ([]models.Ticket, error) {
	return s.repo.FindByUserID(userID)
}

func (s *TicketService) UpdateTicketStatus(id uint, status string) (*models.Ticket, error) {
	ticket, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	ticket.Status = models.TicketStatus(status)
	return ticket, s.repo.Update(ticket)
}
