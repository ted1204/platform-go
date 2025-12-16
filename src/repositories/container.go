package repositories

type Repos struct {
	ConfigFile ConfigFileRepo
	Group      GroupRepo
	Project    ProjectRepo
	Resource   ResourceRepo
	UserGroup  UserGroupRepo
	User       UserRepo
	View       ViewRepo
	Audit      AuditRepo
	Form       *FormRepository
	GPURequest GPURequestRepo
}

func New() *Repos {
	return &Repos{
		ConfigFile: &DBConfigFileRepo{},
		Group:      &DBGroupRepo{},
		Project:    &DBProjectRepo{},
		Resource:   &DBResourceRepo{},
		UserGroup:  &DBUserGroupRepo{},
		User:       &DBUserRepo{},
		View:       &DBViewRepo{},
		Audit:      &DBAuditRepo{},
		Form:       NewFormRepository(),
		GPURequest: &DBGPURequestRepo{},
	}
}
