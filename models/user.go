package models

import "time"

type UserStatus string

const (
	UserStatusOnline  UserStatus = "online"
	UserStatusOffline UserStatus = "offline"
	UserStatusDelete  UserStatus = "delete"
)

type UserType string

const (
	UserTypeOrigin UserType = "origin"
	UserTypeOauth2 UserType = "oauth2"
)

type UserRole string

const (
	UserRoleAdmin   UserRole = "admin"
	UserRoleManager UserRole = "manager"
	UserRoleUser    UserRole = "user"
)

type User struct {
	UID       uint      `gorm:"primaryKey;column:u_id"`
	Username  string    `gorm:"size:50;not null;unique" json:"Username"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Email     *string   `gorm:"size:100"`
	FullName  *string   `gorm:"size:50"`
	Type      string    `gorm:"type:user_type;default:'origin';not null"`
	Status    string    `gorm:"type:user_status;default:'offline';not null"`
	CreatedAt time.Time `gorm:"column:create_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:update_at;autoUpdateTime"`
}
