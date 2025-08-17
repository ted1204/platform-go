package services

import (
	"errors"
	"time"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/middleware"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrIncorrectPassword   = errors.New("old password is incorrect")
	ErrMissingOldPassword  = errors.New("old password is required to change password")
	ErrPasswordHashFailure = errors.New("failed to hash new password")
	ErrUsernameTaken       = errors.New("username already taken")
)

type UserService struct {
	Repos *repositories.Repos
}

func NewUserService(repos *repositories.Repos) *UserService {
	return &UserService{
		Repos: repos,
	}
}

func (s *UserService) RegisterUser(input dto.CreateUserInput) error {
	var existing models.User
	err := db.DB.Where("username = ?", input.Username).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		return ErrUsernameTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return ErrPasswordHashFailure
	}

	user := models.User{
		Username: input.Username,
		Password: string(hashed),
		Email:    input.Email,
		FullName: input.FullName,
		Type:     "origin",
		Status:   "offline",
	}

	if input.Type != nil {
		user.Type = *input.Type
	}
	if input.Status != nil {
		user.Status = *input.Status
	}

	return db.DB.Create(&user).Error
}

func (s *UserService) LoginUser(username, password string) (models.User, string, bool, error) {
	var user models.User
	if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return user, "", false, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return user, "", false, errors.New("invalid credentials")
	}

	token, isAdmin, err := middleware.GenerateToken(user.UID, user.Username, 24*time.Hour, s.Repos.View)
	if err != nil {
		return user, "", false, err
	}

	return user, token, isAdmin, nil
}

func (s *UserService) ListUsers() ([]models.UserWithSuperAdmin, error) {
	return s.Repos.User.GetAllUsers()
}

func (s *UserService) ListUserByPaging(page, limit int) ([]models.UserWithSuperAdmin, error) {
	return s.Repos.User.ListUsersPaging(page, limit)
}

func (s *UserService) FindUserByID(id uint) (models.UserWithSuperAdmin, error) {
	return s.Repos.User.GetUserByID(id)
}

func (s *UserService) UpdateUser(id uint, input dto.UpdateUserInput) (models.User, error) {
	user, err := s.Repos.User.GetUserRawByID(id)
	if err != nil {
		return models.User{}, ErrUserNotFound
	}

	if input.Password != nil {
		if input.OldPassword == nil {
			return models.User{}, ErrMissingOldPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(*input.OldPassword)); err != nil {
			return models.User{}, ErrIncorrectPassword
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return models.User{}, ErrPasswordHashFailure
		}
		user.Password = string(hashed)
	}

	if input.Type != nil {
		user.Type = string(*input.Type) // enum to string
	}
	if input.Status != nil {
		user.Status = string(*input.Status) // enum to string
	}
	if input.Email != nil {
		user.Email = input.Email
	}
	if input.FullName != nil {
		user.FullName = input.FullName
	}

	if err := s.Repos.User.SaveUser(&user); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *UserService) RemoveUser(id uint) error {
	return s.Repos.User.DeleteUser(id)
}
