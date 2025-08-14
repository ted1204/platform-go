package dto

type GroupProjects struct {
	GroupName string              `json:"group_name"`
	Projects  []SimpleProjectInfo `json:"projects"`
}

type SimpleProjectInfo struct {
	PID         uint   `json:"p_id"`
	ProjectName string `json:"project_name"`
}

type GroupProjectsWithID struct {
	GID       uint                `json:"g_id"`
	GroupName string              `json:"group_name"`
	Projects  []SimpleProjectInfo `json:"projects"`
}

type UserGroups struct {
	UID      uint
	Username string
	Groups   []GroupInfo
}

type GroupInfo struct {
	GID       uint
	GroupName string
	Role      string
}
