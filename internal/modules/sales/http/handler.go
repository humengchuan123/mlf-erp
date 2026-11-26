package saleshttp

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	approvalapp "github.com/mlf-erp/mlf-erp/internal/modules/approval/app"
	approvaldomain "github.com/mlf-erp/mlf-erp/internal/modules/approval/domain"
	gldomain "github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/sales/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/auth"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	orders    *app.OrderService
	shipments *app.ShipmentService
	returns   *app.ReturnService
	approvals *approvalapp.RequestService
	audit     *systemapp.AuditService
	log       *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	orders *app.OrderService,
	shipments *app.ShipmentService,
	returns *app.ReturnService,
	approvals *approvalapp.RequestService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	handler := &Handler{orders: orders, shipments: shipments, returns: returns, approvals: approvals, audit: audit, log: log}
	sales := group.Group("/sales")
	sales.GET("/orders", authmw.RequirePermission("sales.order.read"), handler.listOrders)
	sales.POST("/orders", authmw.RequirePermission("sales.order.write"), handler.createOrder)
	sales.GET("/orders/:id", authmw.RequirePermission("sales.order.read"), handler.getOrder)
	sales.PUT("/orders/:id", authmw.RequirePermission("sales.order.write"), handler.updateOrder)
	sales.POST("/orders/:id/submit", authmw.RequirePermission("sales.order.write"), handler.submitOrder)
	sales.POST("/orders/:id/approve", authmw.RequirePermission("sales.order.approve"), handler.approveOrder)
	sales.POST("/orders/:id/reject", authmw.RequirePermission("sales.order.approve"), handler.rejectOrder)
	sales.POST("/orders/:id/cancel", authmw.RequirePermission("sales.order.write"), handler.cancelOrder)
	sales.POST("/orders/:id/ship", authmw.RequirePermission("sales.order.write"), handler.shipOrder)
	sales.GET("/shipments", authmw.RequirePermission("sales.shipment.read"), handler.listShipments)
	sales.POST("/shipments/:id/returns", authmw.RequirePermission("sales.return.write"), handler.createReturn)
	sales.GET("/returns", authmw.RequirePermission("sales.return.read"), handler.listReturns)
	sales.GET("/returns/:id", authmw.RequirePermission("sales.return.read"), handler.getReturn)
}

type orderLineRequest struct {
	MaterialID string `json:"material_id"`
	Quantity   string `json:"quantity"`
	UnitPrice  string `json:"unit_price"`
	TaxRateID  string `json:"tax_rate_id"`
}

type orderRequest struct {
	CustomerID       string             `json:"customer_id"`
	WarehouseID      string             `json:"warehouse_id"`
	OrderDate        string             `json:"order_date"`
	Currency         string             `json:"currency"`
	ExchangeRate     string             `json:"exchange_rate"`
	PriceIncludesTax *bool              `json:"price_includes_tax"`
	Remark           string             `json:"remark"`
	Lines            []orderLineRequest `json:"lines"`
}

type shipLineRequest struct {
	OrderLineID string `json:"order_line_id"`
	Quantity    string `json:"quantity"`
}

type shipRequest struct {
	ShipmentDate string            `json:"shipment_date"`
	ExchangeRate string            `json:"exchange_rate"`
	Remark       string            `json:"remark"`
	Lines        []shipLineRequest `json:"lines"`
}

type returnLineRequest struct {
	ShipmentLineID string `json:"shipment_line_id"`
	Quantity       string `json:"quantity"`
}

type returnRequest struct {
	ReturnDate string              `json:"return_date"`
	Reason     string              `json:"reason"`
	Remark     string              `json:"remark"`
	Lines      []returnLineRequest `json:"lines"`
}

