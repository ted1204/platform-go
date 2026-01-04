package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// HTTPClient represents an HTTP test client
type HTTPClient struct {
	router *gin.Engine
	token  string
}

// NewHTTPClient creates a new HTTP client for testing
func NewHTTPClient(router *gin.Engine, token string) *HTTPClient {
	return &HTTPClient{
		router: router,
		token:  token,
	}
}

// Request represents an HTTP request
type Request struct {
	Method      string
	Path        string
	Body        interface{}
	Headers     map[string]string
	QueryParams map[string]string
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Do performs an HTTP request
func (c *HTTPClient) Do(req Request) (*Response, error) {
	// Prepare request body
	var bodyReader io.Reader
	if req.Body != nil {
		// Check if body is already a bytes.Buffer (for form data)
		if buf, ok := req.Body.(*bytes.Buffer); ok {
			bodyReader = buf
		} else {
			// Marshal as JSON
			bodyBytes, err := json.Marshal(req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %v", err)
			}
			bodyReader = bytes.NewReader(bodyBytes)
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, req.Path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set default headers (only if not already set)
	if req.Headers == nil || req.Headers["Content-Type"] == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Set custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set query parameters
	if len(req.QueryParams) > 0 {
		q := httpReq.URL.Query()
		for key, value := range req.QueryParams {
			q.Add(key, value)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	// Perform request
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, httpReq)

	// Read response body
	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return &Response{
		StatusCode: w.Code,
		Body:       bodyBytes,
		Headers:    w.Header(),
	}, nil
}

// GET performs a GET request
func (c *HTTPClient) GET(path string, queryParams ...map[string]string) (*Response, error) {
	req := Request{
		Method: "GET",
		Path:   path,
	}
	if len(queryParams) > 0 {
		req.QueryParams = queryParams[0]
	}
	return c.Do(req)
}

// POST performs a POST request
func (c *HTTPClient) POST(path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method: "POST",
		Path:   path,
		Body:   body,
	})
}

// PUT performs a PUT request
func (c *HTTPClient) PUT(path string, body interface{}) (*Response, error) {
	return c.Do(Request{
		Method: "PUT",
		Path:   path,
		Body:   body,
	})
}

// DELETE performs a DELETE request
func (c *HTTPClient) DELETE(path string, body ...interface{}) (*Response, error) {
	req := Request{
		Method: "DELETE",
		Path:   path,
	}
	if len(body) > 0 {
		req.Body = body[0]
	}
	return c.Do(req)
}

// POSTForm performs a POST request with form-data
func (c *HTTPClient) POSTForm(path string, formData map[string]string) (*Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range formData {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %v", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	return c.Do(Request{
		Method: "POST",
		Path:   path,
		Headers: map[string]string{
			"Content-Type": writer.FormDataContentType(),
		},
		Body: body,
	})
}

// POSTFormRaw performs a POST request with raw form body
func (c *HTTPClient) POSTFormRaw(path string, formData map[string]string) (*Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range formData {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %v", key, err)
		}
	}

	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", contentType)
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Perform request
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, httpReq)

	// Read response body
	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return &Response{
		StatusCode: w.Code,
		Body:       bodyBytes,
		Headers:    w.Header(),
	}, nil
}

// PUTForm performs a PUT request with form-data
func (c *HTTPClient) PUTForm(path string, formData map[string]string) (*Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range formData {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %v", key, err)
		}
	}

	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("PUT", path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", contentType)
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Perform request
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, httpReq)

	// Read response body
	bodyBytes, err := io.ReadAll(w.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return &Response{
		StatusCode: w.Code,
		Body:       bodyBytes,
		Headers:    w.Header(),
	}, nil
}

// DecodeJSON decodes JSON response body into target
func (r *Response) DecodeJSON(target interface{}) error {
	return json.Unmarshal(r.Body, target)
}

// GetErrorMessage extracts error message from response
func (r *Response) GetErrorMessage() string {
	var errResp map[string]interface{}
	if err := json.Unmarshal(r.Body, &errResp); err != nil {
		return string(r.Body)
	}

	if msg, ok := errResp["error"].(string); ok {
		return msg
	}

	return string(r.Body)
}
