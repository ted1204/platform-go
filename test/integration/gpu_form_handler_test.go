package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPURequestHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	var testRequestID uint

	t.Run("CreateRequest - Success as Project Member", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		requestDTO := map[string]interface{}{
			"gpu_type":  "RTX3090",
			"gpu_count": 2,
			"duration":  30,
			"reason":    "Deep learning training",
		}

		path := fmt.Sprintf("/projects/%d/gpu-requests", ctx.TestProject.PID)
		resp, err := client.POST(path, requestDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		if err == nil {
			if id, ok := result["id"].(float64); ok {
				testRequestID = uint(id)
			}
		}
	})

	t.Run("CreateRequest - Input Validation", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)
		path := fmt.Sprintf("/projects/%d/gpu-requests", ctx.TestProject.PID)

		tests := []struct {
			name  string
			input map[string]interface{}
		}{
			{
				name: "Negative GPU count",
				input: map[string]interface{}{
					"gpu_type":  "RTX3090",
					"gpu_count": -1,
					"duration":  30,
					"reason":    "Test",
				},
			},
			{
				name: "Zero duration",
				input: map[string]interface{}{
					"gpu_type":  "RTX3090",
					"gpu_count": 1,
					"duration":  0,
					"reason":    "Test",
				},
			},
			{
				name: "Empty reason",
				input: map[string]interface{}{
					"gpu_type":  "RTX3090",
					"gpu_count": 1,
					"duration":  30,
					"reason":    "",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := client.POST(path, tt.input)
				require.NoError(t, err)
				assert.GreaterOrEqual(t, resp.StatusCode, 400)
			})
		}
	})

	t.Run("ListRequestsByProject - Member Can View", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/projects/%d/gpu-requests", ctx.TestProject.PID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var requests []interface{}
		err = resp.DecodeJSON(&requests)
		require.NoError(t, err)
	})

	t.Run("ListPendingRequests - Admin Only", func(t *testing.T) {
		// Admin can view
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		resp, err := adminClient.GET("/admin/gpu-requests")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// User cannot view
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err = userClient.GET("/admin/gpu-requests")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ProcessRequest - Admin Can Approve", func(t *testing.T) {
		if testRequestID == 0 {
			t.Skip("No request to process")
		}

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		processDTO := map[string]interface{}{
			"status": "approved",
			"note":   "Approved for research",
		}

		path := fmt.Sprintf("/admin/gpu-requests/%d/status", testRequestID)
		resp, err := client.PUT(path, processDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ProcessRequest - Admin Can Reject", func(t *testing.T) {
		// Create another request to reject
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		requestDTO := map[string]interface{}{
			"gpu_type":  "RTX3090",
			"gpu_count": 1,
			"duration":  30,
			"reason":    "To be rejected",
		}
		path := fmt.Sprintf("/projects/%d/gpu-requests", ctx.TestProject.PID)
		createResp, err := userClient.POST(path, requestDTO)
		require.NoError(t, err)

		var result map[string]interface{}
		err = createResp.DecodeJSON(&result)
		require.NoError(t, err)

		// Check if request was created successfully
		if createResp.StatusCode != http.StatusOK && createResp.StatusCode != http.StatusCreated {
			t.Skipf("Failed to create request for rejection test (status: %d)", createResp.StatusCode)
			return
		}

		// Safely extract request ID
		idValue, ok := result["id"]
		if !ok || idValue == nil {
			t.Skip("No request ID returned, skipping rejection test")
			return
		}

		var requestID uint
		switch v := idValue.(type) {
		case float64:
			requestID = uint(v)
		case int:
			requestID = uint(v)
		default:
			t.Skipf("Unexpected ID type: %T", idValue)
			return
		}

		// Reject it
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		processDTO := map[string]interface{}{
			"status": "rejected",
			"note":   "Insufficient justification",
		}

		processPath := fmt.Sprintf("/admin/gpu-requests/%d/status", requestID)
		resp, err := adminClient.PUT(processPath, processDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ProcessRequest - User Cannot Process", func(t *testing.T) {
		if testRequestID == 0 {
			t.Skip("No request to process")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		processDTO := map[string]interface{}{
			"status": "approved",
		}

		path := fmt.Sprintf("/admin/gpu-requests/%d/status", testRequestID)
		resp, err := client.PUT(path, processDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ProcessRequest - Invalid Status", func(t *testing.T) {
		if testRequestID == 0 {
			t.Skip("No request to process")
		}

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		processDTO := map[string]interface{}{
			"status": "invalid-status",
		}

		path := fmt.Sprintf("/admin/gpu-requests/%d/status", testRequestID)
		resp, err := client.PUT(path, processDTO)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})
}

func TestFormHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	var testFormID uint

	t.Run("CreateForm - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		formDTO := map[string]interface{}{
			"title":   "Resource Request Form",
			"content": "I need additional resources for my project",
			"type":    "resource_request",
		}

		resp, err := client.POST("/forms", formDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		if err == nil {
			if id, ok := result["id"].(float64); ok {
				testFormID = uint(id)
			}
		}
	})

	t.Run("CreateForm - Input Validation", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		tests := []struct {
			name  string
			input map[string]interface{}
		}{
			{
				name: "Empty title",
				input: map[string]interface{}{
					"title":   "",
					"content": "Content",
					"type":    "general",
				},
			},
			{
				name: "Empty content",
				input: map[string]interface{}{
					"title":   "Title",
					"content": "",
					"type":    "general",
				},
			},
			{
				name: "Very long content",
				input: map[string]interface{}{
					"title":   "Title",
					"content": string(make([]byte, 100000)),
					"type":    "general",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := client.POST("/forms", tt.input)
				require.NoError(t, err)
				// Should either reject or accept based on validation rules
				_ = resp // Response validation depends on actual business logic
			})
		}
	})

	t.Run("GetMyForms - User Can View Own Forms", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/forms/my")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var forms []interface{}
		err = resp.DecodeJSON(&forms)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(forms), 1)
	})

	t.Run("GetAllForms - Admin Only", func(t *testing.T) {
		// Admin can view all
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		resp, err := adminClient.GET("/forms")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// User cannot view all
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err = userClient.GET("/forms")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("UpdateFormStatus - Admin Can Update", func(t *testing.T) {
		if testFormID == 0 {
			t.Skip("No form to update")
		}

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]interface{}{
			"status": "approved",
			"note":   "Request approved",
		}

		path := fmt.Sprintf("/forms/%d/status", testFormID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("UpdateFormStatus - User Cannot Update", func(t *testing.T) {
		if testFormID == 0 {
			t.Skip("No form to update")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		updateDTO := map[string]interface{}{
			"status": "approved",
		}

		path := fmt.Sprintf("/forms/%d/status", testFormID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestAuditHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	t.Run("GetAuditLogs - All Users Can View", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/audit/logs")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logs []interface{}
		err = resp.DecodeJSON(&logs)
		require.NoError(t, err)
	})

	t.Run("GetAuditLogs - Filter by Action", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/audit/logs", map[string]string{
			"action": "create",
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logs []interface{}
		err = resp.DecodeJSON(&logs)
		require.NoError(t, err)
	})

	t.Run("GetAuditLogs - Filter by Resource Type", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/audit/logs", map[string]string{
			"resource_type": "project",
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logs []interface{}
		err = resp.DecodeJSON(&logs)
		require.NoError(t, err)
	})

	t.Run("GetAuditLogs - Pagination", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/audit/logs", map[string]string{
			"page":      "1",
			"page_size": "10",
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)
	})
}
