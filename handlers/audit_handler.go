package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
	"github.com/linskybing/platform-go/utils"
)

// GetAuditLogs godoc
// @Summary      Query audit logs
// @Description  Retrieve audit logs filtered by optional parameters like user_id, resource_type, action, time range, with pagination support.
// @Tags         audit
// @Security     BearerAuth
// @Produce      json
// @Param        user_id       query     uint     false  "User ID to filter logs by user" example(123)
// @Param        resource_type query     string   false  "Resource type to filter" example("pod")
// @Param        action        query     string   false  "Action type to filter" example("create")
// @Param        start_time    query     string   false  "Start time in RFC3339 format, e.g. 2023-01-01T00:00:00Z" example("2023-01-01T00:00:00Z")
// @Param        end_time      query     string   false  "End time in RFC3339 format, e.g. 2023-02-01T00:00:00Z" example("2023-02-01T00:00:00Z")
// @Param        limit         query     int      false  "Max number of records to return (default 100, max 1000)" example(100)
// @Param        offset        query     int      false  "Offset for pagination (default 0)" example(0)
// @Success 	 200 {array}   object 					 "List of audit logs"
// @Failure      400 {object}  response.ErrorResponse "Invalid query parameters"
// @Failure      500 {object}  response.ErrorResponse "Internal server error"
// @Router       /audit/logs [get]
func GetAuditLogs(c *gin.Context) {
	var params repositories.AuditQueryParams

	if uid, err := utils.ParseQueryUintParam(c, "user_id"); err != nil {
		if !errors.Is(err, utils.ErrEmptyParameter) {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid user_id"})
			return
		}
	} else {
		params.UserID = &uid
	}

	if rt := c.Query("resource_type"); rt != "" {
		params.ResourceType = &rt
	}
	if act := c.Query("action"); act != "" {
		params.Action = &act
	}

	if start := c.Query("start_time"); start != "" {
		t, err := time.Parse(time.RFC3339, start)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid start_time"})
			return
		}
		params.StartTime = &t
	}

	if end := c.Query("end_time"); end != "" {
		t, err := time.Parse(time.RFC3339, end)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid end_time"})
			return
		}
		params.EndTime = &t
	}

	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit > 1000 {
		limit = 1000
	}

	params.Limit = limit
	params.Offset = offset

	logs, err := services.QueryAuditLogs(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}
