package systemhttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type userResponse struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id"`
	OrgID        string     `json:"org_id"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"display_name"`
	Email        string     `json:"email"`
	Status       string     `json:"status"`
	IsSuperAdmin bool       `json:"is_super_admin"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type roleResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

type permissionResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Module      string `json:"module"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type orgResponse struct {
	ID        string        `json:"id"`
	TenantID  string        `json:"tenant_id"`
	ParentID  string        `json:"parent_id"`
	Type      string        `json:"type"`
	Code      string        `json:"code"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	Status    string        `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Children  []orgResponse `json:"children,omitempty"`
}

type auditLogResponse struct {
	ID           string          `json:"id"`
	TenantID     string          `json:"tenant_id"`
	OrgID        string          `json:"org_id"`
	UserID       string          `json:"user_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	RequestID    string          `json:"request_id"`
	IP           string          `json:"ip"`
	UserAgent    string          `json:"user_agent"`
	BeforeData   json.RawMessage `json:"before_data"`
	AfterData    json.RawMessage `json:"after_data"`
	CreatedAt    time.Time       `json:"created_at"`
}

func toUserResponse(user *domain.User) userResponse {
	return userResponse{
		ID:           user.ID,
		TenantID:     user.TenantID,
		OrgID:        user.OrgID,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		Status:       user.Status,
		IsSuperAdmin: user.IsSuperAdmin,
		LastLoginAt:  user.LastLoginAt,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func toRoleResponse(role *domain.Role) roleResponse {
	return roleResponse{
		ID:          role.ID,
		TenantID:    role.TenantID,
		Code:        role.Code,
		Name:        role.Name,
		IsSystem:    role.IsSystem,
		Permissions: role.Permissions,
		CreatedAt:   role.CreatedAt,
	}
}

func toPermissionResponse(permission *domain.Permission) permissionResponse {
	return permissionResponse{
		Code:        permission.Code,
		Name:        permission.Name,
		Module:      permission.Module,
		Type:        permission.Type,
		Description: permission.Description,
	}
}

func toOrgResponse(org *domain.Org) orgResponse {
	children := make([]orgResponse, 0, len(org.Children))
	for _, child := range org.Children {
		children = append(children, toOrgResponse(child))
	}
	return orgResponse{
		ID:        org.ID,
		TenantID:  org.TenantID,
		ParentID:  org.ParentID,
		Type:      org.Type,
		Code:      org.Code,
		Name:      org.Name,
		Path:      org.Path,
		Status:    org.Status,
		CreatedAt: org.CreatedAt,
		UpdatedAt: org.UpdatedAt,
		Children:  children,
	}
}

func toAuditLogResponse(log *domain.AuditLog) auditLogResponse {
	return auditLogResponse{
		ID:           log.ID,
		TenantID:     log.TenantID,
		OrgID:        log.OrgID,
		UserID:       log.UserID,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   log.ResourceID,
		RequestID:    log.RequestID,
		IP:           log.IP,
		UserAgent:    log.UserAgent,
		BeforeData:   json.RawMessage(log.BeforeData),
		AfterData:    json.RawMessage(log.AfterData),
		CreatedAt:    log.CreatedAt,
	}
}

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func tenantIDFrom(c *gin.Context) string {
	info, _ := tenant.From(c.Request.Context())
	return info.ID
}

type requestMeta struct {
	RequestID string
	IP        string
	UserAgent string
}

func metaFrom(c *gin.Context) requestMeta {
	return requestMeta{
		RequestID: c.GetString(httpmw.RequestIDKey),
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
}

func handleError(c *gin.Context, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "resource already exists or invalid input")
	case errors.Is(err, domain.ErrInvalidCredentials):
		httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid username or password")
	case errors.Is(err, domain.ErrUserDisabled):
		httpx.Error(c, http.StatusForbidden, httpx.CodeForbidden, "user is disabled")
	case errors.Is(err, domain.ErrForbidden):
		httpx.Error(c, http.StatusForbidden, httpx.CodeForbidden, "forbidden")
	case errors.Is(err, domain.ErrInvalidToken):
		httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token")
	case errors.Is(err, domain.ErrOrgHasChildren):
		httpx.Error(c, http.StatusConflict, "ORG_HAS_CHILDREN", "organization has children")
	case errors.Is(err, domain.ErrSystemRole):
		httpx.Error(c, http.StatusConflict, "SYSTEM_ROLE", "system role cannot be deleted")
	case errors.Is(err, domain.ErrWeakPassword):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "password must be at least 8 characters")
	default:
		log.Error("system request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}
