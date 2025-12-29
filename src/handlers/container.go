package handlers

import (
	"github.com/gin-gonic/gin"
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
	Router     *gin.Engine
}

func New(svc *services.Services, router *gin.Engine) *Handlers {
	h := &Handlers{
		Audit:      NewAuditHandler(svc.Audit),
		ConfigFile: NewConfigFileHandler(svc.ConfigFile),
		Group:      NewGroupHandler(svc.Group),
		Project:    NewProjectHandler(svc.Project),
		Resource:   NewResourceHandler(svc.Resource),
		UserGroup:  NewUserGroupHandler(svc.UserGroup),
		User:       NewUserHandler(svc.User),
		K8s:        NewK8sHandler(svc.K8s, svc.User),
		Form:       NewFormHandler(svc.Form),
		GPURequest: NewGPURequestHandler(svc.GPURequest, svc.Project),
		Router:     router,
	}
	adminHandler := NewAdminHandler(svc.User)
	h.Router.POST("/admin/ensure-user-pv", adminHandler.EnsureAllUserPV)
	return h
}
