package approvalhttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	flows    *app.FlowService
	requests *app.RequestService
	audit    *systemapp.AuditService
	log      *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	flows *app.FlowService,
	requests *app.RequestService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	h := &Handler{flows: flows, requests: requests, audit: audit, log: log}

	system := group.Group("/system")
	system.GET("/approval-flows", authmw.RequirePermission("system.approval.read"), h.listFlows)
	system.POST("/approval-flows", authmw.RequirePermission("system.approval.write"), h.createFlow)
	system.GET("/approval-flows/:id", authmw.RequirePermission("system.approval.read"), h.getFlow)
	system.PUT("/approval-flows/:id", authmw.RequirePermission("system.approval.write"), h.updateFlow)
	system.DELETE("/approval-flows/:id", authmw.RequirePermission("system.approval.write"), h.deleteFlow)

	approvals := group.Group("/approvals")
	approvals.GET("/pending", authmw.RequirePermission("system.approval.read"), h.listPending)
	approvals.GET("/requests", authmw.RequirePermission("system.approval.read"), h.listRequests)
	approvals.GET("/requests/:id", authmw.RequirePermission("system.approval.read"), h.getRequest)
}

type flowStepRequest struct {
	StepNo       int    `json:"step_no"`
	Name         string `json:"name"`
	ApproverType string `json:"approver_type"`
	ApproverID   string `json:"approver_id"`
}

type flowRequest struct {
	Name            string            `json:"name"`
	DocumentType    string            `json:"document_type"`
	OrgID           string            `json:"org_id"`
	ConditionType   string            `json:"condition_type"`
	ThresholdAmount string            `json:"threshold_amount"`
	Priority        int               `json:"priority"`
	Active          *bool             `json:"active"`
	Steps           []flowStepRequest `json:"steps"`
}

type flowStepResponse struct {
	ID           string `json:"id"`
	StepNo       int    `json:"step_no"`
	Name         string `json:"name"`
	ApproverType string `json:"approver_type"`
	ApproverID   string `json:"approver_id"`
}

type flowResponse struct {
	ID              string             `json:"id"`
	TenantID        string             `json:"tenant_id"`
	OrgID           string             `json:"org_id"`
	Name            string             `json:"name"`
	DocumentType    string             `json:"document_type"`
	ConditionType   string             `json:"condition_type"`
	ThresholdAmount string             `json:"threshold_amount"`
	Priority        int                `json:"priority"`
	Active          bool               `json:"active"`
	CreatedBy       string             `json:"created_by"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	Steps           []flowStepResponse `json:"steps"`
}

type flowListResponse struct {
	Items []flowResponse `json:"items"`
}

type requestListResponse struct {
	Items []requestResponse `json:"items"`
}

type requestResponse struct {
	ID           string             `json:"id"`
	TenantID     string             `json:"tenant_id"`
	OrgID        string             `json:"org_id"`
	DocumentType string             `json:"document_type"`
	DocumentID   string             `json:"document_id"`
	DocumentCode string             `json:"document_code"`
	FlowID       string             `json:"flow_id"`
	Status       string             `json:"status"`
	CurrentStep  int                `json:"current_step"`
	RequestedBy  string             `json:"requested_by"`
	RequestedAt  time.Time          `json:"requested_at"`
	CompletedAt  *time.Time         `json:"completed_at"`
	Amount       string             `json:"amount"`
	Currency     string             `json:"currency"`
	BaseAmount   string             `json:"base_amount"`
	Steps        []flowStepResponse `json:"steps"`
	Actions      []actionResponse   `json:"actions"`
}

type actionResponse struct {
	ID        string    `json:"id"`
	StepNo    int       `json:"step_no"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) listFlows(c *gin.Context) {
	var active *bool
	if raw := strings.TrimSpace(c.Query("active")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid active")
			return
		}
		active = &value
	}

	items, err := h.flows.List(c.Request.Context(), domain.DocumentType(c.Query("document_type")), active)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]flowResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toFlowResponse(item))
	}
	httpx.OK(c, flowListResponse{Items: responses})
}

func (h *Handler) createFlow(c *gin.Context) {
	var request flowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	threshold, err := parseThreshold(request.ThresholdAmount)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid threshold_amount")
		return
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}

	flow, err := h.flows.Create(c.Request.Context(), toFlowInput(request, threshold, active))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := toFlowResponse(flow)
	recordAudit(c, h.audit, "system.approval_flow.create", "approval_flow", flow.ID, nil, response)
	httpx.Created(c, response)
}

func (h *Handler) getFlow(c *gin.Context) {
	flow, err := h.flows.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toFlowResponse(flow))
}

func (h *Handler) updateFlow(c *gin.Context) {
	var request flowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	threshold, err := parseThreshold(request.ThresholdAmount)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid threshold_amount")
		return
	}

	id := c.Param("id")
	existing, err := h.flows.Get(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	active := existing.Active
	if request.Active != nil {
		active = *request.Active
	}

	flow, err := h.flows.Update(c.Request.Context(), id, toFlowInput(request, threshold, active))
	if err != nil {
		h.handleError(c, err)
		return
	}

	recordAudit(c, h.audit, "system.approval_flow.update", "approval_flow", id, toFlowResponse(existing), toFlowResponse(flow))
	httpx.OK(c, toFlowResponse(flow))
}

