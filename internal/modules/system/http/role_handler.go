package systemhttp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type RoleHandler struct {
	roles       *app.RoleService
	permissions *app.PermissionService
	audit       *app.AuditService
	log         *slog.Logger
}

func NewRoleHandler(
	roles *app.RoleService,
	permissions *app.PermissionService,
	audit *app.AuditService,
	log *slog.Logger,
) *RoleHandler {
	return &RoleHandler{roles: roles, permissions: permissions, audit: audit, log: log}
}

func (h *RoleHandler) Register(group *gin.RouterGroup) {
	system := group.Group("/system")
	system.GET("/roles", authmw.RequirePermission("system.role.read"), h.list)
	system.POST("/roles", authmw.RequirePermission("system.role.write"), h.create)
	system.PUT("/roles/:id", authmw.RequirePermission("system.role.write"), h.update)
	system.DELETE("/roles/:id", authmw.RequirePermission("system.role.write"), h.delete)
	system.PUT("/roles/:id/permissions", authmw.RequirePermission("system.role.write"), h.setPermissions)
	system.GET("/permissions", authmw.RequirePermission("system.role.read"), h.listPermissions)
}

type createRoleRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type updateRoleRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type setPermissionsRequest struct {
	PermissionCodes []string `json:"permission_codes"`
}

func (h *RoleHandler) list(c *gin.Context) {
	roles, err := h.roles.List(c.Request.Context(), tenantIDFrom(c))
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]roleResponse, 0, len(roles))
	for _, role := range roles {
		items = append(items, toRoleResponse(role))
	}
	httpx.OK(c, items)
}

func (h *RoleHandler) create(c *gin.Context) {
	var request createRoleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	role, err := h.roles.Create(c.Request.Context(), app.CreateRoleInput{
		TenantID: tenantIDFrom(c),
		Code:     request.Code,
		Name:     request.Name,
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toRoleResponse(role)
	recordAudit(c, h.audit, "role.create", "role", role.ID, nil, response)
	httpx.Created(c, response)
}

func (h *RoleHandler) update(c *gin.Context) {
	var request updateRoleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	role, err := h.roles.Update(c.Request.Context(), tenantIDFrom(c), c.Param("id"), request.Code, request.Name)
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toRoleResponse(role)
	recordAudit(c, h.audit, "role.update", "role", role.ID, nil, response)
	httpx.OK(c, response)
}

func (h *RoleHandler) delete(c *gin.Context) {
	roleID := c.Param("id")
	if err := h.roles.Delete(c.Request.Context(), tenantIDFrom(c), roleID); err != nil {
		handleError(c, h.log, err)
		return
	}
	recordAudit(c, h.audit, "role.delete", "role", roleID, nil, nil)
	httpx.OK(c, gin.H{"id": roleID, "deleted": true})
}

func (h *RoleHandler) setPermissions(c *gin.Context) {
	var request setPermissionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	roleID := c.Param("id")
	if err := h.roles.SetPermissions(c.Request.Context(), tenantIDFrom(c), roleID, request.PermissionCodes); err != nil {
		handleError(c, h.log, err)
		return
	}
	recordAudit(c, h.audit, "role.set_permissions", "role", roleID, nil, request.PermissionCodes)
	httpx.OK(c, gin.H{"role_id": roleID, "permission_codes": request.PermissionCodes})
}

func (h *RoleHandler) listPermissions(c *gin.Context) {
	permissions, err := h.permissions.List(c.Request.Context())
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]permissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		items = append(items, toPermissionResponse(permission))
	}
	httpx.OK(c, items)
}
