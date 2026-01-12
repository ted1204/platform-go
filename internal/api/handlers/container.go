package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/repository"
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
	Job        *JobHandler
	Image      *ImageHandler
	Router     *gin.Engine
}

func New(svc *application.Services, repos *repository.Repos, router *gin.Engine) *Handlers {
	h := &Handlers{
		Audit:      NewAuditHandler(svc.Audit),
		ConfigFile: NewConfigFileHandler(svc.ConfigFile),
		Group:      NewGroupHandler(svc.Group),
		Project:    NewProjectHandler(svc.Project),
		Resource:   NewResourceHandler(svc.Resource),
		UserGroup:  NewUserGroupHandler(svc.UserGroup),
		User:       NewUserHandler(svc.User),
		K8s:        NewK8sHandler(svc.K8s, svc.User, svc.Project),
		Form:       NewFormHandler(svc.Form),
		Job:        NewJobHandler(svc.Job, repos),
		Image:      NewImageHandler(svc.Image),
		Router:     router,
	}
	return h
}
