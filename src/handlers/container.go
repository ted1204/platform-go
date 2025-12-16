package handlers

import (
	"github.com/linskybing/platform-go/src/services"
)

type Handlers struct {
	Audit      *AuditHandler
	ConfigFile *ConfigFileHandler
	Group      *GroupHandler
	Project    *ProjectHandler
	Resource   *ResourceHandler
	UserGroup  *UserGroupHandler
	User       *UserHandler
	K8s        *K8sHandler
	Form       *FormHandler
	GPURequest *GPURequestHandler
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
		K8s:        NewK8sHandler(svc.K8s),
		Form:       NewFormHandler(svc.Form),
		GPURequest: NewGPURequestHandler(svc.GPURequest, svc.Project),
	}
}
