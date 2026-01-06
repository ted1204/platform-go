package project

// Repository defines data access interface for projects
type Repository interface {
	Create(project *Project) error
	GetByID(pid uint) (*Project, error)
	FindByID(pid uint) (*Project, error) // Alias for GetByID
	GetByName(name string) (*Project, error)
	GetByGroupID(gid uint) ([]Project, error)
	List() ([]Project, error)
	Update(project *Project) error
	Delete(pid uint) error
	UpdateGPUQuota(pid uint, quota int) error
}