func (h *Handler) deleteFlow(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.flows.Get(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if err := h.flows.Delete(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	recordAudit(c, h.audit, "system.approval_flow.delete", "approval_flow", id, toFlowResponse(existing), nil)
	httpx.OK(c, gin.H{"id": id, "deleted": true})
}

func (h *Handler) listPending(c *gin.Context) {
	actor, _ := auth.ActorFrom(c.Request.Context())
	items, err := h.requests.ListPending(c.Request.Context(), domain.Actor{
		UserID:       actor.UserID,
		IsSuperAdmin: actor.IsSuperAdmin,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]requestResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toRequestResponse(item))
	}
	httpx.OK(c, requestListResponse{Items: responses})
}

func (h *Handler) listRequests(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.requests.List(
		c.Request.Context(),
		domain.DocumentType(c.Query("document_type")),
		c.Query("document_id"),
		c.Query("status"),
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]requestResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toRequestResponse(item))
	}
	httpx.OK(c, pagedResponse[requestResponse]{Items: responses, Total: total})
}

func (h *Handler) getRequest(c *gin.Context) {
	request, err := h.requests.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toRequestResponse(request))
}

func parseThreshold(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func toFlowInput(request flowRequest, threshold decimal.Decimal, active bool) app.FlowInput {
	steps := make([]app.FlowStepInput, 0, len(request.Steps))
	for _, step := range request.Steps {
		steps = append(steps, app.FlowStepInput{
			StepNo:       step.StepNo,
			Name:         step.Name,
			ApproverType: domain.ApproverType(step.ApproverType),
			ApproverID:   step.ApproverID,
		})
	}
	return app.FlowInput{
		Name:            request.Name,
		DocumentType:    domain.DocumentType(request.DocumentType),
		OrgID:           strings.TrimSpace(request.OrgID),
		ConditionType:   domain.ConditionType(request.ConditionType),
		ThresholdAmount: threshold,
		Priority:        request.Priority,
		Active:          active,
		Steps:           steps,
	}
}

func toFlowResponse(flow *domain.Flow) flowResponse {
	steps := make([]flowStepResponse, 0, len(flow.Steps))
	for _, step := range flow.Steps {
		steps = append(steps, toFlowStepResponse(step))
	}
	return flowResponse{
		ID:              flow.ID,
		TenantID:        flow.TenantID,
		OrgID:           flow.OrgID,
		Name:            flow.Name,
		DocumentType:    string(flow.DocumentType),
		ConditionType:   string(flow.ConditionType),
		ThresholdAmount: flow.ThresholdAmount.StringFixed(4),
		Priority:        flow.Priority,
		Active:          flow.Active,
		CreatedBy:       flow.CreatedBy,
		CreatedAt:       flow.CreatedAt,
		UpdatedAt:       flow.UpdatedAt,
		Steps:           steps,
	}
}

func toFlowStepResponse(step domain.FlowStep) flowStepResponse {
	return flowStepResponse{
		ID:           step.ID,
		StepNo:       step.StepNo,
		Name:         step.Name,
		ApproverType: string(step.ApproverType),
		ApproverID:   step.ApproverID,
	}
}

func toRequestResponse(request *domain.Request) requestResponse {
	steps := make([]flowStepResponse, 0, len(request.Steps))
	for _, step := range request.Steps {
		steps = append(steps, toFlowStepResponse(step))
	}
	actions := make([]actionResponse, 0, len(request.Actions))
	for _, action := range request.Actions {
		actions = append(actions, actionResponse{
			ID:        action.ID,
			StepNo:    action.StepNo,
			Action:    string(action.Action),
			ActorID:   action.ActorID,
			Comment:   action.Comment,
			CreatedAt: action.CreatedAt,
		})
	}
	return requestResponse{
		ID:           request.ID,
		TenantID:     request.TenantID,
		OrgID:        request.OrgID,
		DocumentType: string(request.DocumentType),
		DocumentID:   request.DocumentID,
		DocumentCode: request.DocumentCode,
		FlowID:       request.FlowID,
		Status:       string(request.Status),
		CurrentStep:  request.CurrentStep,
		RequestedBy:  request.RequestedBy,
		RequestedAt:  request.RequestedAt,
		CompletedAt:  request.CompletedAt,
		Amount:       request.Amount.StringFixed(4),
		Currency:     request.Currency,
		BaseAmount:   request.BaseAmount.StringFixed(4),
		Steps:        steps,
		Actions:      actions,
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

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found")
	case errors.Is(err, domain.ErrInvalidFlow):
		httpx.Error(c, http.StatusBadRequest, "INVALID_APPROVAL_FLOW", "invalid approval flow")
	case errors.Is(err, domain.ErrFlowInUse):
		httpx.Error(c, http.StatusConflict, "FLOW_IN_USE", "approval flow has approval requests")
	case errors.Is(err, domain.ErrPendingExists):
		httpx.Error(c, http.StatusConflict, "PENDING_APPROVAL_EXISTS", "pending approval already exists")
	case errors.Is(err, domain.ErrNoPending):
		httpx.Error(c, http.StatusConflict, "NO_PENDING_APPROVAL", "no pending approval")
	case errors.Is(err, domain.ErrNotApprover):
		httpx.Error(c, http.StatusForbidden, "NOT_APPROVER", "not the current approver")
	case errors.Is(err, domain.ErrSelfApproval):
		httpx.Error(c, http.StatusForbidden, "SELF_APPROVAL", "cannot approve own request")
	case errors.Is(err, domain.ErrAlreadyActed):
		httpx.Error(c, http.StatusConflict, "ALREADY_ACTED", "approval step already acted")
	case errors.Is(err, domain.ErrNoApprover):
		httpx.Error(c, http.StatusConflict, "APPROVAL_NO_APPROVER", "no eligible approver")
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "invalid approval status")
	default:
		h.log.Error("approval request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
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
