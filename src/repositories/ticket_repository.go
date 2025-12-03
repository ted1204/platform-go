package repositories

import (
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/models"
)

type TicketRepository struct{}

func NewTicketRepository() *TicketRepository {
	return &TicketRepository{}
}

func (r *TicketRepository) Create(ticket *models.Ticket) error {
	return db.DB.Create(ticket).Error
}

func (r *TicketRepository) FindAll() ([]models.Ticket, error) {
	var tickets []models.Ticket
	err := db.DB.Preload("User").Preload("Project").Order("created_at desc").Find(&tickets).Error
	return tickets, err
}

func (r *TicketRepository) FindByUserID(userID uint) ([]models.Ticket, error) {
	var tickets []models.Ticket
	err := db.DB.Where("user_id = ?", userID).Preload("User").Preload("Project").Order("created_at desc").Find(&tickets).Error
	return tickets, err
}

func (r *TicketRepository) FindByID(id uint) (*models.Ticket, error) {
	var ticket models.Ticket
	err := db.DB.Preload("User").Preload("Project").First(&ticket, id).Error
	return &ticket, err
}

func (r *TicketRepository) Update(ticket *models.Ticket) error {
	return db.DB.Save(ticket).Error
}
