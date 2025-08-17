package handlers

import (
	"github.com/linskybing/platform-go/services"
)

type Handlers struct {
	Audit      *AuditHandler
	ConfigFile *ConfigFileHandler
	Group      *GroupHandler
	Project    *ProjectHandler
	Resource   *ResourceHandler
	UserGroup  *UserGroupHandler
	User       *UserHandler
}

func New(svc *services.Services) *Handlers {
	return &Handlers{
		Audit:      NewAuditHandler(svc.Audit),
		ConfigFile: NewConfigFileHandler(svc.ConfigFile),
		Group:      NewGroupHandler(svc.Group),
		Project:    NewProjectHandler(svc.Project),
		Resource:   NewResourceHandler(svc.Resource),
		UserGroup:  NewUserGroupHandler(svc.UserGroup),
		User:       NewUserHandler(svc.User),
	}
}
