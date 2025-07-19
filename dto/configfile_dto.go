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

func (d CreateConfigFileInput) GetGID() uint {
	gId, _ := repositories.GetGroupIDByProjectID(d.ProjectID)
	return gId
}
