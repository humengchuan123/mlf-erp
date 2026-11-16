package systemhttp

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type AuditHandler struct {
	service *app.AuditService
	log     *slog.Logger
}

func NewAuditHandler(service *app.AuditService, log *slog.Logger) *AuditHandler {
	return &AuditHandler{service: service, log: log}
}

func (h *AuditHandler) Register(group *gin.RouterGroup) {
	system := group.Group("/system")
	system.GET("/audit-logs", authmw.RequirePermission("system.audit.read"), h.list)
}

func (h *AuditHandler) list(c *gin.Context) {
	page, pageSize := parsePage(c)
	logs, total, err := h.service.List(c.Request.Context(), tenantIDFrom(c), page, pageSize)
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]auditLogResponse, 0, len(logs))
	for _, log := range logs {
		items = append(items, toAuditLogResponse(log))
	}
	httpx.OK(c, pagedResponse[auditLogResponse]{Items: items, Total: total})
}
