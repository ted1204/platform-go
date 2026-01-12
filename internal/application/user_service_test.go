package application

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/internal/api/middleware"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/internal/repository/mock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// --------------------- Setup ---------------------
func setupUserServiceMocks(t *testing.T) (*UserService, *mock.MockUserRepo) {
	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	mockUser := mock.NewMockUserRepo(ctrl)
	repos := &repository.Repos{
		User: mockUser,
	}
	svc := NewUserService(repos)
	return svc, mockUser
}

// --------------------- RegisterUser ---------------------
func TestRegisterUser_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	input := user.CreateUserInput{
		Username: "alice",
		Password: "123456",
		Email:    ptrString("alice@test.com"),
		FullName: ptrString("Alice"),
	}

	mockUser.EXPECT().GetUserByUsername("alice").Return(user.User{}, gorm.ErrRecordNotFound)
	mockUser.EXPECT().SaveUser(gomock.Any()).Return(nil)

	err := svc.RegisterUser(input)
	assert.NoError(t, err)
}

func TestRegisterUser_UsernameTaken(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	mockUser.EXPECT().GetUserByUsername("admin").Return(user.User{UID: 1}, nil)

	input := user.CreateUserInput{Username: "admin", Password: "123456"}
	err := svc.RegisterUser(input)
	assert.Equal(t, ErrUsernameTaken, err)
}

// --------------------- LoginUser ---------------------
func TestLoginUser_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	password := "123456"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	usr := user.User{UID: 1, Username: "bob", Password: string(hashed)}

	mockUser.EXPECT().GetUserByUsername("bob").Return(usr, nil)

	oldGen := middleware.GenerateToken
	middleware.GenerateToken = func(uid uint, username string, exp time.Duration, view repository.UserGroupRepo) (string, bool, error) {
		return "token123", true, nil
	}
	defer func() { middleware.GenerateToken = oldGen }()

	u, token, isAdmin, err := svc.LoginUser("bob", "123456")
	assert.NoError(t, err)
	assert.Equal(t, "bob", u.Username)
	assert.Equal(t, "token123", token)
	assert.True(t, isAdmin)
}

func TestLoginUser_InvalidPassword(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	password := "123456"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	usr := user.User{UID: 1, Username: "bob", Password: string(hashed)}

	mockUser.EXPECT().GetUserByUsername("bob").Return(usr, nil)

	u, token, isAdmin, err := svc.LoginUser("bob", "wrong")
	assert.Error(t, err)
	assert.Equal(t, user.User{}, u)
	assert.Empty(t, token)
	assert.False(t, isAdmin)
}

func TestLoginUser_UserNotFound(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)
	mockUser.EXPECT().GetUserByUsername("notexist").Return(user.User{}, errors.New("not found"))

	u, token, isAdmin, err := svc.LoginUser("notexist", "123")
	assert.Error(t, err)
	assert.Equal(t, user.User{}, u)
	assert.Empty(t, token)
	assert.False(t, isAdmin)
}

// --------------------- UpdateUser ---------------------
func TestUpdateUser_SuccessChangePassword(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	oldPass := "oldpass"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(oldPass), bcrypt.DefaultCost)
	existing := user.User{UID: 1, Password: string(hashed)}

	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(existing, nil)
	mockUser.EXPECT().SaveUser(gomock.Any()).Return(nil)

	newPass := "newpass"
	input := user.UpdateUserInput{
		OldPassword: &oldPass,
		Password:    &newPass,
	}

	updated, err := svc.UpdateUser(1, input)
	assert.NoError(t, err)
	assert.NotEqual(t, existing.Password, updated.Password)
}

func TestUpdateUser_WrongOldPassword(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	oldPass := "oldpass"
	hashed, _ := bcrypt.GenerateFromPassword([]byte(oldPass), bcrypt.DefaultCost)
	existing := user.User{UID: 1, Password: string(hashed)}

	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(existing, nil)

	wrongPass := "wrong"
	input := user.UpdateUserInput{OldPassword: &wrongPass, Password: &wrongPass}

	updated, err := svc.UpdateUser(1, input)
	assert.ErrorIs(t, err, ErrIncorrectPassword)
	assert.Equal(t, user.User{}, updated)
}

func TestUpdateUser_UserNotFound(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)
	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(user.User{}, errors.New("not found"))

	input := user.UpdateUserInput{FullName: ptrString("NewName")}
	updated, err := svc.UpdateUser(1, input)
	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Equal(t, user.User{}, updated)
}

// --------------------- RemoveUser ---------------------
func TestRemoveUser_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)
	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(user.User{Username: "testuser"}, nil)
	mockUser.EXPECT().DeleteUser(uint(1)).Return(nil)

	err := svc.RemoveUser(1)
	assert.NoError(t, err)
}

func TestRemoveUser_Fail(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)
	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(user.User{Username: "testuser"}, nil)
	mockUser.EXPECT().DeleteUser(uint(1)).Return(errors.New("delete fail"))

	err := svc.RemoveUser(1)
	assert.EqualError(t, err, "delete fail")
}

// --------------------- ListUsers ---------------------
func TestListUsers_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	users := []user.UserWithSuperAdmin{
		{UID: 1, Username: "alice"},
		{UID: 2, Username: "bob"},
	}
	mockUser.EXPECT().GetAllUsers().Return(users, nil)

	result, err := svc.ListUsers()
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

// --------------------- ListUserByPaging ---------------------
func TestListUserByPaging_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	users := []user.UserWithSuperAdmin{
		{UID: 1, Username: "alice"},
	}
	mockUser.EXPECT().ListUsersPaging(1, 10).Return(users, nil)

	result, err := svc.ListUserByPaging(1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

// --------------------- FindUserByID ---------------------
func TestFindUserByID_Success(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	usr := user.UserWithSuperAdmin{UID: 1, Username: "alice"}
	mockUser.EXPECT().GetUserByID(uint(1)).Return(usr, nil)

	result, err := svc.FindUserByID(1)
	assert.NoError(t, err)
	assert.Equal(t, "alice", result.Username)
}

func TestFindUserByID_NotFound(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	mockUser.EXPECT().GetUserByID(uint(999)).Return(user.UserWithSuperAdmin{}, errors.New("not found"))

	_, err := svc.FindUserByID(999)
	assert.Error(t, err)
}

// --------------------- UpdateUser (More cases) ---------------------
func TestUpdateUser_SuccessNoPasswordChange(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	oldEmail := "old@test.com"
	existing := user.User{UID: 1, Username: "alice", Email: &oldEmail}
	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(existing, nil)

	// Expect SaveUser with updated email
	mockUser.EXPECT().SaveUser(gomock.Any()).DoAndReturn(func(u *user.User) error {
		assert.Equal(t, "new@test.com", *u.Email)
		return nil
	})

	input := user.UpdateUserInput{Email: ptrString("new@test.com")}
	updated, err := svc.UpdateUser(1, input)
	assert.NoError(t, err)
	assert.Equal(t, "new@test.com", *updated.Email)
}

func TestUpdateUser_FailSave(t *testing.T) {
	svc, mockUser := setupUserServiceMocks(t)

	existing := user.User{UID: 1}
	mockUser.EXPECT().GetUserRawByID(uint(1)).Return(existing, nil)
	mockUser.EXPECT().SaveUser(gomock.Any()).Return(errors.New("db error"))

	input := user.UpdateUserInput{Email: ptrString("new@test.com")}
	_, err := svc.UpdateUser(1, input)
	assert.Error(t, err)
}

// --------------------- Helper ---------------------
func ptrString(s string) *string { return &s }
