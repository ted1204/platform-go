package models

type ProjectGroupView struct {
    GID           uint
    GroupName     string
    ProjectCount  int64
    ResourceCount int64
    GroupCreateAt string
    GroupUpdateAt string
}

type ProjectResourceView struct {
    PID              uint
    ProjectName      string
    RID              uint
    Type             string
    Name             string
    Filename         string
    ResourceCreateAt string
    ResourceUpdateAt string
}

type UserGroupView struct {
    UID       uint
    Username  string
    GID       uint
    GroupName string
    Role      string
}
