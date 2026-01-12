package image

import (
	"time"

	"gorm.io/gorm"
)

type ContainerRepository struct {
	gorm.Model
	Registry  string         `gorm:"size:255;default:'docker.io'"`
	Namespace string         `gorm:"size:255"`
	Name      string         `gorm:"size:255;index"`
	FullName  string         `gorm:"uniqueIndex;size:512"`
	Tags      []ContainerTag `gorm:"foreignKey:RepositoryID"`
}

type ContainerTag struct {
	gorm.Model
	RepositoryID uint   `gorm:"index;not null"`
	Name         string `gorm:"size:128;index"`
	Digest       string `gorm:"size:255"`
	Size         int64
	PushedAt     *time.Time
}

type ImageAllowList struct {
	gorm.Model
	ProjectID    *uint `gorm:"index"`
	TagID        *uint `gorm:"index"`
	RepositoryID uint  `gorm:"index;not null"`
	RequestID    *uint
	CreatedBy    uint
	IsEnabled    bool                `gorm:"default:true"`
	Repository   ContainerRepository `gorm:"foreignKey:RepositoryID"`
	Tag          ContainerTag        `gorm:"foreignKey:TagID"`
}

type ImageRequest struct {
	gorm.Model
	UserID         uint  `gorm:"index"`
	ProjectID      *uint `gorm:"index"`
	InputRegistry  string
	InputImageName string
	InputTag       string
	Status         string `gorm:"size:32;default:'pending';index"`
	ReviewerID     *uint
	ReviewedAt     *time.Time
	ReviewerNote   string
}

type ClusterImageStatus struct {
	gorm.Model
	TagID        uint `gorm:"uniqueIndex"`
	IsPulled     bool `gorm:"default:false"`
	LastPulledAt *time.Time
	PullError    string
}