type orderLineResponse struct {
	ID              string `json:"id"`
	LineNo          int    `json:"line_no"`
	MaterialID      string `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	TaxRateID       string `json:"tax_rate_id"`
	TaxRate         string `json:"tax_rate"`
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	GrossAmount     string `json:"gross_amount"`
	BaseAmount      string `json:"base_amount"`
	BaseTaxAmount   string `json:"base_tax_amount"`
	BaseGrossAmount string `json:"base_gross_amount"`
	ShippedQty      string `json:"shipped_qty"`
	RemainingQty    string `json:"remaining_qty"`
}

type orderResponse struct {
	ID               string              `json:"id"`
	Code             string              `json:"code"`
	CustomerID       string              `json:"customer_id"`
	CustomerName     string              `json:"customer_name"`
	WarehouseID      string              `json:"warehouse_id"`
	WarehouseName    string              `json:"warehouse_name"`
	OrderDate        string              `json:"order_date"`
	Currency         string              `json:"currency"`
	ExchangeRate     string              `json:"exchange_rate"`
	Status           string              `json:"status"`
	PriceIncludesTax bool                `json:"price_includes_tax"`
	NetAmount        string              `json:"net_amount"`
	TaxAmount        string              `json:"tax_amount"`
	TotalAmount      string              `json:"total_amount"`
	BaseNetAmount    string              `json:"base_net_amount"`
	BaseTaxAmount    string              `json:"base_tax_amount"`
	BaseTotalAmount  string              `json:"base_total_amount"`
	Remark           string              `json:"remark"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	Lines            []orderLineResponse `json:"lines"`
}

