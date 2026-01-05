package utils

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/domain/audit"
	"github.com/linskybing/platform-go/internal/repository"
)

var LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repository.AuditRepo) {
	// Extract data synchronously to avoid race conditions
	userID, _ := GetUserIDFromContext(c)
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")

	// Run DB operation in background
	go func() {
		if err := LogAudit(userID, ip, ua, action, resourceType, resourceID, oldData, newData, msg, repos); err != nil {
			fmt.Printf("[LogAudit] error: %v\n", err)
		}
	}()
}

var LogAudit = func(
	userID uint,
	ip string,
	ua string,
	action string,
	resourceType string,
	resourceID string,
	before any,
	after any,
	description string,
	repos repository.AuditRepo,
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

	auditLog := &audit.AuditLog{
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

	return repos.CreateAuditLog(auditLog)
}
