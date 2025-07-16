package models

import "time"

type UserGroup struct {
    UID       uint      `gorm:"primaryKey;column:u_id"`
    GID       uint      `gorm:"primaryKey;column:g_id"`
    Role      string    `gorm:"type:user_role;default:user;not null"` // ENUM
    CreatedAt time.Time `gorm:"column:create_at"`
    UpdatedAt time.Time `gorm:"column:update_at"`
}
