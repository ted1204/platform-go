package db

import (
	"fmt"
	"log"

	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func createEnums() {
	enums := []string{
		`DO $$ BEGIN CREATE TYPE user_type AS ENUM ('origin', 'oauth2'); EXCEPTION WHEN duplicate_object THEN null; END $$;`,
		`DO $$ BEGIN CREATE TYPE user_status AS ENUM ('online', 'offline', 'delete'); EXCEPTION WHEN duplicate_object THEN null; END $$;`,
		`DO $$ BEGIN CREATE TYPE user_role AS ENUM ('admin', 'manager', 'user'); EXCEPTION WHEN duplicate_object THEN null; END $$;`,
		`DO $$ BEGIN CREATE TYPE resource_type AS ENUM ('cpu', 'memory', 'gpu'); EXCEPTION WHEN duplicate_object THEN null; END $$;`,
	}

	for _, enum := range enums {
		if err := DB.Exec(enum).Error; err != nil {
			log.Printf("Failed to create enum: %s, error: %v", enum, err)
		}
	}
}

func Init() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DbHost,
		config.DbPort,
		config.DbUser,
		config.DbPassword,
		config.DbName,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	dropViews()

	// Create enums
	createEnums()

	if err := DB.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.UserGroup{},
		&models.Project{},
		&models.ConfigFile{},
		&models.Resource{},
		&models.AuditLog{},
		&models.Form{},
		&models.Job{},
		&models.GPURequest{},
	); err != nil {
		log.Fatal("Failed to auto migrate:", err)
	}

	createViews()

	log.Println("Database connected and migrated")
}

func InitWithGormDB(gormDB *gorm.DB) {
	DB = gormDB
}

func dropViews() {
	views := []string{
		"project_group_views",
		"project_resource_views",
		"group_resource_views",
		"users_with_superadmin",
		"user_group_views",
		"project_user_views",
	}

	for _, view := range views {
		if err := DB.Exec(fmt.Sprintf("DROP VIEW IF EXISTS %s CASCADE", view)).Error; err != nil {
			log.Printf("Failed to drop view %s: %v", view, err)
		}
	}
}

func createViews() {
	views := []string{
		`CREATE OR REPLACE VIEW project_group_views AS
		SELECT
		g.g_id,
		g.group_name,
		COUNT(DISTINCT p.p_id) AS project_count,
		COUNT(r.r_id) AS resource_count,
		MAX(g.create_at) AS group_create_at,
		MAX(g.update_at) AS group_update_at
		FROM group_list g
		LEFT JOIN projects p ON p.g_id = g.g_id
		LEFT JOIN config_files cf ON cf.project_id = p.p_id
		LEFT JOIN resources r ON r.cf_id = cf.cf_id
		GROUP BY g.g_id, g.group_name;`,

		`CREATE OR REPLACE VIEW project_resource_views AS
		SELECT
		p.p_id,
		p.project_name,
		r.r_id,
		r.type,
		r.name,
		cf.filename,
		r.create_at AS resource_create_at
		FROM projects p
		JOIN config_files cf ON cf.project_id = p.p_id
		JOIN resources r ON r.cf_id = cf.cf_id;`,

		`CREATE OR REPLACE VIEW group_resource_views AS
		SELECT
		g.g_id,
		g.group_name,
		p.p_id,
		p.project_name,
		r.r_id,
		r.type AS resource_type,
		r.name AS resource_name,
		cf.filename,
		r.create_at AS resource_create_at
		FROM group_list g
		LEFT JOIN projects p ON p.g_id = g.g_id
		LEFT JOIN config_files cf ON cf.project_id = p.p_id
		LEFT JOIN resources r ON r.cf_id = cf.cf_id
		WHERE r.r_id IS NOT NULL;`,

		`CREATE OR REPLACE VIEW user_group_views AS
		SELECT
		u.u_id,
		u.username,
		g.g_id,
		g.group_name,
		ug.role
		FROM users u
		JOIN user_group ug ON u.u_id = ug.u_id
		JOIN group_list g ON ug.g_id = g.g_id;`,

		`CREATE OR REPLACE VIEW users_with_superadmin AS
		SELECT
		u.u_id,
		u.username,
		u.password,
		u.email,
		u.full_name,
		u.type,
		u.status,
		u.create_at,
		u.update_at,
		CASE WHEN ug.role = 'admin' AND ug.group_name = 'super' THEN true ELSE false END AS is_super_admin
		FROM users u
		LEFT JOIN user_group_views ug ON u.u_id = ug.u_id AND ug.group_name = 'super' AND ug.role = 'admin';`,

		`CREATE OR REPLACE VIEW project_user_views AS
		SELECT
		p.p_id,
		p.project_name,
		g.g_id,
		g.group_name,
		u.u_id,
		u.username
		FROM projects p
		JOIN group_list g ON p.g_id = g.g_id
		JOIN user_group ug ON ug.g_id = g.g_id
		JOIN users u ON u.u_id = ug.u_id;`,
	}

	for _, view := range views {
		if err := DB.Exec(view).Error; err != nil {
			log.Printf("Failed to create view: %v", err)
		}
	}
}
