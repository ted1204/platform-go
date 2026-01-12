package repository

import (
	"github.com/linskybing/platform-go/internal/domain/view"
	"gorm.io/gorm"
)

// ViewRepo is a compatibility interface kept for tests and older code.
// The project now prefers using specific repos (e.g., UserGroupRepo, ProjectRepo),
// but some places still reference ViewRepo; this file provides the minimal
// interface so those references compile without changing program logic.
type ViewRepo interface {
	GetAllProjectGroupViews() ([]view.ProjectGroupView, error)
	GetGroupIDByConfigFileID(cfID uint) (uint, error)
	GetGroupIDByResourceID(rID uint) (uint, error)
	GetGroupResourcesByGroupID(groupID uint) ([]view.GroupResourceView, error)
	GetProjectResourcesByGroupID(groupID uint) ([]view.ProjectResourceView, error)
	GetUserRoleInGroup(uid, gid uint) (string, error)
	IsSuperAdmin(uid uint) (bool, error)
	ListProjectsByUserID(userID uint) ([]view.ProjectUserView, error)
	ListUsersByProjectID(projectID uint) ([]view.ProjectUserView, error)
	WithTx(tx *gorm.DB) ViewRepo
}
