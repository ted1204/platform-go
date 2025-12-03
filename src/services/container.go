package services

import "github.com/linskybing/platform-go/src/repositories"

type Services struct {
	Audit      *AuditService
	ConfigFile *ConfigFileService
	Group      *GroupService
	Project    *ProjectService
	Resource   *ResourceService
	UserGroup  *UserGroupService
	User       *UserService
	K8s        *K8sService
	Ticket     *TicketService
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
		K8s:        NewK8sService(repos),
		Ticket:     NewTicketService(repos.Ticket),
	}
}
