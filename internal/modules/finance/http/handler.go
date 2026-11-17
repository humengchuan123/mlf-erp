package financehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/finance/domain"
	gldomain "github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/fx"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	invoices      *app.InvoiceService
	payments      *app.PaymentService
	exchangeRates *app.ExchangeRateService
	audit         *systemapp.AuditService
	log           *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	invoices *app.InvoiceService,
	payments *app.PaymentService,
	exchangeRates *app.ExchangeRateService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	handler := &Handler{invoices: invoices, payments: payments, exchangeRates: exchangeRates, audit: audit, log: log}
	finance := group.Group("/finance")
	finance.GET("/invoices", authmw.RequirePermission("finance.invoice.read"), handler.listInvoices)
	finance.GET("/invoices/:id", authmw.RequirePermission("finance.invoice.read"), handler.getInvoice)
	finance.POST("/invoices/:id/issue", authmw.RequirePermission("finance.invoice.write"), handler.issueInvoice)
	finance.POST("/invoices/:id/void", authmw.RequirePermission("finance.invoice.write"), handler.voidInvoice)
	finance.GET("/exchange-rates", authmw.RequirePermission("finance.exchange_rate.read"), handler.listExchangeRates)
	finance.GET("/exchange-rates/latest", authmw.RequirePermission("finance.exchange_rate.read"), handler.latestExchangeRate)
	finance.POST("/exchange-rates", authmw.RequirePermission("finance.exchange_rate.write"), handler.upsertExchangeRate)
	finance.PUT("/exchange-rates/:id", authmw.RequirePermission("finance.exchange_rate.write"), handler.updateExchangeRate)
	finance.GET("/payments", authmw.RequirePermission("finance.payment.read"), handler.listPayments)
	finance.GET("/payments/:id", authmw.RequirePermission("finance.payment.read"), handler.getPayment)
	finance.POST("/payments", authmw.RequirePermission("finance.payment.write"), handler.createPayment)
	finance.POST("/payments/:id/allocate", authmw.RequirePermission("finance.payment.write"), handler.allocatePayment)
}

type invoiceResponse struct {
	ID                 string    `json:"id"`
	Code               string    `json:"code"`
	InvoiceType        string    `json:"invoice_type"`
	SourceType         string    `json:"source_type"`
	SourceID           string    `json:"source_id"`
	SourceCode         string    `json:"source_code"`
	PartnerID          string    `json:"partner_id"`
	PartnerName        string    `json:"partner_name"`
	InvoiceDate        string    `json:"invoice_date"`
	DueDate            string    `json:"due_date"`
	Currency           string    `json:"currency"`
	ExchangeRate       string    `json:"exchange_rate"`
	Status             string    `json:"status"`
	NetAmount          string    `json:"net_amount"`
	TaxAmount          string    `json:"tax_amount"`
	TotalAmount        string    `json:"total_amount"`
	BaseNetAmount      string    `json:"base_net_amount"`
	BaseTaxAmount      string    `json:"base_tax_amount"`
	BaseTotalAmount    string    `json:"base_total_amount"`
	BasePaidAmount     string    `json:"base_paid_amount"`
	BaseRevaluedAmount string    `json:"base_revalued_amount"`
	PaidAmount         string    `json:"paid_amount"`
	OutstandingAmount  string    `json:"outstanding_amount"`
	VatInvoiceNo       string    `json:"vat_invoice_no"`
	VatInvoiceType     string    `json:"vat_invoice_type"`
	VatInvoiceDate     string    `json:"vat_invoice_date"`
	VatInvoiceStatus   string    `json:"vat_invoice_status"`
	BuyerTaxNo         string    `json:"buyer_tax_no"`
	SellerTaxNo        string    `json:"seller_tax_no"`
	Remark             string    `json:"remark"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type allocationResponse struct {
	ID          string    `json:"id"`
	PaymentID   string    `json:"payment_id"`
	PaymentCode string    `json:"payment_code"`
	PaymentType string    `json:"payment_type"`
	PaymentDate string    `json:"payment_date"`
	Amount      string    `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
}

type invoiceDetailResponse struct {
	invoiceResponse
	Allocations []allocationResponse `json:"allocations"`
}

