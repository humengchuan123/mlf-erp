package systemhttp

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type AuthHandler struct {
	service         *app.AuthService
	defaultTenantID string
	log             *slog.Logger
}

func NewAuthHandler(service *app.AuthService, defaultTenantID string, log *slog.Logger) *AuthHandler {
	return &AuthHandler{
		service:         service,
		defaultTenantID: defaultTenantID,
		log:             log,
	}
}

func (h *AuthHandler) RegisterPublic(group *gin.RouterGroup) {
	group.POST("/auth/login", h.login)
	group.POST("/auth/refresh", h.refresh)
}

func (h *AuthHandler) RegisterProtected(group *gin.RouterGroup) {
	group.POST("/auth/logout", h.logout)
	group.GET("/auth/me", h.me)
	group.POST("/auth/change-password", h.changePassword)
}

type loginRequest struct {
	TenantID string `json:"tenant_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type loginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         userResponse `json:"user"`
	Roles        []string     `json:"roles"`
	Permissions  []string     `json:"permissions"`
}

type meResponse struct {
	User        userResponse `json:"user"`
	Roles       []string     `json:"roles"`
	Permissions []string     `json:"permissions"`
}

func (h *AuthHandler) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	tenantID := request.TenantID
	if tenantID == "" {
		tenantID = h.defaultTenantID
	}

	result, err := h.service.Login(c.Request.Context(), app.LoginInput{
		TenantID:  tenantID,
		Username:  request.Username,
		Password:  request.Password,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	httpx.OK(c, toLoginResponse(result))
}

func (h *AuthHandler) refresh(c *gin.Context) {
	var request refreshRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.RefreshToken == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "refresh_token is required")
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), request.RefreshToken, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	httpx.OK(c, toLoginResponse(result))
}

func (h *AuthHandler) logout(c *gin.Context) {
	var request logoutRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.RefreshToken == "" {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "refresh_token is required")
		return
	}

	if err := h.service.Logout(c.Request.Context(), request.RefreshToken); err != nil {
		handleError(c, h.log, err)
		return
	}
	httpx.OK(c, gin.H{"logged_out": true})
}

func (h *AuthHandler) me(c *gin.Context) {
	actor, ok := auth.ActorFrom(c.Request.Context())
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "authentication required")
		return
	}

	result, err := h.service.Me(c.Request.Context(), actor.TenantID, actor.UserID)
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	httpx.OK(c, meResponse{
		User:        toUserResponse(result.User),
		Roles:       result.Roles,
		Permissions: result.Permissions,
	})
}

func (h *AuthHandler) changePassword(c *gin.Context) {
	actor, ok := auth.ActorFrom(c.Request.Context())
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "authentication required")
		return
	}

	var request changePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), actor.TenantID, actor.UserID, request.OldPassword, request.NewPassword); err != nil {
		handleError(c, h.log, err)
		return
	}
	httpx.OK(c, gin.H{"changed": true})
}

func toLoginResponse(result *app.LoginResult) loginResponse {
	return loginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		User:         toUserResponse(result.User),
		Roles:        result.Roles,
		Permissions:  result.Permissions,
	}
}
