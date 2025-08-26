package dto

type UserGroupInputDTO struct {
	UID  uint   `form:"u_id" binding:"required"`
	GID  uint   `form:"g_id" binding:"required"`
	Role string `form:"role" binding:"required,oneof=admin manager user"`
}

type UserGroupDeleteDTO struct {
	UID uint `form:"u_id" binding:"required"`
	GID uint `form:"g_id" binding:"required"`
}

func (d UserGroupInputDTO) GetGID() uint {
	return d.GID
}

func (d UserGroupDeleteDTO) GetGID() uint {
	return d.GID
}
