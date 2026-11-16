package httpmw

import (
	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/tenant"
)

func RequestContext(defaultTenantID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = defaultTenantID
		}

		orgID := c.GetHeader("X-Org-ID")
		userID := c.GetHeader("X-User-ID")
		userName := c.GetHeader("X-User-Name")

		ctx := tenant.With(c.Request.Context(), tenant.Info{
			ID:    tenantID,
			OrgID: orgID,
		})
		ctx = auth.WithActor(ctx, auth.Actor{
			UserID:   userID,
			UserName: userName,
		})

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
