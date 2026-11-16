package masterdatahttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type CrudService[T any, I any] interface {
	List(ctx context.Context, keyword string, page, pageSize int) ([]T, int, error)
	Get(ctx context.Context, id string) (T, error)
	Create(ctx context.Context, input I) (T, error)
	Update(ctx context.Context, id string, input I) (T, error)
	Disable(ctx context.Context, id string) error
}

type CrudHandler[T any, I any] struct {
	service    CrudService[T, I]
	path       string
	permission string
	entity     string
	decode     func(*gin.Context) (I, error)
	response   func(T) any
	audit      *systemapp.AuditService
	log        *slog.Logger
}

func NewCrudHandler[T any, I any](
	service CrudService[T, I],
	path, permission, entity string,
	decode func(*gin.Context) (I, error),
	response func(T) any,
	audit *systemapp.AuditService,
	log *slog.Logger,
) *CrudHandler[T, I] {
	return &CrudHandler[T, I]{
		service:    service,
		path:       path,
		permission: permission,
		entity:     entity,
		decode:     decode,
		response:   response,
		audit:      audit,
		log:        log,
	}
}

func (h *CrudHandler[T, I]) Register(group *gin.RouterGroup) {
	crud := group.Group(h.path)
	crud.GET("", authmw.RequirePermission(h.permission+".read"), h.list)
	crud.POST("", authmw.RequirePermission(h.permission+".write"), h.create)
	crud.GET("/:id", authmw.RequirePermission(h.permission+".read"), h.get)
	crud.PUT("/:id", authmw.RequirePermission(h.permission+".write"), h.update)
	crud.DELETE("/:id", authmw.RequirePermission(h.permission+".write"), h.disable)
}

func (h *CrudHandler[T, I]) list(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.service.List(c.Request.Context(), c.Query("keyword"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]any, 0, len(items))
	for _, item := range items {
		responses = append(responses, h.response(item))
	}
	httpx.OK(c, pagedResponse{Items: responses, Total: total})
}

func (h *CrudHandler[T, I]) create(c *gin.Context) {
	input, err := h.decode(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := h.response(item)
	recordAudit(c, h.audit, h.entity+".create", h.entity, resourceID(item), nil, response)
	httpx.Created(c, response)
}

func (h *CrudHandler[T, I]) get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, h.response(item))
}

func (h *CrudHandler[T, I]) update(c *gin.Context) {
	input, err := h.decode(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	id := c.Param("id")
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := h.response(item)
	recordAudit(c, h.audit, h.entity+".update", h.entity, id, nil, response)
	httpx.OK(c, response)
}

func (h *CrudHandler[T, I]) disable(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Disable(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, h.audit, h.entity+".disable", h.entity, id, nil, nil)
	httpx.OK(c, gin.H{"id": id, "status": domain.StatusDisabled})
}

func (h *CrudHandler[T, I]) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "resource already exists or is still referenced")
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid input")
	default:
		h.log.Error("masterdata request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

type pagedResponse struct {
	Items []any `json:"items"`
	Total int   `json:"total"`
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

func recordAudit(c *gin.Context, audit *systemapp.AuditService, action, resourceType, resourceID string, before, after any) {
	if audit == nil {
		return
	}
	_ = audit.Record(c.Request.Context(), systemapp.AuditInput{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestID:    c.GetString(httpmw.RequestIDKey),
		IP:           c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Before:       before,
		After:        after,
	})
}

func resourceID[T any](item T) string {
	type identifiable interface{ GetID() string }
	if value, ok := any(item).(identifiable); ok {
		return value.GetID()
	}
	return ""
}
