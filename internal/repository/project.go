package repository

import (
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/view"
	"gorm.io/gorm"
)

type ProjectRepo interface {
	GetProjectByID(id uint) (project.Project, error)
	GetGroupIDByProjectID(pID uint) (uint, error)
	CreateProject(p *project.Project) error
	UpdateProject(p *project.Project) error
	DeleteProject(id uint) error
	ListProjects() ([]project.Project, error)
	ListProjectsByGroup(id uint) ([]project.Project, error)
	ListProjectsByUserID(userID uint) ([]view.ProjectUserView, error)
	GetAllProjectGroupViews() ([]view.ProjectGroupView, error)
	WithTx(tx *gorm.DB) ProjectRepo
}

type DBProjectRepo struct {
	db *gorm.DB
}

func NewProjectRepo(db *gorm.DB) *DBProjectRepo {
	return &DBProjectRepo{
		db: db,
	}
}

func (r *DBProjectRepo) GetProjectByID(id uint) (project.Project, error) {
	var project project.Project
	err := r.db.First(&project, id).Error
	return project, err
}

func (r *DBProjectRepo) GetGroupIDByProjectID(pID uint) (uint, error) {
	var gID uint
	err := r.db.Model(&project.Project{}).Select("g_id").Where("p_id = ?", pID).Scan(&gID).Error
	if err != nil {
		return 0, err
	}
	return gID, nil
}

func (r *DBProjectRepo) CreateProject(p *project.Project) error {
	if err := r.db.Create(p).Error; err != nil {
		return err
	}

	var created project.Project
	if err := r.db.Where("project_name = ? AND g_id = ?", p.ProjectName, p.GID).
		Order("create_at DESC").
		First(&created).Error; err != nil {
		return err
	}

	p.PID = created.PID
	return nil
}

func (r *DBProjectRepo) UpdateProject(p *project.Project) error {
	return r.db.Save(p).Error
}

func (r *DBProjectRepo) DeleteProject(id uint) error {
	return r.db.Delete(&project.Project{}, id).Error
}

func (r *DBProjectRepo) ListProjects() ([]project.Project, error) {
	var projects []project.Project
	err := r.db.Find(&projects).Error
	return projects, err
}

func (r *DBProjectRepo) ListProjectsByGroup(id uint) ([]project.Project, error) {
	var projects []project.Project
	if err := r.db.Where("g_id = ?", id).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *DBProjectRepo) ListProjectsByUserID(userID uint) ([]view.ProjectUserView, error) {
	var results []view.ProjectUserView

	err := r.db.Table("project_list p").
		Select(`
            p.p_id, p.project_name, 
            g.g_id, g.group_name, 
            u.u_id, u.username, ug.role
        `).
		Joins("JOIN group_list g ON p.g_id = g.g_id").
		Joins("JOIN user_group ug ON ug.g_id = g.g_id").
		Joins("JOIN users u ON u.u_id = ug.u_id").
		Where("u.u_id = ?", userID).
		Scan(&results).Error

	return results, err
}

func (r *DBProjectRepo) GetAllProjectGroupViews() ([]view.ProjectGroupView, error) {
	var results []view.ProjectGroupView

	err := r.db.Table("group_list g").
		Select(`
            g.g_id, 
            g.group_name, 
            COUNT(DISTINCT p.p_id) AS project_count, 
            COUNT(r.r_id) AS resource_count, 
            MAX(g.create_at) AS group_create_at, 
            MAX(g.update_at) AS group_update_at
        `).
		Joins("LEFT JOIN project_list p ON p.g_id = g.g_id").
		Joins("LEFT JOIN config_files cf ON cf.project_id = p.p_id").
		Joins("LEFT JOIN resource_list r ON r.cf_id = cf.cf_id").
		Group("g.g_id, g.group_name").
		Scan(&results).Error

	return results, err
}

func (r *DBProjectRepo) WithTx(tx *gorm.DB) ProjectRepo {
	if tx == nil {
		return r
	}
	return &DBProjectRepo{
		db: tx,
	}
}
