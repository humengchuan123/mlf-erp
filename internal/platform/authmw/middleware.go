package authmw

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

func Auth(
	tokenService *security.TokenService,
	users domain.UserRepository,
	roles domain.RoleRepository,
	orgs domain.OrgRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "missing bearer token")
			c.Abort()
			return
		}

		rawToken := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := tokenService.ParseAccessToken(rawToken)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		user, err := users.FindByID(c.Request.Context(), claims.TenantID, claims.UserID)
		if err != nil || !user.IsActive() {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "user not found or disabled")
			c.Abort()
			return
		}

		roleCodes, err := roles.RoleCodesByUserID(c.Request.Context(), user.TenantID, user.ID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to load roles")
			c.Abort()
			return
		}
		permissions, err := roles.PermissionCodesByUserID(c.Request.Context(), user.TenantID, user.ID)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to load permissions")
			c.Abort()
			return
		}

		dataScope, err := users.DataScopeByUserID(c.Request.Context(), user.TenantID, user.ID, user.OrgID)
		if err != nil {
			dataScope = "self"
		}
		if user.IsSuperAdmin {
			dataScope = "all"
		}

		var orgPath, companyPath string
		if user.OrgID == "" {
			dataScope = "self"
		} else {
			org, orgErr := orgs.FindByID(c.Request.Context(), user.TenantID, user.OrgID)
			if orgErr != nil {
				dataScope = "self"
			} else {
				orgPath = org.Path
			}

			if dataScope == "company" {
				company, companyErr := orgs.AncestorByType(c.Request.Context(), user.TenantID, user.OrgID, domain.OrgTypeCompany)
				if companyErr != nil {
					dataScope = "self"
				} else {
					companyPath = company.Path
				}
			}
		}

		ctx := tenant.With(c.Request.Context(), tenant.Info{
			ID:    user.TenantID,
			OrgID: user.OrgID,
		})
		ctx = auth.WithActor(ctx, auth.Actor{
			UserID:       user.ID,
			UserName:     user.DisplayName,
			TenantID:     user.TenantID,
			OrgID:        user.OrgID,
			OrgPath:      orgPath,
			CompanyPath:  companyPath,
			DataScope:    dataScope,
			Roles:        roleCodes,
			Permissions:  permissions,
			IsSuperAdmin: user.IsSuperAdmin,
		})
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := auth.ActorFrom(c.Request.Context())
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, httpx.CodeUnauthorized, "authentication required")
			c.Abort()
			return
		}
		if !actor.HasPermission(code) {
			httpx.Error(c, http.StatusForbidden, httpx.CodeForbidden, "permission denied: "+code)
			c.Abort()
			return
		}
		c.Next()
	}
}