type shipmentLineResponse struct {
	ID              string `json:"id"`
	OrderLineID     string `json:"order_line_id"`
	MaterialID      string `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	TaxRateID       string `json:"tax_rate_id"`
	TaxRate         string `json:"tax_rate"`
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	GrossAmount     string `json:"gross_amount"`
	BaseAmount      string `json:"base_amount"`
	BaseTaxAmount   string `json:"base_tax_amount"`
	BaseGrossAmount string `json:"base_gross_amount"`
	UnitCost        string `json:"unit_cost"`
	CostAmount      string `json:"cost_amount"`
	ReturnedQty     string `json:"returned_qty"`
	ReturnableQty   string `json:"returnable_qty"`
}

type shipmentResponse struct {
	ID               string                 `json:"id"`
	Code             string                 `json:"code"`
	OrderID          string                 `json:"order_id"`
	OrderCode        string                 `json:"order_code"`
	CustomerID       string                 `json:"customer_id"`
	CustomerName     string                 `json:"customer_name"`
	WarehouseID      string                 `json:"warehouse_id"`
	WarehouseName    string                 `json:"warehouse_name"`
	ShipmentDate     string                 `json:"shipment_date"`
	Status           string                 `json:"status"`
	PriceIncludesTax bool                   `json:"price_includes_tax"`
	ExchangeRate     string                 `json:"exchange_rate"`
	NetAmount        string                 `json:"net_amount"`
	TaxAmount        string                 `json:"tax_amount"`
	BaseNetAmount    string                 `json:"base_net_amount"`
	BaseTaxAmount    string                 `json:"base_tax_amount"`
	BaseTotalAmount  string                 `json:"base_total_amount"`
	Remark           string                 `json:"remark"`
	InvoiceID        string                 `json:"invoice_id"`
	InvoiceCode      string                 `json:"invoice_code"`
	InvoiceDueDate   string                 `json:"invoice_due_date"`
	VoucherCode      string                 `json:"voucher_code"`
	TotalAmount      string                 `json:"total_amount"`
	TotalCost        string                 `json:"total_cost"`
	CreatedAt        time.Time              `json:"created_at"`
	Lines            []shipmentLineResponse `json:"lines"`
}

type salesReturnLineResponse struct {
	ID              string `json:"id"`
	LineNo          int    `json:"line_no"`
	ShipmentLineID  string `json:"shipment_line_id"`
	OrderLineID     string `json:"order_line_id"`
	MaterialID      string `json:"material_id"`
	MaterialName    string `json:"material_name"`
	Quantity        string `json:"quantity"`
	UnitPrice       string `json:"unit_price"`
	TaxRateID       string `json:"tax_rate_id"`
	TaxRate         string `json:"tax_rate"`
	Amount          string `json:"amount"`
	TaxAmount       string `json:"tax_amount"`
	GrossAmount     string `json:"gross_amount"`
	BaseAmount      string `json:"base_amount"`
	BaseTaxAmount   string `json:"base_tax_amount"`
	BaseGrossAmount string `json:"base_gross_amount"`
	UnitCost        string `json:"unit_cost"`
	CostAmount      string `json:"cost_amount"`
}

type salesReturnResponse struct {
	ID                  string                    `json:"id"`
	Code                string                    `json:"code"`
	ShipmentID          string                    `json:"shipment_id"`
	ShipmentCode        string                    `json:"shipment_code"`
	OrderID             string                    `json:"order_id"`
	OrderCode           string                    `json:"order_code"`
	CustomerID          string                    `json:"customer_id"`
	CustomerName        string                    `json:"customer_name"`
	WarehouseID         string                    `json:"warehouse_id"`
	WarehouseName       string                    `json:"warehouse_name"`
	ReturnDate          string                    `json:"return_date"`
	Status              string                    `json:"status"`
	Currency            string                    `json:"currency"`
	ExchangeRate        string                    `json:"exchange_rate"`
	PriceIncludesTax    bool                      `json:"price_includes_tax"`
	NetAmount           string                    `json:"net_amount"`
	TaxAmount           string                    `json:"tax_amount"`
	TotalAmount         string                    `json:"total_amount"`
	BaseNetAmount       string                    `json:"base_net_amount"`
	BaseTaxAmount       string                    `json:"base_tax_amount"`
	BaseTotalAmount     string                    `json:"base_total_amount"`
	CostAmount          string                    `json:"cost_amount"`
	InvoiceID           string                    `json:"invoice_id"`
	InvoiceCode         string                    `json:"invoice_code"`
	CreditNoteID        string                    `json:"credit_note_id"`
	CreditNoteCode      string                    `json:"credit_note_code"`
	CreditNoteAmount    string                    `json:"credit_note_amount"`
	CreditNoteApplied   string                    `json:"credit_note_applied_amount"`
	CreditNoteRemaining string                    `json:"credit_note_remaining_amount"`
	CreditNoteStatus    string                    `json:"credit_note_status"`
	VoucherCode         string                    `json:"voucher_code"`
	Reason              string                    `json:"reason"`
	Remark              string                    `json:"remark"`
	CreatedAt           time.Time                 `json:"created_at"`
	Lines               []salesReturnLineResponse `json:"lines"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) listOrders(c *gin.Context) {
	page, pageSize := parsePage(c)
	orders, total, err := h.orders.List(c.Request.Context(), c.Query("status"), c.Query("keyword"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]orderResponse, 0, len(orders))
	for _, order := range orders {
		items = append(items, toOrderResponse(order))
	}
	httpx.OK(c, pagedResponse[orderResponse]{Items: items, Total: total})
}

func (h *Handler) getOrder(c *gin.Context) {
	order, err := h.orders.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toOrderResponse(order))
}

