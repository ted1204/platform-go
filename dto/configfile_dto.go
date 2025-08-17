package dto

import "github.com/linskybing/platform-go/repositories"

type ConfigFileUpdateDTO struct {
	Filename *string `form:"filename"`
	RawYaml  *string `form:"raw_yaml"`
}

type CreateConfigFileInput struct {
	Filename  string `form:"filename"`
	RawYaml   string `form:"raw_yaml"`
	ProjectID uint   `form:"project_id"`
}

type ProjectGetter interface {
	GetGroupIDByProjectID(projectID uint) uint
}

func (d CreateConfigFileInput) GetGIDByRepo(repo *repositories.Repos) uint {
	gId, _ := repo.Project.GetGroupIDByProjectID(d.ProjectID)
	return gId
}
