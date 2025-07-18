package response

import "github.com/linskybing/platform-go/models"

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	Token    string `json:"token"`
	UID      uint   `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type GroupResponse struct {
	Message string       `json:"message"`
	Group   models.Group `json:"group"`
}
