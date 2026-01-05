package group

type GroupUpdateDTO struct {
	GroupName   *string `json:"group_name" form:"group_name"`
	Description *string `json:"description" form:"description"`
}

type GroupCreateDTO struct {
	GroupName   string  `json:"group_name" form:"group_name" binding:"required"`
	Description *string `json:"description" form:"description"`
}

type UserGroupInputDTO struct {
	UID  uint   `json:"uid" form:"uid" binding:"required"`
	GID  uint   `json:"gid" form:"gid" binding:"required"`
	Role string `json:"role" form:"role" binding:"required,oneof=admin manager user"`
}

type UserGroupDeleteDTO struct {
	UID uint `json:"uid" form:"uid" binding:"required"`
	GID uint `json:"gid" form:"gid" binding:"required"`
}

func (d UserGroupInputDTO) GetGID() uint {
	return d.GID
}

func (d UserGroupDeleteDTO) GetGID() uint {
	return d.GID
}
