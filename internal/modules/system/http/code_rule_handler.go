package systemhttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/system/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type CodeRuleHandler struct {
	service *app.CodeRuleService
	audit   *app.AuditService
	log     *slog.Logger
}

func NewCodeRuleHandler(service *app.CodeRuleService, audit *app.AuditService, log *slog.Logger) *CodeRuleHandler {
	return &CodeRuleHandler{service: service, audit: audit, log: log}
}

func (h *CodeRuleHandler) Register(group *gin.RouterGroup) {
	system := group.Group("/system")
	system.GET("/code-rules", authmw.RequirePermission("system.code_rule.read"), h.list)
	system.POST("/code-rules", authmw.RequirePermission("system.code_rule.write"), h.create)
	system.PUT("/code-rules/:id", authmw.RequirePermission("system.code_rule.write"), h.update)
}

type codeRuleRequest struct {
	Entity         string `json:"entity"`
	Prefix         string `json:"prefix"`
	DatePattern    string `json:"date_pattern"`
	Separator      string `json:"separator"`
	SequenceLength int    `json:"sequence_length"`
	ResetPolicy    string `json:"reset_policy"`
	Status         string `json:"status"`
}

type codeRuleResponse struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Entity         string     `json:"entity"`
	Prefix         string     `json:"prefix"`
	DatePattern    string     `json:"date_pattern"`
	Separator      string     `json:"separator"`
	SequenceLength int        `json:"sequence_length"`
	CurrentValue   int64      `json:"current_value"`
	ResetPolicy    string     `json:"reset_policy"`
	LastResetDate  *time.Time `json:"last_reset_date,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (h *CodeRuleHandler) list(c *gin.Context) {
	rules, err := h.service.List(c.Request.Context(), tenantIDFrom(c))
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	items := make([]codeRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toCodeRuleResponse(rule))
	}
	httpx.OK(c, items)
}

func (h *CodeRuleHandler) create(c *gin.Context) {
	var request codeRuleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	rule, err := h.service.Create(c.Request.Context(), tenantIDFrom(c), toCodeRuleInput(request))
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toCodeRuleResponse(rule)
	recordAudit(c, h.audit, "code_rule.create", "code_rule", rule.ID, nil, response)
	httpx.Created(c, response)
}

func (h *CodeRuleHandler) update(c *gin.Context) {
	var request codeRuleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	rule, err := h.service.Update(c.Request.Context(), tenantIDFrom(c), c.Param("id"), toCodeRuleInput(request))
	if err != nil {
		handleError(c, h.log, err)
		return
	}

	response := toCodeRuleResponse(rule)
	recordAudit(c, h.audit, "code_rule.update", "code_rule", rule.ID, nil, response)
	httpx.OK(c, response)
}

func toCodeRuleInput(request codeRuleRequest) app.CodeRuleInput {
	return app.CodeRuleInput{
		Entity:         request.Entity,
		Prefix:         request.Prefix,
		DatePattern:    request.DatePattern,
		Separator:      request.Separator,
		SequenceLength: request.SequenceLength,
		ResetPolicy:    request.ResetPolicy,
		Status:         request.Status,
	}
}

func toCodeRuleResponse(rule *domain.CodeRule) codeRuleResponse {
	return codeRuleResponse{
		ID:             rule.ID,
		TenantID:       rule.TenantID,
		Entity:         rule.Entity,
		Prefix:         rule.Prefix,
		DatePattern:    rule.DatePattern,
		Separator:      rule.Separator,
		SequenceLength: rule.SequenceLength,
		CurrentValue:   rule.CurrentValue,
		ResetPolicy:    rule.ResetPolicy,
		LastResetDate:  rule.LastResetDate,
		Status:         rule.Status,
		CreatedAt:      rule.CreatedAt,
		UpdatedAt:      rule.UpdatedAt,
	}
}