func (h *Handler) createOrder(c *gin.Context) {
	input, err := parseOrderInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales order payload")
		return
	}
	order, err := h.orders.Create(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "sales.order.create", "sales_order", order.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateOrder(c *gin.Context) {
	input, err := parseOrderInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales order payload")
		return
	}
	order, err := h.orders.UpdateDraft(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toOrderResponse(order)
	h.recordAudit(c, "sales.order.update", "sales_order", order.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) submitOrder(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if err := h.orders.Submit(ctx, id); err != nil {
		h.handleError(c, err)
		return
	}
	order, err := h.orders.Get(ctx, id)
	if err != nil {
		if reopenErr := h.orders.Reopen(ctx, id); reopenErr != nil {
			h.log.Error("sales order submit compensation failed", "order_id", id, "error", reopenErr)
		}
		h.handleError(c, err)
		return
	}
	actor, _ := auth.ActorFrom(ctx)
	started, err := h.approvals.Start(ctx, approvaldomain.StartInput{
		TenantID:     actor.TenantID,
		OrgID:        order.OrgID,
		DocumentType: approvaldomain.DocumentSalesOrder,
		DocumentID:   order.ID,
		DocumentCode: order.Code,
		RequestedBy:  actor.UserID,
		Amount:       order.TotalAmount,
		BaseAmount:   order.BaseTotalAmount,
		Currency:     order.Currency,
	})
	if err != nil {
		if !errors.Is(err, approvaldomain.ErrPendingExists) {
			if reopenErr := h.orders.Reopen(ctx, id); reopenErr != nil {
				h.log.Error("sales order submit compensation failed", "order_id", id, "error", reopenErr)
			}
		}
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "sales.order.submit", "sales_order", id, nil)
	httpx.OK(c, gin.H{"id": id, "action": "sales.order.submit", "approval_pending": started})
}

func (h *Handler) approveOrder(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	comment, ok := parseApprovalComment(c)
	if !ok {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid approval payload")
		return
	}
	actor, _ := auth.ActorFrom(ctx)
	pending, err := h.approvals.Pending(ctx, approvaldomain.DocumentSalesOrder, id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if pending {
		final, approveErr := h.approvals.Approve(ctx, approvaldomain.DocumentSalesOrder, id, approvaldomain.Actor{
			UserID:       actor.UserID,
			IsSuperAdmin: actor.IsSuperAdmin,
		}, comment)
		if approveErr != nil {
			h.handleError(c, approveErr)
			return
		}
		if !final {
			currentStep := 0
			if pendingRequest, pendingErr := h.approvals.PendingRequest(ctx, approvaldomain.DocumentSalesOrder, id); pendingErr == nil && pendingRequest != nil {
				currentStep = pendingRequest.CurrentStep
			}
			h.recordAudit(c, "sales.order.approve", "sales_order", id, nil)
			httpx.OK(c, gin.H{"id": id, "action": "sales.order.approve", "status": "pending", "current_step": currentStep, "approval_pending": true})
			return
		}
		if err := h.orders.ApproveByApproval(ctx, id); err != nil {
			h.log.Error("approval final transition failed", "order_id", id, "document_type", "sales_order", "error", err)
			h.handleError(c, err)
			return
		}
	} else {
		if err := h.orders.Approve(ctx, id); err != nil {
			h.handleError(c, err)
			return
		}
	}
	h.recordAudit(c, "sales.order.approve", "sales_order", id, nil)
	httpx.OK(c, gin.H{"id": id, "action": "sales.order.approve", "approval_pending": false})
}

func (h *Handler) rejectOrder(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	comment, ok := parseApprovalComment(c)
	if !ok {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid rejection payload")
		return
	}
	if comment == "" {
		httpx.Error(c, http.StatusBadRequest, "REASON_REQUIRED", "rejection reason is required")
		return
	}
	actor, _ := auth.ActorFrom(ctx)
	if err := h.approvals.Reject(ctx, approvaldomain.DocumentSalesOrder, id, approvaldomain.Actor{
		UserID:       actor.UserID,
		IsSuperAdmin: actor.IsSuperAdmin,
	}, comment); err != nil {
		h.handleError(c, err)
		return
	}
	if err := h.orders.ReopenByApproval(ctx, id); err != nil {
		h.log.Error("approval rejection transition failed", "order_id", id, "document_type", "sales_order", "error", err)
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "sales.order.reject", "sales_order", id, nil)
	httpx.OK(c, gin.H{"id": id, "action": "sales.order.reject"})
}

func (h *Handler) cancelOrder(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	actor, _ := auth.ActorFrom(ctx)
	if err := h.approvals.Cancel(ctx, approvaldomain.DocumentSalesOrder, id, actor.UserID, "order cancelled"); err != nil {
		h.handleError(c, err)
		return
	}
	if err := h.orders.Cancel(ctx, id); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "sales.order.cancel", "sales_order", id, nil)
	httpx.OK(c, gin.H{"id": id, "action": "sales.order.cancel"})
}

func (h *Handler) shipOrder(c *gin.Context) {
	var request shipRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid ship payload")
		return
	}
	input, err := parseShipInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid ship payload")
		return
	}
	shipment, err := h.shipments.Ship(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toShipmentResponse(shipment)
	h.recordAudit(c, "sales.shipment.create", "sales_shipment", shipment.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) listShipments(c *gin.Context) {
	page, pageSize := parsePage(c)
	shipments, total, err := h.shipments.List(c.Request.Context(), c.Query("order_id"), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]shipmentResponse, 0, len(shipments))
	for _, shipment := range shipments {
		items = append(items, toShipmentResponse(shipment))
	}
	httpx.OK(c, pagedResponse[shipmentResponse]{Items: items, Total: total})
}

func (h *Handler) createReturn(c *gin.Context) {
	var request returnRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales return payload")
		return
	}
	input, err := parseReturnInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid sales return payload")
		return
	}
	item, err := h.returns.Create(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toReturnResponse(item)
	h.recordAudit(c, "sales.return.create", "sales_return", item.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) listReturns(c *gin.Context) {
	page, pageSize := parsePage(c)
	startDate, err := parseDate(c.Query("start_date"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid start_date")
		return
	}
	endDate, err := parseDate(c.Query("end_date"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid end_date")
		return
	}
	items, total, err := h.returns.List(
		c.Request.Context(),
		c.Query("shipment_id"),
		c.Query("customer_id"),
		c.Query("keyword"),
		startDate,
		endDate,
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]salesReturnResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toReturnResponse(item))
	}
	httpx.OK(c, pagedResponse[salesReturnResponse]{Items: responses, Total: total})
}

func (h *Handler) getReturn(c *gin.Context) {
	item, err := h.returns.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toReturnResponse(item))
}

