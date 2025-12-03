package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/linskybing/platform-go/src/response"
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

func TestRegister(t *testing.T) {
	username := "newuser"
	password := "123456"
	reqBody := map[string]string{"username": username, "password": password, "email": "new@test.com"}
	resp := doRequest(t, "POST", "/register", "", reqBody, http.StatusCreated)
	require.Contains(t, resp.Body.String(), "success")

	// Verify login works
	token := loginUser(t, username, password)
	require.NotEmpty(t, token)
}

func TestGetUserByID(t *testing.T) {
	token := loginUser(t, "alice", "123456")

	// Get ID of alice
	respList := doRequest(t, "GET", "/users", token, nil, http.StatusOK)
	var listResult []struct {
		UID      int    `json:"uid"`
		Username string `json:"username"`
	}
	err := json.Unmarshal(respList.Body.Bytes(), &listResult)
	require.NoError(t, err)

	var aliceID int
	for _, u := range listResult {
		if u.Username == "alice" {
			aliceID = u.UID
			break
		}
	}
	require.NotZero(t, aliceID)

	resp := doRequest(t, "GET", fmt.Sprintf("/users/%d", aliceID), token, nil, http.StatusOK)
	require.Contains(t, resp.Body.String(), "alice")
}

func TestUpdateUser(t *testing.T) {
	token := loginUser(t, "test1", "123456")

	// Get ID of test1
	respList := doRequest(t, "GET", "/users", token, nil, http.StatusOK)
	var listResult []struct {
		UID      int    `json:"uid"`
		Username string `json:"username"`
	}
	json.Unmarshal(respList.Body.Bytes(), &listResult)
	var test1ID int
	for _, u := range listResult {
		if u.Username == "test1" {
			test1ID = u.UID
			break
		}
	}

	newEmail := "updated@test.com"
	reqBody := map[string]string{"email": newEmail}
	doRequest(t, "PUT", fmt.Sprintf("/users/%d", test1ID), token, reqBody, http.StatusOK)

	// Verify update
	resp := doRequest(t, "GET", fmt.Sprintf("/users/%d", test1ID), token, nil, http.StatusOK)
	require.Contains(t, resp.Body.String(), newEmail)
}

func TestDeleteUser(t *testing.T) {
	// Create a user to delete
	registerUserForTests("todelete", "123456")
	token := loginUser(t, "todelete", "123456")

	// Get ID
	respList := doRequest(t, "GET", "/users", token, nil, http.StatusOK)
	var listResult []struct {
		UID      int    `json:"uid"`
		Username string `json:"username"`
	}
	json.Unmarshal(respList.Body.Bytes(), &listResult)
	var id int
	for _, u := range listResult {
		if u.Username == "todelete" {
			id = u.UID
			break
		}
	}

	// Delete self
	doRequest(t, "DELETE", fmt.Sprintf("/users/%d", id), token, nil, http.StatusNoContent)

	// Verify login fails
	form := url.Values{}
	form.Add("username", "todelete")
	form.Add("password", "123456")
	doRequest(t, "POST", "/login", "", form, http.StatusUnauthorized)
}
