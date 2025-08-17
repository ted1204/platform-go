package models

import "time"

type ProjectGroupView struct {
	GID           uint   `gorm:"column:g_id" json:"GID"`
	GroupName     string `gorm:"column:group_name" json:"GroupName"`
	ProjectCount  int64  `gorm:"column:project_count" json:"ProjectCount"`
	ResourceCount int64  `gorm:"column:resource_count" json:"ResourceCount"`
	GroupCreateAt string `gorm:"column:group_create_at" json:"GroupCreateAt"`
	GroupUpdateAt string `gorm:"column:group_update_at" json:"GroupUpdateAt"`
}

type ProjectResourceView struct {
	PID              uint   `gorm:"column:p_id" json:"PID"`
	ProjectName      string `gorm:"column:project_name" json:"ProjectName"`
	RID              uint   `gorm:"column:r_id" json:"RID"`
	Type             string `gorm:"column:type" json:"Type"`
	Name             string `gorm:"column:name" json:"Name"`
	Filename         string `gorm:"column:filename" json:"Filename"`
	ResourceCreateAt string `gorm:"column:resource_create_at" json:"ResourceCreateAt"`
	ResourceUpdateAt string `gorm:"column:resource_update_at" json:"ResourceUpdateAt"`
}

type GroupResourceView struct {
	GID              uint   `gorm:"column:g_id" json:"GID"`
	GroupName        string `gorm:"column:group_name" json:"GroupName"`
	PID              uint   `gorm:"column:p_id" json:"PID"`
	ProjectName      string `gorm:"column:project_name" json:"ProjectName"`
	RID              uint   `gorm:"column:r_id" json:"RID"`
	ResourceType     string `gorm:"column:resource_type" json:"ResourceType"`
	ResourceName     string `gorm:"column:resource_name" json:"ResourceName"`
	Filename         string `gorm:"column:filename" json:"Filename"`
	ResourceCreateAt string `gorm:"column:resource_create_at" json:"ResourceCreateAt"`
	ResourceUpdateAt string `gorm:"column:resource_update_at" json:"ResourceUpdateAt"`
}

type UserGroupView struct {
	UID       uint   `gorm:"column:u_id" json:"UID"`
	Username  string `gorm:"column:username" json:"Username"`
	GID       uint   `gorm:"column:g_id" json:"GID"`
	GroupName string `gorm:"column:group_name" json:"GroupName"`
	Role      string `gorm:"column:role" json:"Role"`
}

type UserWithSuperAdmin struct {
	UID          uint      `gorm:"column:u_id" json:"UID"`
	Username     string    `gorm:"column:username" json:"Username"`
	Password     string    `gorm:"column:password" json:"-"`
	Email        string    `gorm:"column:email" json:"Email"`
	FullName     string    `gorm:"column:full_name" json:"FullName"`
	Type         string    `gorm:"column:type" json:"Type"`
	Status       string    `gorm:"column:status" json:"Status"`
	CreatedAt    time.Time `gorm:"column:create_at" json:"CreatedAt"`
	UpdatedAt    time.Time `gorm:"column:update_at" json:"UpdatedAt"`
	IsSuperAdmin bool      `gorm:"column:is_super_admin" json:"IsSuperAdmin"`
}

type ProjectUserView struct {
	PID         uint   `gorm:"column:p_id" json:"PID"`
	ProjectName string `gorm:"column:project_name" json:"ProjectName"`
	GID         uint   `gorm:"column:g_id" json:"GID"`
	GroupName   string `gorm:"column:group_name" json:"GroupName"`
	UID         uint   `gorm:"column:u_id" json:"UID"`
	Username    string `gorm:"column:username" json:"Username"`
}

func (UserWithSuperAdmin) TableName() string {
	return "users_with_superadmin"
}
