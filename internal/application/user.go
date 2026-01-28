package application

import (
	"errors"
	"time"

	"github.com/linskybing/platform-go/internal/api/middleware"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrIncorrectPassword   = errors.New("old password is incorrect")
	ErrMissingOldPassword  = errors.New("old password is required to change password")
	ErrPasswordHashFailure = errors.New("failed to hash new password")
	ErrUsernameTaken       = errors.New("username already taken")
	ErrReservedAdminUser   = errors.New("cannot delete or downgrade reserved admin user '" + config.ReservedAdminUsername + "'")
)

type UserService struct {
	Repos *repository.Repos
}

func NewUserService(repos *repository.Repos) *UserService {
	return &UserService{
		Repos: repos,
	}
}

func (s *UserService) RegisterUser(input user.CreateUserInput) error {
	_, err := s.Repos.User.GetUserByUsername(input.Username)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == nil {
		return ErrUsernameTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return ErrPasswordHashFailure
	}

	usr := user.User{
		Username: input.Username,
		Password: string(hashed),
		Email:    input.Email,
		FullName: input.FullName,
		Type:     "origin",
		Status:   "offline",
	}

	if input.Type != nil {
		usr.Type = *input.Type
	}
	if input.Status != nil {
		usr.Status = *input.Status
	}
	return s.Repos.User.SaveUser(&usr)
}

func (s *UserService) LoginUser(username, password string) (user.User, string, bool, error) {
	usr, err := s.Repos.User.GetUserByUsername(username)
	if err != nil {
		return user.User{}, "", false, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(password)); err != nil {
		return user.User{}, "", false, errors.New("invalid credentials")
	}

	token, isAdmin, err := middleware.GenerateToken(usr.UID, usr.Username, 24*time.Hour, s.Repos.UserGroup)
	if err != nil {
		return user.User{}, "", false, err
	}

	return usr, token, isAdmin, nil
}

func (s *UserService) ListUsers() ([]user.UserWithSuperAdmin, error) {
	return s.Repos.User.GetAllUsers()
}

func (s *UserService) ListUserByPaging(page, limit int) ([]user.UserWithSuperAdmin, error) {
	return s.Repos.User.ListUsersPaging(page, limit)
}

func (s *UserService) FindUserByID(id uint) (user.UserWithSuperAdmin, error) {
	return s.Repos.User.GetUserByID(id)
}

func (s *UserService) UpdateUser(id uint, input user.UpdateUserInput) (user.User, error) {
	usr, err := s.Repos.User.GetUserRawByID(id)
	if err != nil {
		return user.User{}, ErrUserNotFound
	}

	// Prevent downgrading the reserved admin user
	if usr.Username == config.ReservedAdminUsername && input.Type != nil && *input.Type != "admin" {
		return user.User{}, ErrReservedAdminUser
	}

	if input.Password != nil {
		if input.OldPassword == nil {
			return user.User{}, ErrMissingOldPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(*input.OldPassword)); err != nil {
			return user.User{}, ErrIncorrectPassword
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return user.User{}, ErrPasswordHashFailure
		}
		usr.Password = string(hashed)
	}

	if input.Type != nil {
		usr.Type = string(*input.Type)
	}
	if input.Status != nil {
		usr.Status = string(*input.Status)
	}
	if input.Email != nil {
		usr.Email = input.Email
	}
	if input.FullName != nil {
		usr.FullName = input.FullName
	}

	if err := s.Repos.User.SaveUser(&usr); err != nil {
		return user.User{}, err
	}
	return usr, nil
}

func (s *UserService) ForgotPassword(username, newPassword string) error {
	usr, err := s.Repos.User.GetUserByUsername(username)
	if err != nil {
		return ErrUserNotFound
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return ErrPasswordHashFailure
	}

	usr.Password = string(hashed)
	return s.Repos.User.SaveUser(&usr)
}

func (s *UserService) RemoveUser(id uint) error {
	usr, err := s.Repos.User.GetUserRawByID(id)
	if err != nil {
		return ErrUserNotFound
	}

	// Prevent deleting the reserved admin user
	if usr.Username == config.ReservedAdminUsername {
		return ErrReservedAdminUser
	}

	return s.Repos.User.DeleteUser(id)
}