func parseApprovalComment(c *gin.Context) (string, bool) {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return "", true
	}
	var request struct {
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		if errors.Is(err, io.EOF) {
			return "", true
		}
		return "", false
	}
	return strings.TrimSpace(request.Comment), true
}

func parseOrderInput(c *gin.Context) (app.OrderInput, error) {
	var request orderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.OrderInput{}, err
	}
	orderDate, err := parseDate(request.OrderDate)
	if err != nil {
		return app.OrderInput{}, err
	}
	lines := make([]app.OrderLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.OrderInput{}, quantityErr
		}
		unitPrice, priceErr := decimal.NewFromString(strings.TrimSpace(line.UnitPrice))
		if priceErr != nil {
			return app.OrderInput{}, priceErr
		}
		lines = append(lines, app.OrderLineInput{
			MaterialID: line.MaterialID,
			Quantity:   quantity,
			UnitPrice:  unitPrice,
			TaxRateID:  line.TaxRateID,
		})
	}
	priceIncludesTax := true
	if request.PriceIncludesTax != nil {
		priceIncludesTax = *request.PriceIncludesTax
	}
	exchangeRate, err := parseOptionalDecimal(request.ExchangeRate)
	if err != nil {
		return app.OrderInput{}, err
	}
	return app.OrderInput{
		CustomerID: request.CustomerID, WarehouseID: request.WarehouseID,
		OrderDate: orderDate, Currency: request.Currency,
		ExchangeRate: exchangeRate, PriceIncludesTax: priceIncludesTax,
		Remark: request.Remark, Lines: lines,
	}, nil
}

func parseShipInput(request shipRequest) (app.ShipInput, error) {
	shipmentDate, err := parseDate(request.ShipmentDate)
	if err != nil {
		return app.ShipInput{}, err
	}
	exchangeRate, err := parseOptionalDecimal(request.ExchangeRate)
	if err != nil {
		return app.ShipInput{}, err
	}
	lines := make([]app.ShipLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.ShipInput{}, quantityErr
		}
		lines = append(lines, app.ShipLineInput{OrderLineID: line.OrderLineID, Quantity: quantity})
	}
	return app.ShipInput{ShipmentDate: shipmentDate, ExchangeRate: exchangeRate, Remark: request.Remark, Lines: lines}, nil
}

