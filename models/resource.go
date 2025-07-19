package models

import (
	"time"

	"gorm.io/datatypes"
)

type ResourceType string

const (
	ResourcePod        ResourceType = "pod"
	ResourceService    ResourceType = "service"
	ResourceDeployment ResourceType = "deployment"
	ResourceConfigMap  ResourceType = "configmap"
	ResourceIngress    ResourceType = "ingress"
)

type Resource struct {
	RID         uint           `gorm:"primaryKey;column:r_id"`
	CFID        uint           `gorm:"not null;column:cf_id"`
	Type        string         `gorm:"type:resource_type;not null"`
	Name        string         `gorm:"size:50;not null"`
	ParsedYAML  datatypes.JSON `gorm:"type:jsonb;not null"`
	Description *string        `gorm:"type:text"`
	CreatedAt   time.Time      `gorm:"column:create_at;autoCreateTime"`
}
