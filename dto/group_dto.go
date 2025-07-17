package dto

type GroupUpdateDTO struct {
	GroupName   *string `form:"group_name"`
	Description *string `form:"description"`
}

type GroupCreateDTO struct {
	GroupName   string  `form:"group_name" binding:"required"`
	Description *string `form:"description"`
}
