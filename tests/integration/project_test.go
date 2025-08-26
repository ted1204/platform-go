package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/response"
	"github.com/stretchr/testify/require"
)

func TestProjectFlow_UserCreatesProject(t *testing.T) {
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	// Admin creates a group
	groupForm := url.Values{}
	groupForm.Add("group_name", "project_team")
	groupForm.Add("description", "group for project test")
	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)

	var group models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &group)
	require.NoError(t, err)

	// Admin adds user to the group
	userGroupForm := url.Values{}
	userGroupForm.Add("u_id", "2") // alice UID
	userGroupForm.Add("g_id", fmt.Sprintf("%d", group.GID))
	userGroupForm.Add("role", "manager")

	resp = doRequest(t, "POST", "/user-group", adminToken, userGroupForm, http.StatusCreated)
	var userGroup models.UserGroup
	err = json.Unmarshal(resp.Body.Bytes(), &userGroup)
	require.NoError(t, err)
	require.Equal(t, "manager", userGroup.Role)

	// User creates a project
	projectForm := url.Values{}
	projectForm.Add("project_name", "user_project")
	projectForm.Add("description", "created by user")
	projectForm.Add("g_id", fmt.Sprintf("%d", group.GID))

	resp = doRequest(t, "POST", "/projects", userToken, projectForm, http.StatusCreated)
	var project models.Project
	err = json.Unmarshal(resp.Body.Bytes(), &project)
	require.NoError(t, err)
	require.Equal(t, "user_project", project.ProjectName)
	require.Equal(t, group.GID, project.GID)

	// User gets project by ID
	urlPath := fmt.Sprintf("/projects/%d", project.PID)
	resp = doRequest(t, "GET", urlPath, userToken, nil, http.StatusOK)
	var fetchedProject models.Project
	err = json.Unmarshal(resp.Body.Bytes(), &fetchedProject)
	require.NoError(t, err)
	require.Equal(t, project.PID, fetchedProject.PID)

	// User updates project
	updateForm := url.Values{}
	updateForm.Add("description", "updated by user")
	resp = doRequest(t, "PUT", urlPath, userToken, updateForm, http.StatusOK)

	var updatedProject models.Project
	err = json.Unmarshal(resp.Body.Bytes(), &updatedProject)
	require.NoError(t, err)
	require.Equal(t, "updated by user", updatedProject.Description)

	// Another normal user attempts to update project -> 403 Forbidden
	otherUserToken := loginUser(t, "bob", "123456")
	require.NotEmpty(t, otherUserToken)

	resp = doRequest(t, "PUT", urlPath, otherUserToken, updateForm, http.StatusForbidden)

	// Get projects by user
	resp = doRequest(t, "GET", "/projects/by-user", userToken, nil, http.StatusOK)
	var userProjects map[string]dto.GroupProjects
	err = json.Unmarshal(resp.Body.Bytes(), &userProjects)
	require.NoError(t, err)
	require.Contains(t, userProjects, fmt.Sprintf("%d", group.GID))

	// Another user attempts to delete project -> 403 Forbidden
	resp = doRequest(t, "DELETE", urlPath, otherUserToken, nil, http.StatusForbidden)

	// User deletes project
	resp = doRequest(t, "DELETE", urlPath, userToken, nil, http.StatusOK)
	var deleteResp response.MessageResponse
	err = json.Unmarshal(resp.Body.Bytes(), &deleteResp)
	require.NoError(t, err)
	require.Equal(t, "project deleted", deleteResp.Message)
}
