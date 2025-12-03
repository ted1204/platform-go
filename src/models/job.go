package models

import (
	"time"
)

type Job struct {
	ID          uint      `gorm:"primaryKey;column:id"`
	UserID      uint      `gorm:"not null;column:user_id"`
	Name        string    `gorm:"size:100;not null"`
	Namespace   string    `gorm:"size:100;not null"`
	Image       string    `gorm:"size:255;not null"`
	Status      string    `gorm:"size:50;default:'Pending'"`
	K8sJobName  string    `gorm:"size:100;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Job) TableName() string {
	return "jobs"
}
