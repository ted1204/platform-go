package models

import "time"

type Resource struct {
    RID       uint      `gorm:"primaryKey;column:r_id"`
    Type      string    `gorm:"type:resource_type;not null"` // ENUM
    Name      string    `gorm:"size:50;not null"`
    Filename  string    `gorm:"size:200;not null"`
    PID       uint      `gorm:"not null"` // foreign key: project.p_id
    CreatedAt time.Time `gorm:"column:create_at"`
    UpdatedAt time.Time `gorm:"column:update_at"`
}