type paymentResponse struct {
	ID                  string    `json:"id"`
	Code                string    `json:"code"`
	PaymentType         string    `json:"payment_type"`
	PartnerID           string    `json:"partner_id"`
	PartnerName         string    `json:"partner_name"`
	PaymentDate         string    `json:"payment_date"`
	Currency            string    `json:"currency"`
	ExchangeRate        string    `json:"exchange_rate"`
	Amount              string    `json:"amount"`
	BaseAmount          string    `json:"base_amount"`
	AllocatedAmount     string    `json:"allocated_amount"`
	BaseAllocatedAmount string    `json:"base_allocated_amount"`
	UnallocatedAmount   string    `json:"unallocated_amount"`
	Status              string    `json:"status"`
	Method              string    `json:"method"`
	Reference           string    `json:"reference"`
	Remark              string    `json:"remark"`
	VoucherCode         string    `json:"voucher_code"`
	CreatedAt           time.Time `json:"created_at"`
}

type createPaymentRequest struct {
	PaymentType  string `json:"payment_type"`
	PartnerID    string `json:"partner_id"`
	PaymentDate  string `json:"payment_date"`
	Currency     string `json:"currency"`
	ExchangeRate string `json:"exchange_rate"`
	Amount       string `json:"amount"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Remark       string `json:"remark"`
}

type allocationRequest struct {
	InvoiceID string `json:"invoice_id"`
	Amount    string `json:"amount"`
}

type allocatePaymentRequest struct {
	Allocations []allocationRequest `json:"allocations"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type issueInvoiceRequest struct {
	VatInvoiceNo   string `json:"vat_invoice_no"`
	VatInvoiceType string `json:"vat_invoice_type"`
	VatInvoiceDate string `json:"vat_invoice_date"`
	BuyerTaxNo     string `json:"buyer_tax_no"`
	SellerTaxNo    string `json:"seller_tax_no"`
}

func (h *Handler) issueInvoice(c *gin.Context) {
	var request issueInvoiceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid invoice payload")
		return
	}
	invoiceDate, err := time.Parse("2006-01-02", request.VatInvoiceDate)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid vat invoice date")
		return
	}
	input := domain.IssueInput{
		VatInvoiceNo:   strings.TrimSpace(request.VatInvoiceNo),
		VatInvoiceType: strings.TrimSpace(request.VatInvoiceType),
		VatInvoiceDate: invoiceDate,
		BuyerTaxNo:     strings.TrimSpace(request.BuyerTaxNo),
		SellerTaxNo:    strings.TrimSpace(request.SellerTaxNo),
	}
	if input.VatInvoiceNo == "" ||
		(input.VatInvoiceType != domain.VatInvoiceTypeSpecial && input.VatInvoiceType != domain.VatInvoiceTypeGeneral) {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid vat invoice payload")
		return
	}
	invoice, err := h.invoices.Issue(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toInvoiceResponse(invoice)
	h.recordAudit(c, "finance.invoice.issue", "finance_invoice", invoice.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) voidInvoice(c *gin.Context) {
	invoice, err := h.invoices.Void(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toInvoiceResponse(invoice)
	h.recordAudit(c, "finance.invoice.void", "finance_invoice", invoice.ID, response)
	httpx.OK(c, response)
}

type exchangeRateRequest struct {
	CurrencyCode string `json:"currency_code"`
	RateDate     string `json:"rate_date"`
	Rate         string `json:"rate"`
}

type exchangeRateResponse struct {
	ID           string    `json:"id"`
	CurrencyCode string    `json:"currency_code"`
	RateDate     string    `json:"rate_date"`
	Rate         string    `json:"rate"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (h *Handler) listExchangeRates(c *gin.Context) {
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
	items, total, err := h.exchangeRates.List(c.Request.Context(), c.Query("currency"), startDate, endDate, page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	responses := make([]exchangeRateResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toExchangeRateResponse(item))
	}
	httpx.OK(c, pagedResponse[exchangeRateResponse]{Items: responses, Total: total})
}

func (h *Handler) latestExchangeRate(c *gin.Context) {
	date, err := parseDate(c.Query("date"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date")
		return
	}
	rate, err := h.exchangeRates.Latest(c.Request.Context(), c.Query("currency"), date)
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"currency_code": c.Query("currency"), "date": date.Format("2006-01-02"), "rate": rate.StringFixed(8)})
}

func (h *Handler) upsertExchangeRate(c *gin.Context) {
	var request exchangeRateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid exchange rate payload")
		return
	}
	input, err := parseExchangeRateInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid exchange rate payload")
		return
	}
	item, err := h.exchangeRates.Upsert(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toExchangeRateResponse(item)
	h.recordAudit(c, "finance.exchange_rate.upsert", "finance_exchange_rate", item.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateExchangeRate(c *gin.Context) {
	var request exchangeRateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid exchange rate payload")
		return
	}
	input, err := parseExchangeRateInput(request)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid exchange rate payload")
		return
	}
	item, err := h.exchangeRates.Update(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toExchangeRateResponse(item)
	h.recordAudit(c, "finance.exchange_rate.update", "finance_exchange_rate", item.ID, response)
	httpx.OK(c, response)
}

func parseExchangeRateInput(request exchangeRateRequest) (domain.ExchangeRateInput, error) {
	rateDate, err := parseDate(request.RateDate)
	if err != nil || rateDate.IsZero() {
		return domain.ExchangeRateInput{}, domain.ErrInvalidInput
	}
	rate, err := decimal.NewFromString(strings.TrimSpace(request.Rate))
	if err != nil || !rate.IsPositive() {
		return domain.ExchangeRateInput{}, domain.ErrInvalidInput
	}
	return domain.ExchangeRateInput{
		CurrencyCode: request.CurrencyCode,
		RateDate:     rateDate,
		Rate:         rate,
	}, nil
}

func toExchangeRateResponse(item *domain.ExchangeRate) exchangeRateResponse {
	return exchangeRateResponse{
		ID: item.ID, CurrencyCode: item.CurrencyCode,
		RateDate: item.RateDate.Format("2006-01-02"), Rate: item.Rate.StringFixed(8),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (h *Handler) listInvoices(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.invoices.List(
		c.Request.Context(),
		c.Query("invoice_type"),
		c.Query("status"),
		c.Query("vat_invoice_status"),
		c.Query("partner_id"),
		c.Query("keyword"),
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]invoiceResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toInvoiceResponse(item))
	}
	httpx.OK(c, pagedResponse[invoiceResponse]{Items: responses, Total: total})
}

func (h *Handler) getInvoice(c *gin.Context) {
	detail, err := h.invoices.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}

	allocations := make([]allocationResponse, 0, len(detail.Allocations))
	for _, item := range detail.Allocations {
		allocations = append(allocations, allocationResponse{
			ID: item.ID, PaymentID: item.PaymentID, PaymentCode: item.PaymentCode,
			PaymentType: item.PaymentType, PaymentDate: item.PaymentDate.Format("2006-01-02"),
			Amount: item.Amount.StringFixed(4), CreatedAt: item.CreatedAt,
		})
	}
	httpx.OK(c, invoiceDetailResponse{
		invoiceResponse: toInvoiceResponse(detail.Invoice),
		Allocations:     allocations,
	})
}

func (h *Handler) listPayments(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.payments.List(
		c.Request.Context(),
		c.Query("payment_type"),
		c.Query("status"),
		c.Query("partner_id"),
		c.Query("keyword"),
		page,
		pageSize,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	responses := make([]paymentResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toPaymentResponse(item))
	}
	httpx.OK(c, pagedResponse[paymentResponse]{Items: responses, Total: total})
}

func (h *Handler) getPayment(c *gin.Context) {
	payment, err := h.payments.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toPaymentResponse(payment))
}

func (h *Handler) createPayment(c *gin.Context) {
	var request createPaymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid payment payload")
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(request.Amount))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid payment amount")
		return
	}
	paymentDate, err := parseDate(request.PaymentDate)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid payment date")
		return
	}
	exchangeRate := decimal.Zero
	if strings.TrimSpace(request.ExchangeRate) != "" {
		exchangeRate, err = decimal.NewFromString(strings.TrimSpace(request.ExchangeRate))
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid exchange rate")
			return
		}
	}

	payment, err := h.payments.Create(c.Request.Context(), app.PaymentInput{
		PaymentType:  request.PaymentType,
		PartnerID:    request.PartnerID,
		PaymentDate:  paymentDate,
		Currency:     request.Currency,
		ExchangeRate: exchangeRate,
		Amount:       amount,
		Method:       request.Method,
		Reference:    request.Reference,
		Remark:       request.Remark,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toPaymentResponse(payment)
	h.recordAudit(c, "finance.payment.create", "finance_payment", payment.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) allocatePayment(c *gin.Context) {
	var request allocatePaymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid allocation payload")
		return
	}
	allocations := make([]domain.AllocationInput, 0, len(request.Allocations))
	for _, item := range request.Allocations {
		amount, err := decimal.NewFromString(strings.TrimSpace(item.Amount))
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid allocation amount")
			return
		}
		allocations = append(allocations, domain.AllocationInput{
			InvoiceID: strings.TrimSpace(item.InvoiceID),
			Amount:    amount,
		})
	}

	payment, err := h.payments.Allocate(c.Request.Context(), c.Param("id"), app.AllocateInput{Allocations: allocations})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toPaymentResponse(payment)
	h.recordAudit(c, "finance.payment.allocate", "finance_payment", payment.ID, response)
	httpx.OK(c, response)
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
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid input")
	case errors.Is(err, fx.ErrRateNotFound):
		httpx.Error(c, http.StatusBadRequest, "EXCHANGE_RATE_NOT_FOUND", "exchange rate not found")
	case errors.Is(err, fx.ErrInvalidRate):
		httpx.Error(c, http.StatusBadRequest, "INVALID_EXCHANGE_RATE", "invalid exchange rate")
	case errors.Is(err, domain.ErrTypeMismatch):
		httpx.Error(c, http.StatusBadRequest, "PAYMENT_TYPE_MISMATCH", "invoice and payment type mismatch")
	case errors.Is(err, domain.ErrPartnerMismatch):
		httpx.Error(c, http.StatusBadRequest, "PARTNER_MISMATCH", "invoice and payment partner mismatch")
	case errors.Is(err, domain.ErrCurrencyMismatch):
		httpx.Error(c, http.StatusBadRequest, "CURRENCY_MISMATCH", "invoice and payment currency mismatch")
	case errors.Is(err, domain.ErrOverAllocate):
		httpx.Error(c, http.StatusBadRequest, "OVER_ALLOCATE", "allocation exceeds outstanding amount")
	case errors.Is(err, domain.ErrInvoicePaid):
		httpx.Error(c, http.StatusConflict, "INVOICE_PAID", "invoice is already paid")
	case errors.Is(err, domain.ErrPaymentAllocated):
		httpx.Error(c, http.StatusConflict, "PAYMENT_ALLOCATED", "payment is already fully allocated")
	case errors.Is(err, domain.ErrInvoiceStatus):
		httpx.Error(c, http.StatusConflict, "INVOICE_STATUS", "invoice status does not allow this operation")
	case errors.Is(err, domain.ErrVatInvoiceDuplicate):
		httpx.Error(c, http.StatusConflict, "VAT_INVOICE_NO_DUPLICATE", "VAT invoice number is already used")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "finance record is in conflict")
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
	default:
		h.log.Error("finance request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

func toInvoiceResponse(item *domain.Invoice) invoiceResponse {
	vatInvoiceDate := ""
	if item.VatInvoiceDate != nil {
		vatInvoiceDate = item.VatInvoiceDate.Format("2006-01-02")
	}
	return invoiceResponse{
		ID: item.ID, Code: item.Code, InvoiceType: item.InvoiceType,
		SourceType: item.SourceType, SourceID: item.SourceID, SourceCode: item.SourceCode,
		PartnerID: item.PartnerID, PartnerName: item.PartnerName,
		InvoiceDate: item.InvoiceDate.Format("2006-01-02"), DueDate: item.DueDate.Format("2006-01-02"),
		Currency: item.Currency, ExchangeRate: item.ExchangeRate.StringFixed(8), Status: item.Status,
		NetAmount: item.NetAmount.StringFixed(4), TaxAmount: item.TaxAmount.StringFixed(4),
		TotalAmount:   item.TotalAmount.StringFixed(4),
		BaseNetAmount: item.BaseNetAmount.StringFixed(4), BaseTaxAmount: item.BaseTaxAmount.StringFixed(4),
		BaseTotalAmount: item.BaseTotalAmount.StringFixed(4), BasePaidAmount: item.BasePaidAmount.StringFixed(4),
		BaseRevaluedAmount: item.BaseRevaluedAmount.StringFixed(4),
		PaidAmount:         item.PaidAmount.StringFixed(4),
		OutstandingAmount:  item.Outstanding().StringFixed(4),
		VatInvoiceNo:       item.VatInvoiceNo, VatInvoiceType: item.VatInvoiceType,
		VatInvoiceDate: vatInvoiceDate, VatInvoiceStatus: item.VatInvoiceStatus,
		BuyerTaxNo: item.BuyerTaxNo, SellerTaxNo: item.SellerTaxNo,
		Remark:    item.Remark,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toPaymentResponse(item *domain.Payment) paymentResponse {
	return paymentResponse{
		ID: item.ID, Code: item.Code, PaymentType: item.PaymentType,
		PartnerID: item.PartnerID, PartnerName: item.PartnerName,
		PaymentDate: item.PaymentDate.Format("2006-01-02"), Currency: item.Currency,
		ExchangeRate: item.ExchangeRate.StringFixed(8),
		Amount:       item.Amount.StringFixed(4), BaseAmount: item.BaseAmount.StringFixed(4),
		AllocatedAmount:     item.AllocatedAmount.StringFixed(4),
		BaseAllocatedAmount: item.BaseAllocatedAmount.StringFixed(4),
		UnallocatedAmount:   item.Unallocated().StringFixed(4), Status: item.Status,
		Method: item.Method, Reference: item.Reference, Remark: item.Remark,
		VoucherCode: item.VoucherCode, CreatedAt: item.CreatedAt,
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