func parseReturnInput(request returnRequest) (app.ReturnInput, error) {
	returnDate, err := parseDate(request.ReturnDate)
	if err != nil {
		return app.ReturnInput{}, err
	}
	if len(request.Lines) == 0 {
		return app.ReturnInput{}, domain.ErrInvalidInput
	}
	lines := make([]domain.ReturnLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		quantity, quantityErr := decimal.NewFromString(strings.TrimSpace(line.Quantity))
		if quantityErr != nil {
			return app.ReturnInput{}, quantityErr
		}
		if strings.TrimSpace(line.ShipmentLineID) == "" || !quantity.IsPositive() {
			return app.ReturnInput{}, domain.ErrInvalidInput
		}
		lines = append(lines, domain.ReturnLineInput{
			ShipmentLineID: strings.TrimSpace(line.ShipmentLineID),
			Quantity:       quantity,
		})
	}
	return app.ReturnInput{ReturnDate: returnDate, Reason: request.Reason, Remark: request.Remark, Lines: lines}, nil
}

func parseOptionalDecimal(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func parseDate(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", value)
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
	case errors.Is(err, domain.ErrShipmentLineNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "shipment line not found")
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid input")
	case errors.Is(err, domain.ErrOverShipment):
		httpx.Error(c, http.StatusConflict, "OVER_SHIPMENT", "shipped quantity exceeds remaining quantity")
	case errors.Is(err, domain.ErrOverReturn):
		httpx.Error(c, http.StatusConflict, "OVER_RETURN", "return quantity exceeds remaining shipment quantity")
	case errors.Is(err, domain.ErrInsufficientStock):
		httpx.Error(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient inventory")
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "order or shipment status does not allow this operation")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "sales order or shipment is in conflict")
	case errors.Is(err, gldomain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "GL_CONFLICT", "general ledger posting is in conflict")
	case errors.Is(err, gldomain.ErrPeriodClosed):
		httpx.Error(c, http.StatusConflict, "PERIOD_CLOSED", "accounting period is closed")
	case errors.Is(err, gldomain.ErrAccountNotLeaf):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_NOT_LEAF", "account must be a leaf")
	case errors.Is(err, gldomain.ErrAccountDisabled):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_DISABLED", "account is disabled")
	case errors.Is(err, gldomain.ErrUnbalanced):
		httpx.Error(c, http.StatusBadRequest, "UNBALANCED", "automatic voucher is not balanced")
	case errors.Is(err, gldomain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "general ledger posting is not configured")
	case errors.Is(err, fx.ErrRateNotFound):
		httpx.Error(c, http.StatusBadRequest, "EXCHANGE_RATE_NOT_FOUND", "exchange rate not found")
	case errors.Is(err, fx.ErrInvalidRate):
		httpx.Error(c, http.StatusBadRequest, "INVALID_EXCHANGE_RATE", "invalid exchange rate")
	case errors.Is(err, approvaldomain.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "resource not found")
	case errors.Is(err, approvaldomain.ErrInvalidFlow):
		httpx.Error(c, http.StatusBadRequest, "INVALID_APPROVAL_FLOW", "invalid approval flow")
	case errors.Is(err, approvaldomain.ErrFlowInUse):
		httpx.Error(c, http.StatusConflict, "FLOW_IN_USE", "approval flow has approval requests")
	case errors.Is(err, approvaldomain.ErrPendingExists):
		httpx.Error(c, http.StatusConflict, "PENDING_APPROVAL_EXISTS", "pending approval already exists")
	case errors.Is(err, approvaldomain.ErrNoPending):
		httpx.Error(c, http.StatusConflict, "NO_PENDING_APPROVAL", "no pending approval")
	case errors.Is(err, approvaldomain.ErrNotApprover):
		httpx.Error(c, http.StatusForbidden, "NOT_APPROVER", "not the current approver")
	case errors.Is(err, approvaldomain.ErrSelfApproval):
		httpx.Error(c, http.StatusForbidden, "SELF_APPROVAL", "cannot approve own request")
	case errors.Is(err, approvaldomain.ErrAlreadyActed):
		httpx.Error(c, http.StatusConflict, "ALREADY_ACTED", "approval step already acted")
	case errors.Is(err, approvaldomain.ErrNoApprover):
		httpx.Error(c, http.StatusConflict, "APPROVAL_NO_APPROVER", "no eligible approver")
	case errors.Is(err, approvaldomain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "invalid approval status")
	default:
		h.log.Error("sales request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

func toOrderResponse(order *domain.Order) orderResponse {
	lines := make([]orderLineResponse, 0, len(order.Lines))
	for _, line := range order.Lines {
		lines = append(lines, orderLineResponse{
			ID: line.ID, LineNo: line.LineNo, MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			TaxRateID: line.TaxRateID, TaxRate: line.TaxRate.String(),
			Amount: line.Amount.StringFixed(4), TaxAmount: line.TaxAmount.StringFixed(4),
			GrossAmount: line.GrossAmount.StringFixed(4),
			BaseAmount:  line.BaseAmount.StringFixed(4), BaseTaxAmount: line.BaseTaxAmount.StringFixed(4),
			BaseGrossAmount: line.BaseGrossAmount.StringFixed(4), ShippedQty: line.ShippedQty.String(),
			RemainingQty: line.Remaining().String(),
		})
	}
	return orderResponse{
		ID: order.ID, Code: order.Code, CustomerID: order.CustomerID, CustomerName: order.CustomerName,
		WarehouseID: order.WarehouseID, WarehouseName: order.WarehouseName,
		OrderDate: order.OrderDate.Format("2006-01-02"), Currency: order.Currency,
		ExchangeRate: order.ExchangeRate.StringFixed(8),
		Status:       string(order.Status), PriceIncludesTax: order.PriceIncludesTax,
		NetAmount: order.NetAmount.StringFixed(4), TaxAmount: order.TaxAmount.StringFixed(4),
		TotalAmount:   order.TotalAmount.StringFixed(4),
		BaseNetAmount: order.BaseNetAmount.StringFixed(4), BaseTaxAmount: order.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: order.BaseTotalAmount.StringFixed(4),
		Remark:          order.Remark, CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt, Lines: lines,
	}
}

func toShipmentResponse(shipment *domain.Shipment) shipmentResponse {
	lines := make([]shipmentLineResponse, 0, len(shipment.Lines))
	for _, line := range shipment.Lines {
		lines = append(lines, shipmentLineResponse{
			ID:          line.ID,
			OrderLineID: line.OrderLineID, MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			TaxRateID: line.TaxRateID, TaxRate: line.TaxRate.String(),
			Amount: line.Amount.StringFixed(4), TaxAmount: line.TaxAmount.StringFixed(4),
			GrossAmount: line.GrossAmount.StringFixed(4),
			BaseAmount:  line.BaseAmount.StringFixed(4), BaseTaxAmount: line.BaseTaxAmount.StringFixed(4),
			BaseGrossAmount: line.BaseGrossAmount.StringFixed(4), UnitCost: line.UnitCost.String(),
			CostAmount:    line.CostAmount.StringFixed(4),
			ReturnedQty:   line.ReturnedQty.String(),
			ReturnableQty: line.Quantity.Sub(line.ReturnedQty).String(),
		})
	}
	invoiceDueDate := ""
	if !shipment.InvoiceDueDate.IsZero() {
		invoiceDueDate = shipment.InvoiceDueDate.Format("2006-01-02")
	}
	return shipmentResponse{
		ID: shipment.ID, Code: shipment.Code, OrderID: shipment.OrderID, OrderCode: shipment.OrderCode,
		CustomerID: shipment.CustomerID, CustomerName: shipment.CustomerName,
		WarehouseID: shipment.WarehouseID, WarehouseName: shipment.WarehouseName,
		ShipmentDate: shipment.ShipmentDate.Format("2006-01-02"), Status: shipment.Status,
		PriceIncludesTax: shipment.PriceIncludesTax, ExchangeRate: shipment.ExchangeRate.StringFixed(8),
		NetAmount: shipment.NetAmount.StringFixed(4), TaxAmount: shipment.TaxAmount.StringFixed(4),
		BaseNetAmount: shipment.BaseNetAmount.StringFixed(4), BaseTaxAmount: shipment.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: shipment.BaseTotalAmount.StringFixed(4),
		Remark:          shipment.Remark, InvoiceID: shipment.InvoiceID, InvoiceCode: shipment.InvoiceCode,
		InvoiceDueDate: invoiceDueDate, VoucherCode: shipment.VoucherCode,
		TotalAmount: shipment.TotalAmount().StringFixed(4), TotalCost: shipment.TotalCost().StringFixed(4),
		CreatedAt: shipment.CreatedAt, Lines: lines,
	}
}

func toReturnResponse(item *domain.SalesReturn) salesReturnResponse {
	lines := make([]salesReturnLineResponse, 0, len(item.Lines))
	for _, line := range item.Lines {
		lines = append(lines, salesReturnLineResponse{
			ID: line.ID, LineNo: line.LineNo,
			ShipmentLineID: line.ShipmentLineID, OrderLineID: line.OrderLineID,
			MaterialID: line.MaterialID, MaterialName: line.MaterialName,
			Quantity: line.Quantity.String(), UnitPrice: line.UnitPrice.String(),
			TaxRateID: line.TaxRateID, TaxRate: line.TaxRate.String(),
			Amount: line.Amount.StringFixed(4), TaxAmount: line.TaxAmount.StringFixed(4),
			GrossAmount: line.GrossAmount.StringFixed(4),
			BaseAmount:  line.BaseAmount.StringFixed(4), BaseTaxAmount: line.BaseTaxAmount.StringFixed(4),
			BaseGrossAmount: line.BaseGrossAmount.StringFixed(4),
			UnitCost:        line.UnitCost.String(), CostAmount: line.CostAmount.StringFixed(4),
		})
	}
	return salesReturnResponse{
		ID: item.ID, Code: item.Code,
		ShipmentID: item.ShipmentID, ShipmentCode: item.ShipmentCode,
		OrderID: item.OrderID, OrderCode: item.OrderCode,
		CustomerID: item.CustomerID, CustomerName: item.CustomerName,
		WarehouseID: item.WarehouseID, WarehouseName: item.WarehouseName,
		ReturnDate: item.ReturnDate.Format("2006-01-02"), Status: item.Status,
		Currency: item.Currency, ExchangeRate: item.ExchangeRate.StringFixed(8),
		PriceIncludesTax: item.PriceIncludesTax,
		NetAmount:        item.NetAmount.StringFixed(4), TaxAmount: item.TaxAmount.StringFixed(4),
		TotalAmount:     item.TotalAmount.StringFixed(4),
		BaseNetAmount:   item.BaseNetAmount.StringFixed(4),
		BaseTaxAmount:   item.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: item.BaseTotalAmount.StringFixed(4),
		CostAmount:      item.CostAmount.StringFixed(4),
		InvoiceID:       item.InvoiceID, InvoiceCode: item.InvoiceCode,
		CreditNoteID: item.CreditNoteID, CreditNoteCode: item.CreditNoteCode,
		CreditNoteAmount:    item.CreditNoteAmount.StringFixed(4),
		CreditNoteApplied:   item.CreditNoteApplied.StringFixed(4),
		CreditNoteRemaining: item.CreditNoteRemaining.StringFixed(4),
		CreditNoteStatus:    item.CreditNoteStatus,
		VoucherCode:         item.VoucherCode, Reason: item.Reason, Remark: item.Remark,
		CreatedAt: item.CreatedAt, Lines: lines,
	}
}

func (h *Handler) recordAudit(c *gin.Context, action, resourceType, resourceID string, after any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Record(c.Request.Context(), systemapp.AuditInput{
		Action: action, ResourceType: resourceType, ResourceID: resourceID,
		RequestID: c.GetString(httpmw.RequestIDKey), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		After: after,
	})
}
