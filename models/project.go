package models

import "time"

type Project struct {
	PID         uint      `gorm:"primaryKey;column:p_id"`
	ProjectName string    `gorm:"size:100;not null"`
	Description string    `gorm:"type:text"`
	GID         uint      `gorm:"not null"` // foreign key: group_list.g_id
	CreatedAt   time.Time `gorm:"column:create_at"`
	UpdatedAt   time.Time `gorm:"column:update_at"`
}
