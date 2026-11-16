package purchasehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/domain"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	service *app.Service
	log     *slog.Logger
}

func Register(group *gin.RouterGroup, service *app.Service, log *slog.Logger) {
	handler := &Handler{
		service: service,
		log:     log,
	}

	purchase := group.Group("/purchase")
	purchase.GET("/orders", authmw.RequirePermission("purchase.read"), handler.listOrders)
	purchase.POST("/orders", authmw.RequirePermission("purchase.write"), handler.createOrder)
}

type createOrderRequest struct {
	Code       string `json:"code"`
	SupplierID string `json:"supplier_id"`
	Amount     string `json:"amount"`
	Currency   string `json:"currency"`
}

type orderResponse struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	SupplierID string    `json:"supplier_id"`
	Amount     string    `json:"amount"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *Handler) createOrder(c *gin.Context) {
	var request createOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}

	amount, err := decimal.NewFromString(request.Amount)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "amount must be a decimal string")
		return
	}

	order, err := h.service.Create(c.Request.Context(), app.CreateOrderInput{
		Code:       request.Code,
		SupplierID: request.SupplierID,
		Amount:     amount,
		Currency:   request.Currency,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	httpx.Created(c, toResponse(order))
}

func (h *Handler) listOrders(c *gin.Context) {
	orders, err := h.service.List(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		response = append(response, toResponse(order))
	}

	httpx.OK(c, response)
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrTenantRequired),
		errors.Is(err, domain.ErrCodeRequired),
		errors.Is(err, domain.ErrAmountNegative):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
	default:
		h.log.Error("purchase request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

func toResponse(order *domain.Order) orderResponse {
	return orderResponse{
		ID:         order.ID,
		Code:       order.Code,
		SupplierID: order.SupplierID,
		Amount:     order.Amount.StringFixed(4),
		Currency:   order.Currency,
		Status:     string(order.Status),
		CreatedAt:  order.CreatedAt,
	}
}
