package models

import "gorm.io/gorm"

type FormStatus string

const (
	FormStatusPending    FormStatus = "Pending"
	FormStatusProcessing FormStatus = "Processing"
	FormStatusCompleted  FormStatus = "Completed"
	FormStatusRejected   FormStatus = "Rejected"
)

type Form struct {
	gorm.Model
	UserID      uint       `json:"user_id"`
	ProjectID   *uint      `json:"project_id"` // Optional
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      FormStatus `json:"status" gorm:"default:'Pending'"`
	User        User       `json:"user" gorm:"foreignKey:UserID"`
	Project     *Project   `json:"project" gorm:"foreignKey:ProjectID"`
}
