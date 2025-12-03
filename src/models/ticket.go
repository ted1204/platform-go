package models

import "gorm.io/gorm"

type TicketStatus string

const (
	TicketStatusPending    TicketStatus = "Pending"
	TicketStatusProcessing TicketStatus = "Processing"
	TicketStatusCompleted  TicketStatus = "Completed"
	TicketStatusRejected   TicketStatus = "Rejected"
)

type Ticket struct {
	gorm.Model
	UserID      uint         `json:"user_id"`
	ProjectID   *uint        `json:"project_id"` // Optional
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TicketStatus `json:"status" gorm:"default:'Pending'"`
	User        User         `json:"user" gorm:"foreignKey:UserID"`
	Project     *Project     `json:"project" gorm:"foreignKey:ProjectID"`
}
