package repository

import (
	"gorm.io/gorm"
)

type Repos struct {
	ConfigFile ConfigFileRepo
	Group      GroupRepo
	Project    ProjectRepo
	Resource   ResourceRepo
	UserGroup  UserGroupRepo
	User       UserRepo
	Audit      AuditRepo
	Form       FormRepo
	Job        JobRepo
	Image      ImageRepo

	db *gorm.DB
}

func NewRepositories(db *gorm.DB) *Repos {
	return &Repos{
		ConfigFile: NewConfigFileRepo(db),
		Group:      NewGroupRepo(db),
		Project:    NewProjectRepo(db),
		Resource:   NewResourceRepo(db),
		UserGroup:  NewUserGroupRepo(db),
		User:       NewUserRepo(db),
		Audit:      NewAuditRepo(db),
		Form:       NewFormRepo(db),
		Job:        NewJobRepo(db),
		Image:      NewImageRepo(db),
		db:         db,
	}
}

func (r *Repos) Begin() *gorm.DB {
	return r.db.Begin()
}

func (r *Repos) WithTx(tx *gorm.DB) *Repos {
	return &Repos{
		ConfigFile: r.ConfigFile.WithTx(tx),
		Group:      r.Group.WithTx(tx),
		Project:    r.Project.WithTx(tx),
		Resource:   r.Resource.WithTx(tx),
		UserGroup:  r.UserGroup.WithTx(tx),
		User:       r.User.WithTx(tx),
		Audit:      r.Audit.WithTx(tx),
		Form:       r.Form.WithTx(tx),
		Job:        r.Job.WithTx(tx),
		Image:      r.Image.WithTx(tx),
		db:         tx,
	}
}

func (r *Repos) ExecTx(fn func(*Repos) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepos := r.WithTx(tx)
		return fn(txRepos)
	})
}
