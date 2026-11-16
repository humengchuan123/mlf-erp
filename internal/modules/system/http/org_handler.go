package systemhttp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type OrgHandler struct {
	service *app.OrgService
	audit   *app.AuditService
	log     *slog.Logger
}

func NewOrgHandler(service *app.OrgService, audit *app.AuditService, log *slog.Logger) *OrgHandler {
	return &OrgHandler{service: service, audit: audit, log: log}
}

func (h *OrgHandler) Register(group *gin.RouterGroup) {
	system := group.Group("/system")
	system.GET("/orgs", authmw.RequirePermission("system.org.read"), h.tree)
	system.POST("/orgs", authmw.RequirePermission("system.org.write"), h.create)
	system.PUT("/orgs/:id", authmw.RequirePermission("system.org.write"), h.update)
	system.DELETE("/orgs/:id", authmw.RequirePermission("system.org.write"), h.delete)
}

type createOrgRequest struct {
	ParentID string `json:"parent_id"`
	Type     string `json:"type"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}

type updateOrgRequest struct {
	ParentID string `json:"parent_id"`
	Type     string `json:"type"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

func (h *OrgHandler) tree(c *gin.Context) {
	orgs, err := h.service.ListTree(c.Request.Context(), tenantIDFrom(c))
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]orgResponse, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, toOrgResponse(org))
	}
	httpx.OK(c, items)
}

func (h *OrgHandler) create(c *gin.Context) {
	var request createOrgRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	org, err := h.service.Create(c.Request.Context(), app.CreateOrgInput{
		TenantID: tenantIDFrom(c),
		ParentID: request.ParentID,
		Type:     request.Type,
		Code:     request.Code,
		Name:     request.Name,
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toOrgResponse(org)
	recordAudit(c, h.audit, "org.create", "org", org.ID, nil, response)
	httpx.Created(c, response)
}

func (h *OrgHandler) update(c *gin.Context) {
	var request updateOrgRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	org, err := h.service.Update(c.Request.Context(), tenantIDFrom(c), c.Param("id"), app.UpdateOrgInput{
		ParentID: request.ParentID,
		Type:     request.Type,
		Code:     request.Code,
		Name:     request.Name,
		Status:   request.Status,
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toOrgResponse(org)
	recordAudit(c, h.audit, "org.update", "org", org.ID, nil, response)
	httpx.OK(c, response)
}

func (h *OrgHandler) delete(c *gin.Context) {
	orgID := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), tenantIDFrom(c), orgID); err != nil {
		handleError(c, h.log, err)
		return
	}
	recordAudit(c, h.audit, "org.delete", "org", orgID, nil, nil)
	httpx.OK(c, gin.H{"id": orgID, "deleted": true})
}
