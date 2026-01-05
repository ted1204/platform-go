package form

import (
	"time"

	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/user"
	"gorm.io/gorm"
)

type FormStatus string

const (
	FormStatusPending    FormStatus = "Pending"
	FormStatusProcessing FormStatus = "Processing"
	FormStatusCompleted  FormStatus = "Completed"
	FormStatusRejected   FormStatus = "Rejected"
)

type Form struct {
	gorm.Model
	UserID      uint             `json:"user_id"`
	ProjectID   *uint            `json:"project_id"` // Optional
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Tag         string           `json:"tag"` // Single-select tag configured by backend
	Status      FormStatus       `json:"status" gorm:"default:'Pending'"`
	User        user.User        `json:"user" gorm:"foreignKey:UserID"`
	Project     *project.Project `json:"project" gorm:"foreignKey:ProjectID"`
	Messages    []FormMessage    `json:"messages" gorm:"foreignKey:FormID"`
}

// FormMessage represents a comment on a form. Both admin and requester can post.
type FormMessage struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	FormID    uint   `json:"form_id" gorm:"index"`
	UserID    uint   `json:"user_id"`
	Content   string `json:"content" gorm:"type:text"`
	CreatedAt time.Time
}
