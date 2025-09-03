package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/linskybing/platform-go/models"
	"github.com/stretchr/testify/require"
)

func TestConfigFileFlow_CreateAndGet(t *testing.T) {
	// Login users
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)
	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	// 1. Create a temporary project
	projectForm := url.Values{
		"project_name": {"temp-project-create-get"},
		"description":  {"project for testing"},
		"g_id":         {"1"},
	}
	resp := doRequest(t, "POST", "/projects", adminToken, projectForm, http.StatusCreated)
	var project models.Project
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &project))
	projectID := project.PID

	// 1.1 Add alice to project group
	addUserToGroup(t, adminToken, 1, 2, "manager", http.StatusCreated)

	// 2. Prepare multipart form for ConfigFile
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("filename", "discovery-server")
	_ = writer.WriteField("project_id", fmt.Sprintf("%d", projectID))
	yamlContent := `apiVersion: v1
kind: Pod
metadata:
  name: ros2-discovery-server
spec:
  containers:
  - name: discovery-server
    image: "ubuntu:22.04"`
	part, _ := writer.CreateFormField("raw_yaml")
	_, _ = part.Write([]byte(yamlContent))
	writer.Close()

	req, _ := http.NewRequest("POST", "/config-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken)
	respRec := httptest.NewRecorder()
	router.ServeHTTP(respRec, req)
	require.Equal(t, http.StatusCreated, respRec.Code)

	var configFile models.ConfigFile
	require.NoError(t, json.Unmarshal(respRec.Body.Bytes(), &configFile))
	require.Equal(t, "discovery-server", configFile.Filename)

	// 3. Fetch ConfigFile
	configFileURL := fmt.Sprintf("/config-files/%d", configFile.CFID)
	respRec = doRequest(t, "GET", configFileURL, userToken, nil, http.StatusOK)
	var fetchedConfig models.ConfigFile
	require.NoError(t, json.Unmarshal(respRec.Body.Bytes(), &fetchedConfig))
	require.Equal(t, configFile.CFID, fetchedConfig.CFID)

	// 4. Clean up
	projectURL := fmt.Sprintf("/projects/%d", projectID)
	_ = doRequest(t, "DELETE", projectURL, adminToken, nil, http.StatusOK)
	removeUserFromGroup(t, adminToken, 1, 2, http.StatusNoContent)
}
func TestConfigFileFlow_UpdateAndDelete(t *testing.T) {
	// Login users
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)
	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)
	otherUserToken := loginUser(t, "bob", "123456")
	require.NotEmpty(t, otherUserToken)

	// 1. Create a temporary project
	projectForm := url.Values{
		"project_name": {"temp-project-update-delete"},
		"description":  {"project for testing"},
		"g_id":         {"1"},
	}
	resp := doRequest(t, "POST", "/projects", adminToken, projectForm, http.StatusCreated)
	var project models.Project
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &project))
	projectID := project.PID

	// 1.1 Add alice to project group
	addUserToGroup(t, adminToken, 1, 2, "manager", http.StatusCreated)

	// 2. Create ConfigFile
	createForm := url.Values{
		"filename": {"discovery-server"},
		"raw_yaml": {`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`},
		"project_id": {fmt.Sprintf("%d", projectID)},
	}
	resp = doRequest(t, "POST", "/config-files", adminToken, createForm, http.StatusCreated)
	var configFile models.ConfigFile
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &configFile))
	configFileURL := fmt.Sprintf("/config-files/%d", configFile.CFID)

	// 3. Update ConfigFile (status only)
	updateForm := url.Values{
		"raw_yaml": {`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: updated`},
	}
	_ = doRequest(t, "PUT", configFileURL, userToken, updateForm, http.StatusOK)
	_ = doRequest(t, "PUT", configFileURL, otherUserToken, updateForm, http.StatusForbidden)

	// 4. Create instance (status only)
	instanceURL := fmt.Sprintf("/instance/%d", configFile.CFID)
	_ = doRequest(t, "POST", instanceURL, userToken, nil, http.StatusOK)
	_ = doRequest(t, "DELETE", instanceURL, otherUserToken, nil, http.StatusForbidden)
	_ = doRequest(t, "DELETE", instanceURL, userToken, nil, http.StatusNoContent)

	// 5. Delete ConfigFile (status only)
	_ = doRequest(t, "DELETE", configFileURL, otherUserToken, nil, http.StatusForbidden)
	_ = doRequest(t, "DELETE", configFileURL, userToken, nil, http.StatusNoContent)

	// 6. Clean up project
	projectURL := fmt.Sprintf("/projects/%d", projectID)
	_ = doRequest(t, "DELETE", projectURL, adminToken, nil, http.StatusOK)
	removeUserFromGroup(t, adminToken, 1, 2, http.StatusNoContent)
}
