package dto

type ConfigFileUpdateDTO struct {
	Filename *string `form:"filename"`
	RawYaml  *string `form:"raw_yaml"`
}

type CreateConfigFileInput struct {
	Filename  string `form:"filename"`
	RawYaml   string `form:"raw_yaml"`
	ProjectID uint   `form:"project_id"`
}
