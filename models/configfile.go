package models

import "time"

type ConfigFile struct {
	CFID      uint      `gorm:"primaryKey;column:cf_id"`
	Filename  string    `gorm:"size:200;not null"`
	MinIOPath string    `gorm:"column:minio_path;size:300;not null"`
	ProjectID uint      `gorm:"not null"`
	CreatedAt time.Time `gorm:"column:create_at"`
}
