package inventoryhttp

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/inventory/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/inventory/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

type Handler struct {
	service *app.QueryService
	log     *slog.Logger
}

func NewHandler(service *app.QueryService, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	inventory := group.Group("/inventory")
	inventory.GET("/balances", authmw.RequirePermission("inventory.balance.read"), h.listBalances)
	inventory.GET("/transactions", authmw.RequirePermission("inventory.transaction.read"), h.listTransactions)
}

type balanceResponse struct {
	TenantID      string    `json:"tenant_id"`
	OrgID         string    `json:"org_id"`
	WarehouseID   string    `json:"warehouse_id"`
	WarehouseName string    `json:"warehouse_name"`
	MaterialID    string    `json:"material_id"`
	MaterialName  string    `json:"material_name"`
	OnHand        string    `json:"on_hand"`
	Reserved      string    `json:"reserved"`
	Available     string    `json:"available"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type transactionResponse struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	OrgID         string    `json:"org_id"`
	WarehouseID   string    `json:"warehouse_id"`
	WarehouseName string    `json:"warehouse_name"`
	MaterialID    string    `json:"material_id"`
	MaterialName  string    `json:"material_name"`
	Direction     string    `json:"direction"`
	BizType       string    `json:"biz_type"`
	BizID         string    `json:"biz_id"`
	BizCode       string    `json:"biz_code"`
	Quantity      string    `json:"quantity"`
	UnitCost      string    `json:"unit_cost"`
	Amount        string    `json:"amount"`
	OccurredAt    time.Time `json:"occurred_at"`
	CreatedBy     string    `json:"created_by"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) listBalances(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.service.ListBalances(
		c.Request.Context(),
		c.Query("warehouse_id"),
		c.Query("material_id"),
		c.Query("keyword"),
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]balanceResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toBalanceResponse(item))
	}
	httpx.OK(c, pagedResponse[balanceResponse]{Items: responses, Total: total})
}

func (h *Handler) listTransactions(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.service.ListTransactions(
		c.Request.Context(),
		c.Query("warehouse_id"),
		c.Query("material_id"),
		c.Query("keyword"),
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]transactionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toTransactionResponse(item))
	}
	httpx.OK(c, pagedResponse[transactionResponse]{Items: responses, Total: total})
}

func (h *Handler) handleError(c *gin.Context, err error) {
	h.log.Error("inventory request failed",
		"error", err,
		"request_id", c.GetString(httpmw.RequestIDKey),
		"path", c.Request.URL.Path,
	)
	httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
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

func toBalanceResponse(item *domain.Balance) balanceResponse {
	return balanceResponse{
		TenantID: item.TenantID, OrgID: item.OrgID,
		WarehouseID: item.WarehouseID, WarehouseName: item.WarehouseName,
		MaterialID: item.MaterialID, MaterialName: item.MaterialName,
		OnHand: item.OnHand.String(), Reserved: item.Reserved.String(),
		Available: item.Available.String(), UpdatedAt: item.UpdatedAt,
	}
}

func toTransactionResponse(item *domain.Transaction) transactionResponse {
	return transactionResponse{
		ID: item.ID, TenantID: item.TenantID, OrgID: item.OrgID,
		WarehouseID: item.WarehouseID, WarehouseName: item.WarehouseName,
		MaterialID: item.MaterialID, MaterialName: item.MaterialName,
		Direction: item.Direction, BizType: item.BizType, BizID: item.BizID, BizCode: item.BizCode,
		Quantity: item.Quantity.String(), UnitCost: item.UnitCost.String(),
		Amount: item.Amount.String(), OccurredAt: item.OccurredAt, CreatedBy: item.CreatedBy,
	}
}
