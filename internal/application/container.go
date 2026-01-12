package application

import (
	"github.com/linskybing/platform-go/internal/application/job"
	"github.com/linskybing/platform-go/internal/repository"
)

type Services struct {
	Audit      *AuditService
	ConfigFile *ConfigFileService
	Group      *GroupService
	Project    *ProjectService
	Resource   *ResourceService
	UserGroup  *UserGroupService
	User       *UserService
	K8s        *K8sService
	Form       *FormService
	Job        *job.Service
	Image      *ImageService
}

func New(repos *repository.Repos) *Services {
	return &Services{
		Audit:      NewAuditService(repos),
		ConfigFile: NewConfigFileService(repos),
		Group:      NewGroupService(repos),
		Project:    NewProjectService(repos),
		Resource:   NewResourceService(repos),
		UserGroup:  NewUserGroupService(repos),
		User:       NewUserService(repos),
		K8s:        NewK8sService(repos),
		Form:       NewFormService(repos.Form),
		Job:        job.NewService(repos.Job, repos.User, repos.Project),
		Image:      NewImageService(repos.Image),
	}
}
