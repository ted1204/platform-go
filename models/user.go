package models

import "time"

type User struct {
    UID       uint      `gorm:"primaryKey;column:u_id"`
    Username  string    `gorm:"size:50;not null"`
    Password  string    `gorm:"size:255;not null"`
    Email     *string   `gorm:"size:100"`
    FullName  *string   `gorm:"size:50"`
    Type      string    `gorm:"type:user_type;default:origin;not null"`     // ENUM
    Status    string    `gorm:"type:user_status;default:offline;not null"`  // ENUM
    CreatedAt time.Time `gorm:"column:create_at"`
    UpdatedAt time.Time `gorm:"column:update_at"`
}