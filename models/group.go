package models

import "time"

type Group struct {
    GID       uint      `gorm:"primaryKey;column:g_id"`
    GroupName string    `gorm:"size:100;not null"`
    CreatedAt time.Time `gorm:"column:create_at"`
    UpdatedAt time.Time `gorm:"column:update_at"`
}
