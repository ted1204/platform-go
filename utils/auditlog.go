package utils

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
)

func LogAuditWithConsole(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repositories.AuditRepo) {
	userID, _ := GetUserIDFromContext(c)
	if err := LogAudit(c, userID, action, resourceType, resourceID, oldData, newData, msg, repos); err != nil {
		fmt.Printf("[LogAudit] error: %v\n", err)
	}
}

func LogAudit(
	c *gin.Context,
	userID uint,
	action string,
	resourceType string,
	resourceID string,
	before any,
	after any,
	description string,
	repos repositories.AuditRepo,
) error {
	var oldData, newData []byte
	var err error

	if before != nil {
		oldData, err = json.Marshal(before)
		if err != nil {
			log.Printf("Audit marshal oldData error: %v", err)
		}
	}
	if after != nil {
		newData, err = json.Marshal(after)
		if err != nil {
			log.Printf("Audit marshal newData error: %v", err)
		}
	}

	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	audit := &models.AuditLog{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		OldData:      oldData,
		NewData:      newData,
		IPAddress:    ip,
		UserAgent:    ua,
		Description:  description,
	}

	return repos.CreateAuditLog(audit)
}
