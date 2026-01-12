package repository

import (
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/internal/domain/view"
	"gorm.io/gorm"
)

type UserRepo interface {
	GetAllUsers() ([]user.UserWithSuperAdmin, error)
	ListUsersPaging(page, limit int) ([]user.UserWithSuperAdmin, error)
	GetUserByID(id uint) (user.UserWithSuperAdmin, error)
	GetUsernameByID(id uint) (string, error)
	GetUserByUsername(username string) (user.User, error)
	GetUserRawByID(id uint) (user.User, error)
	SaveUser(user *user.User) error
	DeleteUser(id uint) error
	ListUsersByProjectID(projectID uint) ([]view.ProjectUserView, error)
	WithTx(tx *gorm.DB) UserRepo
}

type DBUserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *DBUserRepo {
	return &DBUserRepo{
		db: db,
	}
}

func (r *DBUserRepo) GetAllUsers() ([]user.UserWithSuperAdmin, error) {
	var users []user.UserWithSuperAdmin

	err := r.db.Table("users u").
		Select(`
			u.*,
			CASE WHEN ug.role = 'admin' THEN true ELSE false END AS is_super_admin
		`).
		Joins("LEFT JOIN user_group ug ON u.u_id = ug.u_id AND ug.role = 'admin'").
		Joins("LEFT JOIN group_list g ON ug.g_id = g.g_id AND g.group_name = 'super'").
		Scan(&users).Error

	return users, err
}

func (r *DBUserRepo) GetUserByUsername(username string) (user.User, error) {
	var u user.User
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		return u, err
	}
	return u, nil
}

func (r *DBUserRepo) ListUsersPaging(page, limit int) ([]user.UserWithSuperAdmin, error) {
	var users []user.UserWithSuperAdmin

	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	if err := r.db.Offset(int(offset)).Limit(int(limit)).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *DBUserRepo) GetUserByID(id uint) (user.UserWithSuperAdmin, error) {
	var u user.UserWithSuperAdmin
	if err := r.db.Table("users").Where("u_id = ?", id).First(&u).Error; err != nil {
		return u, err
	}
	return u, nil
}

func (r *DBUserRepo) GetUsernameByID(id uint) (string, error) {
	var username string
	err := r.db.Model(&user.User{}).Select("username").Where("u_id = ?", id).First(&username).Error
	if err != nil {
		return "", err
	}
	return username, nil
}

func (r *DBUserRepo) GetUserRawByID(id uint) (user.User, error) {
	var u user.User
	if err := r.db.First(&u, id).Error; err != nil {
		return u, err
	}
	return u, nil
}

func (r *DBUserRepo) SaveUser(user *user.User) error {
	return r.db.Save(user).Error
}

func (r *DBUserRepo) DeleteUser(id uint) error {
	return r.db.Delete(&user.User{}, id).Error
}

func (r *DBUserRepo) ListUsersByProjectID(projectID uint) ([]view.ProjectUserView, error) {
	var results []view.ProjectUserView

	err := r.db.Table("users u").
		Select(`
            p.p_id, p.project_name, 
            g.g_id, g.group_name, 
            u.u_id, u.username, ug.role
        `).
		Joins("JOIN user_group ug ON ug.u_id = u.u_id").
		Joins("JOIN group_list g ON g.g_id = ug.g_id").
		Joins("JOIN project_list p ON p.g_id = g.g_id").
		Where("p.p_id = ?", projectID).
		Scan(&results).Error

	return results, err
}

func (r *DBUserRepo) WithTx(tx *gorm.DB) UserRepo {
	if tx == nil {
		return r
	}
	return &DBUserRepo{
		db: tx,
	}
}
