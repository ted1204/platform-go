package services

import "github.com/linskybing/platform-go/repositories"

type Services struct {
	Audit      *AuditService
	ConfigFile *ConfigFileService
	Group      *GroupService
	Project    *ProjectService
	Resource   *ResourceService
	UserGroup  *UserGroupService
	User       *UserService
}

func New(repos *repositories.Repos) *Services {
	return &Services{
		Audit:      NewAuditService(repos),
		ConfigFile: NewConfigFileService(repos),
		Group:      NewGroupService(repos),
		Project:    NewProjectService(repos),
		Resource:   NewResourceService(repos),
		UserGroup:  NewUserGroupService(repos),
		User:       NewUserService(repos),
	}
}
