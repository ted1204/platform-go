package models

import "time"

type ProjectGroupView struct {
	GID           uint   `gorm:"column:g_id"`
	GroupName     string `gorm:"column:group_name"`
	ProjectCount  int64  `gorm:"column:project_count"`
	ResourceCount int64  `gorm:"column:resource_count"`
	GroupCreateAt string `gorm:"column:group_create_at"`
	GroupUpdateAt string `gorm:"column:group_update_at"`
}

type ProjectResourceView struct {
	PID              uint   `gorm:"column:p_id"`
	ProjectName      string `gorm:"column:project_name"`
	RID              uint   `gorm:"column:r_id"`
	Type             string `gorm:"column:type"`
	Name             string `gorm:"column:name"`
	Filename         string `gorm:"column:filename"`
	ResourceCreateAt string `gorm:"column:resource_create_at"`
	ResourceUpdateAt string `gorm:"column:resource_update_at"`
}

type UserGroupView struct {
	UID       uint   `gorm:"column:u_id"`
	Username  string `gorm:"column:username"`
	GID       uint   `gorm:"column:g_id"`
	GroupName string `gorm:"column:group_name"`
	Role      string `gorm:"column:role"`
}

type UserWithSuperAdmin struct {
	UID          uint      `gorm:"column:u_id" json:"u_id"`
	Username     string    `gorm:"column:username" json:"username"`
	Password     string    `gorm:"column:password" json:"-"`
	Email        string    `gorm:"column:email" json:"email"`
	FullName     string    `gorm:"column:full_name" json:"full_name"`
	Type         string    `gorm:"column:type" json:"type"`
	Status       string    `gorm:"column:status" json:"status"`
	CreatedAt    time.Time `gorm:"column:create_at" json:"create_at"`
	UpdatedAt    time.Time `gorm:"column:update_at" json:"update_at"`
	IsSuperAdmin bool      `gorm:"column:is_super_admin" json:"is_super_admin"`
}

func (UserWithSuperAdmin) TableName() string {
	return "users_with_superadmin"
}
