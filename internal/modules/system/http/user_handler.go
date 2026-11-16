package systemhttp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type UserHandler struct {
	service *app.UserService
	audit   *app.AuditService
	log     *slog.Logger
}

func NewUserHandler(service *app.UserService, audit *app.AuditService, log *slog.Logger) *UserHandler {
	return &UserHandler{service: service, audit: audit, log: log}
}

func (h *UserHandler) Register(group *gin.RouterGroup) {
	system := group.Group("/system")
	system.GET("/users", authmw.RequirePermission("system.user.read"), h.list)
	system.POST("/users", authmw.RequirePermission("system.user.write"), h.create)
	system.PUT("/users/:id", authmw.RequirePermission("system.user.write"), h.update)
	system.DELETE("/users/:id", authmw.RequirePermission("system.user.write"), h.disable)
	system.POST("/users/:id/roles", authmw.RequirePermission("system.user.write"), h.setRoles)
	system.GET("/users/:id/roles", authmw.RequirePermission("system.user.read"), h.getRoles)
}

type createUserRequest struct {
	Username     string   `json:"username"`
	DisplayName  string   `json:"display_name"`
	Email        string   `json:"email"`
	Password     string   `json:"password"`
	OrgID        string   `json:"org_id"`
	IsSuperAdmin bool     `json:"is_super_admin"`
	RoleIDs      []string `json:"role_ids"`
}

type updateUserRequest struct {
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	OrgID        string `json:"org_id"`
	Status       string `json:"status"`
	IsSuperAdmin bool   `json:"is_super_admin"`
	Password     string `json:"password"`
}

type setRolesRequest struct {
	RoleIDs []string `json:"role_ids"`
}

func (h *UserHandler) list(c *gin.Context) {
	page, pageSize := parsePage(c)
	users, total, err := h.service.List(c.Request.Context(), tenantIDFrom(c), c.Query("keyword"), page, pageSize)
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]userResponse, 0, len(users))
	for _, user := range users {
		items = append(items, toUserResponse(user))
	}
	httpx.OK(c, pagedResponse[userResponse]{Items: items, Total: total})
}

func (h *UserHandler) create(c *gin.Context) {
	var request createUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	user, err := h.service.Create(c.Request.Context(), app.CreateUserInput{
		TenantID:     tenantIDFrom(c),
		OrgID:        request.OrgID,
		Username:     request.Username,
		DisplayName:  request.DisplayName,
		Email:        request.Email,
		Password:     request.Password,
		IsSuperAdmin: request.IsSuperAdmin,
		RoleIDs:      request.RoleIDs,
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toUserResponse(user)
	recordAudit(c, h.audit, "user.create", "user", user.ID, nil, response)
	httpx.Created(c, response)
}

func (h *UserHandler) update(c *gin.Context) {
	var request updateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	user, err := h.service.Update(c.Request.Context(), tenantIDFrom(c), c.Param("id"), app.UpdateUserInput{
		OrgID:        request.OrgID,
		DisplayName:  request.DisplayName,
		Email:        request.Email,
		Status:       request.Status,
		IsSuperAdmin: request.IsSuperAdmin,
		Password:     request.Password,
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toUserResponse(user)
	recordAudit(c, h.audit, "user.update", "user", user.ID, nil, response)
	httpx.OK(c, response)
}

func (h *UserHandler) disable(c *gin.Context) {
	userID := c.Param("id")
	if err := h.service.Disable(c.Request.Context(), tenantIDFrom(c), userID); err != nil {
		handleError(c, h.log, err)
		return
	}
	recordAudit(c, h.audit, "user.disable", "user", userID, nil, nil)
	httpx.OK(c, gin.H{"id": userID, "status": "disabled"})
}

func (h *UserHandler) setRoles(c *gin.Context) {
	var request setRolesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	userID := c.Param("id")
	if err := h.service.SetRoles(c.Request.Context(), tenantIDFrom(c), userID, request.RoleIDs); err != nil {
		handleError(c, h.log, err)
		return
	}
	recordAudit(c, h.audit, "user.set_roles", "user", userID, nil, request.RoleIDs)
	httpx.OK(c, gin.H{"user_id": userID, "role_ids": request.RoleIDs})
}

func (h *UserHandler) getRoles(c *gin.Context) {
	userID := c.Param("id")
	roleIDs, err := h.service.GetRoleIDs(c.Request.Context(), tenantIDFrom(c), userID)
	if err != nil {
		handleError(c, h.log, err)
		return
	}
	httpx.OK(c, gin.H{"user_id": userID, "role_ids": roleIDs})
}
