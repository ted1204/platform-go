package dto

type UpdateUserInput struct {
	OldPassword *string `form:"old_password" example:"oldPass123"`
	Password    *string `form:"password" example:"newPass123"`
	Email       *string `form:"email" example:"user@example.com"`
	FullName    *string `form:"full_name" example:"John Doe"`
	Type        *string `form:"type" binding:"omitempty,oneof=origin oauth2" example:"origin"`
	Status      *string `form:"status" binding:"omitempty,oneof=online offline delete" example:"online"`
}

type CreateUserInput struct {
	Username string  `form:"username" binding:"required" example:"johndoe"`
	Password string  `form:"password" binding:"required" example:"password123"`
	Email    *string `form:"email" example:"user@example.com"`
	FullName *string `form:"full_name" example:"John Doe"`
	Type     *string `form:"type" binding:"omitempty,oneof=origin oauth2" example:"origin"`
	Status   *string `form:"status" binding:"omitempty,oneof=online offline delete" example:"online"`
}

type UserDTO struct {
	Uid       uint    `json:"u_id" example:"123"`
	Username  string  `json:"username" example:"johndoe"`
	Email     *string `json:"email" example:"user@example.com"`
	FullName  *string `json:"full_name" example:"John Doe"`
	Type      string  `json:"type" example:"origin"`
	Status    string  `json:"status" example:"online"`
	CreatedAt string  `json:"create_at" example:"2025-07-17 15:20:41"`
	UpdatedAt string  `json:"update_at" example:"2025-07-17 15:20:41"`
	IsAdmin   bool    `json:"is_admin" example:"true"`
}
