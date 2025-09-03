package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/linskybing/platform-go/response"
	"github.com/stretchr/testify/require"
)

func loginUser(t *testing.T, username, password string) string {
	form := url.Values{}
	form.Add("username", username)
	form.Add("password", password)

	resp := doRequest(t, "POST", "/login", "", form, http.StatusOK)

	var result response.TokenResponse
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	return result.Token
}

func registerUser(t *testing.T, username, password string) {
	reqBody := map[string]string{"username": username, "password": password}
	resp := doRequest(t, "POST", "/register", "", reqBody, http.StatusCreated)
	require.Contains(t, resp.Body.String(), "success")
}

func TestLogin(t *testing.T) {
	form := url.Values{}
	form.Add("username", "alice")
	form.Add("password", "123456")

	resp := doRequest(t, "POST", "/login", "", form, http.StatusOK)

	var result response.TokenResponse
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Equal(t, "alice", result.Username)
	require.NotEmpty(t, result.Token)
}

func TestGetUsers(t *testing.T) {
	token := loginUser(t, "alice", "123456")
	resp := doRequest(t, "GET", "/users", token, nil, http.StatusOK)
	require.Contains(t, resp.Body.String(), "alice")
}

func TestListUsersPaging(t *testing.T) {
	token := loginUser(t, "bob", "123456")

	resp := doRequest(t, "GET", "/users/paging?page=1&limit=10", token, nil, http.StatusOK)

	var result struct {
		Data []struct {
			UID      int    `json:"uid"`
			Username string `json:"username"`
			IsAdmin  bool   `json:"is_super_admin"`
		} `json:"data"`
	}
	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	require.NotEmpty(t, result.Data)

	usernames := []string{}
	for _, u := range result.Data {
		usernames = append(usernames, u.Username)
	}
	require.Contains(t, usernames, "bob")
}
