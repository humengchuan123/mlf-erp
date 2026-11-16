package systemhttp

import (
	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
)

func recordAudit(c *gin.Context, audit *app.AuditService, action, resourceType, resourceID string, before, after any) {
	if audit == nil {
		return
	}
	meta := metaFrom(c)
	_ = audit.Record(c.Request.Context(), app.AuditInput{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestID:    meta.RequestID,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
		Before:       before,
		After:        after,
	})
}
