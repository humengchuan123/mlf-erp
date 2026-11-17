package glhttp

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/app"
	"github.com/mlf-erp/mlf-erp/internal/modules/gl/domain"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type Handler struct {
	accounts   *app.AccountService
	periods    *app.PeriodService
	vouchers   *app.VoucherService
	reports    *app.ReportService
	dimensions *app.DimensionService
	audit      *systemapp.AuditService
	log        *slog.Logger
}

func Register(
	group *gin.RouterGroup,
	accounts *app.AccountService,
	periods *app.PeriodService,
	vouchers *app.VoucherService,
	reports *app.ReportService,
	dimensions *app.DimensionService,
	audit *systemapp.AuditService,
	log *slog.Logger,
) {
	handler := &Handler{accounts: accounts, periods: periods, vouchers: vouchers, reports: reports, dimensions: dimensions, audit: audit, log: log}
	finance := group.Group("/finance")
	finance.GET("/accounts", authmw.RequirePermission("finance.account.read"), handler.listAccounts)
	finance.POST("/accounts", authmw.RequirePermission("finance.account.write"), handler.createAccount)
	finance.PUT("/accounts/:id", authmw.RequirePermission("finance.account.write"), handler.updateAccount)

	finance.GET("/periods", authmw.RequirePermission("finance.period.read"), handler.listPeriods)
	finance.POST("/periods/ensure", authmw.RequirePermission("finance.period.close"), handler.ensureYear)
	finance.POST("/periods/:id/close", authmw.RequirePermission("finance.period.close"), handler.closePeriod)
	finance.POST("/periods/:id/reopen", authmw.RequirePermission("finance.period.close"), handler.reopenPeriod)
	finance.POST("/years/:year/close", authmw.RequirePermission("finance.period.close"), handler.closeYear)
	finance.GET("/years/:year/close", authmw.RequirePermission("finance.period.read"), handler.yearCloseStatus)

	finance.GET("/vouchers", authmw.RequirePermission("finance.voucher.read"), handler.listVouchers)
	finance.POST("/vouchers", authmw.RequirePermission("finance.voucher.write"), handler.createVoucher)
	finance.GET("/vouchers/:id", authmw.RequirePermission("finance.voucher.read"), handler.getVoucher)
	finance.PUT("/vouchers/:id", authmw.RequirePermission("finance.voucher.write"), handler.updateVoucher)
	finance.POST("/vouchers/:id/post", authmw.RequirePermission("finance.voucher.post"), handler.postVoucher)
	finance.POST("/vouchers/:id/reverse", authmw.RequirePermission("finance.voucher.reverse"), handler.reverseVoucher)

	finance.GET("/dimensions", authmw.RequirePermission("finance.voucher.read"), handler.listDimensions)
	finance.GET("/reports/trial-balance", authmw.RequirePermission("finance.report.read"), handler.trialBalance)
	finance.GET("/reports/general-ledger", authmw.RequirePermission("finance.report.read"), handler.generalLedger)
	finance.GET("/reports/balance-sheet", authmw.RequirePermission("finance.report.read"), handler.balanceSheet)
	finance.GET("/reports/income-statement", authmw.RequirePermission("finance.report.read"), handler.incomeStatement)
	finance.GET("/reports/auxiliary-balance", authmw.RequirePermission("finance.report.read"), handler.auxiliaryBalance)
	finance.GET("/reports/vat-summary", authmw.RequirePermission("finance.report.read"), handler.vatSummary)
}

type accountRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
	ParentID  string `json:"parent_id"`
	Status    string `json:"status"`
}

type accountResponse struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Direction string    `json:"direction"`
	ParentID  string    `json:"parent_id"`
	Level     int       `json:"level"`
	IsLeaf    bool      `json:"is_leaf"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type yearRequest struct {
	Year int `json:"year"`
}

type periodResponse struct {
	ID        string     `json:"id"`
	Year      int        `json:"year"`
	Month     int        `json:"month"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	Status    string     `json:"status"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	ClosedBy  string     `json:"closed_by"`
}

type voucherLineRequest struct {
	AccountID    string `json:"account_id"`
	Summary      string `json:"summary"`
	Debit        string `json:"debit"`
	Credit       string `json:"credit"`
	PartnerID    string `json:"partner_id"`
	DepartmentID string `json:"department_id"`
	ProjectCode  string `json:"project_code"`
}

type voucherRequest struct {
	VoucherDate string               `json:"voucher_date"`
	Memo        string               `json:"memo"`
	Lines       []voucherLineRequest `json:"lines"`
}

type voucherLineResponse struct {
	ID           string `json:"id"`
	LineNo       int    `json:"line_no"`
	AccountID    string `json:"account_id"`
	AccountCode  string `json:"account_code"`
	AccountName  string `json:"account_name"`
	Summary      string `json:"summary"`
	Debit        string `json:"debit"`
	Credit       string `json:"credit"`
	PartnerID    string `json:"partner_id"`
	DepartmentID string `json:"department_id"`
	ProjectCode  string `json:"project_code"`
}

type voucherResponse struct {
	ID                  string                `json:"id"`
	Code                string                `json:"code"`
	VoucherDate         string                `json:"voucher_date"`
	PeriodID            string                `json:"period_id"`
	VoucherType         string                `json:"voucher_type"`
	SourceType          string                `json:"source_type"`
	SourceID            string                `json:"source_id"`
	SourceCode          string                `json:"source_code"`
	Status              string                `json:"status"`
	Memo                string                `json:"memo"`
	CreatedBy           string                `json:"created_by"`
	PostedBy            string                `json:"posted_by"`
	PostedAt            *time.Time            `json:"posted_at,omitempty"`
	ReversalOfVoucherID string                `json:"reversal_of_voucher_id"`
	TotalAmount         string                `json:"total_amount"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	Lines               []voucherLineResponse `json:"lines"`
}

type pagedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type dimensionResponse struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type dimensionListResponse struct {
	Partners    []dimensionResponse `json:"partners"`
	Departments []dimensionResponse `json:"departments"`
}

type trialBalanceRowResponse struct {
	AccountID    string `json:"account_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Direction    string `json:"direction"`
	Opening      string `json:"opening"`
	PeriodDebit  string `json:"period_debit"`
	PeriodCredit string `json:"period_credit"`
	Closing      string `json:"closing"`
}

type trialBalanceResponse struct {
	StartDate   string                    `json:"start_date"`
	EndDate     string                    `json:"end_date"`
	Rows        []trialBalanceRowResponse `json:"rows"`
	TotalDebit  string                    `json:"total_debit"`
	TotalCredit string                    `json:"total_credit"`
}

type generalLedgerEntryResponse struct {
	VoucherID   string `json:"voucher_id"`
	VoucherCode string `json:"voucher_code"`
	VoucherDate string `json:"voucher_date"`
	Summary     string `json:"summary"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
	Balance     string `json:"balance"`
}

type generalLedgerResponse struct {
	Account   accountResponse              `json:"account"`
	StartDate string                       `json:"start_date"`
	EndDate   string                       `json:"end_date"`
	Opening   string                       `json:"opening"`
	Entries   []generalLedgerEntryResponse `json:"entries"`
	Closing   string                       `json:"closing"`
}

type reportLineResponse struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type balanceSheetResponse struct {
	AsOf                      string               `json:"as_of"`
	Assets                    []reportLineResponse `json:"assets"`
	Liabilities               []reportLineResponse `json:"liabilities"`
	Equity                    []reportLineResponse `json:"equity"`
	TotalAssets               string               `json:"total_assets"`
	TotalLiabilities          string               `json:"total_liabilities"`
	TotalEquity               string               `json:"total_equity"`
	CurrentProfit             string               `json:"current_profit"`
	TotalLiabilitiesAndEquity string               `json:"total_liabilities_and_equity"`
}

type auxiliaryBalanceRowResponse struct {
	AccountID     string `json:"account_id"`
	AccountCode   string `json:"account_code"`
	AccountName   string `json:"account_name"`
	DimensionID   string `json:"dimension_id"`
	DimensionCode string `json:"dimension_code"`
	DimensionName string `json:"dimension_name"`
	Opening       string `json:"opening"`
	PeriodDebit   string `json:"period_debit"`
	PeriodCredit  string `json:"period_credit"`
	Closing       string `json:"closing"`
}

type auxiliaryBalanceResponse struct {
	Dimension   string                        `json:"dimension"`
	StartDate   string                        `json:"start_date"`
	EndDate     string                        `json:"end_date"`
	Rows        []auxiliaryBalanceRowResponse `json:"rows"`
	TotalDebit  string                        `json:"total_debit"`
	TotalCredit string                        `json:"total_credit"`
}

type vatSummaryRowResponse struct {
	Month      string `json:"month"`
	OutputTax  string `json:"output_tax"`
	InputTax   string `json:"input_tax"`
	NetPayable string `json:"net_payable"`
}

type vatSummaryResponse struct {
	StartDate       string                  `json:"start_date"`
	EndDate         string                  `json:"end_date"`
	Rows            []vatSummaryRowResponse `json:"rows"`
	TotalOutputTax  string                  `json:"total_output_tax"`
	TotalInputTax   string                  `json:"total_input_tax"`
	TotalNetPayable string                  `json:"total_net_payable"`
}

type incomeStatementResponse struct {
	StartDate     string               `json:"start_date"`
	EndDate       string               `json:"end_date"`
	Revenue       []reportLineResponse `json:"revenue"`
	Expenses      []reportLineResponse `json:"expenses"`
	TotalRevenue  string               `json:"total_revenue"`
	TotalExpenses string               `json:"total_expenses"`
	NetProfit     string               `json:"net_profit"`
}

func (h *Handler) listAccounts(c *gin.Context) {
	items, err := h.accounts.List(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	responses := make([]accountResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toAccountResponse(item))
	}
	httpx.OK(c, responses)
}

func (h *Handler) createAccount(c *gin.Context) {
	var request accountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid account payload")
		return
	}
	account, err := h.accounts.Create(c.Request.Context(), app.AccountInput{
		Code: request.Code, Name: request.Name, Type: request.Type,
		Direction: request.Direction, ParentID: request.ParentID, Status: request.Status,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toAccountResponse(account)
	h.recordAudit(c, "finance.account.create", "gl_account", account.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateAccount(c *gin.Context) {
	var request accountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid account payload")
		return
	}
	account, err := h.accounts.Update(c.Request.Context(), c.Param("id"), app.AccountInput{
		Code: request.Code, Name: request.Name, Type: request.Type,
		Direction: request.Direction, ParentID: request.ParentID, Status: request.Status,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toAccountResponse(account)
	h.recordAudit(c, "finance.account.update", "gl_account", account.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) listPeriods(c *gin.Context) {
	year, _ := strconv.Atoi(c.DefaultQuery("year", "0"))
	items, err := h.periods.List(c.Request.Context(), year)
	if err != nil {
		h.handleError(c, err)
		return
	}
	responses := make([]periodResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toPeriodResponse(item))
	}
	httpx.OK(c, responses)
}

func (h *Handler) ensureYear(c *gin.Context) {
	var request yearRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Year < 2000 || request.Year > 2200 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid year")
		return
	}
	if err := h.periods.EnsureYear(c.Request.Context(), request.Year); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "finance.period.ensure_year", "gl_period", fmt.Sprintf("%d", request.Year), nil)
	httpx.OK(c, gin.H{"year": request.Year, "status": "ensured"})
}

func (h *Handler) closeYear(c *gin.Context) {
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil || year < 2000 || year > 2200 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid year")
		return
	}
	if err := h.periods.CloseYear(c.Request.Context(), year); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "finance.year.close", "gl_year_close", strconv.Itoa(year), nil)
	httpx.OK(c, gin.H{"year": year, "status": "closed"})
}

func (h *Handler) yearCloseStatus(c *gin.Context) {
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil || year < 2000 || year > 2200 {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid year")
		return
	}
	item, err := h.periods.YearCloseStatus(c.Request.Context(), year)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if item == nil {
		httpx.OK(c, gin.H{"year": year, "closed": false})
		return
	}
	httpx.OK(c, gin.H{
		"year":         item.Year,
		"closed":       true,
		"voucher_id":   item.VoucherID,
		"voucher_code": item.VoucherCode,
		"status":       item.Status,
		"closed_at":    item.CreatedAt,
	})
}

func (h *Handler) closePeriod(c *gin.Context) {
	if err := h.periods.Close(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "finance.period.close", "gl_period", c.Param("id"), nil)
	httpx.OK(c, gin.H{"id": c.Param("id"), "status": "closed"})
}

func (h *Handler) reopenPeriod(c *gin.Context) {
	if err := h.periods.Reopen(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "finance.period.reopen", "gl_period", c.Param("id"), nil)
	httpx.OK(c, gin.H{"id": c.Param("id"), "status": "open"})
}

func (h *Handler) listVouchers(c *gin.Context) {
	page, pageSize := parsePage(c)
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	items, total, err := h.vouchers.List(
		c.Request.Context(),
		c.Query("status"),
		c.Query("voucher_type"),
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
	responses := make([]voucherResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toVoucherResponse(item))
	}
	httpx.OK(c, pagedResponse[voucherResponse]{Items: responses, Total: total})
}

func (h *Handler) getVoucher(c *gin.Context) {
	voucher, err := h.vouchers.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, toVoucherResponse(voucher))
}

func (h *Handler) createVoucher(c *gin.Context) {
	input, err := parseVoucherInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid voucher payload")
		return
	}
	voucher, err := h.vouchers.Create(c.Request.Context(), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toVoucherResponse(voucher)
	h.recordAudit(c, "finance.voucher.create", "gl_voucher", voucher.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) updateVoucher(c *gin.Context) {
	input, err := parseVoucherInput(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid voucher payload")
		return
	}
	voucher, err := h.vouchers.UpdateDraft(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toVoucherResponse(voucher)
	h.recordAudit(c, "finance.voucher.update", "gl_voucher", voucher.ID, response)
	httpx.OK(c, response)
}

func (h *Handler) postVoucher(c *gin.Context) {
	if err := h.vouchers.Post(c.Request.Context(), c.Param("id")); err != nil {
		h.handleError(c, err)
		return
	}
	h.recordAudit(c, "finance.voucher.post", "gl_voucher", c.Param("id"), nil)
	httpx.OK(c, gin.H{"id": c.Param("id"), "status": "posted"})
}

func (h *Handler) reverseVoucher(c *gin.Context) {
	voucher, err := h.vouchers.Reverse(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.handleError(c, err)
		return
	}
	response := toVoucherResponse(voucher)
	h.recordAudit(c, "finance.voucher.reverse", "gl_voucher", voucher.ID, response)
	httpx.Created(c, response)
}

func (h *Handler) listDimensions(c *gin.Context) {
	dimensions, err := h.dimensions.List(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	toResponse := func(items []*domain.Dimension) []dimensionResponse {
		result := make([]dimensionResponse, 0, len(items))
		for _, item := range items {
			result = append(result, dimensionResponse{ID: item.ID, Code: item.Code, Name: item.Name})
		}
		return result
	}
	httpx.OK(c, dimensionListResponse{
		Partners:    toResponse(dimensions.Partners),
		Departments: toResponse(dimensions.Departments),
	})
}

func (h *Handler) trialBalance(c *gin.Context) {
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.TrialBalance(c.Request.Context(), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	rows := make([]trialBalanceRowResponse, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, trialBalanceRowResponse{
			AccountID: row.AccountID, Code: row.Code, Name: row.Name, Type: row.Type, Direction: row.Direction,
			Opening: row.Opening.StringFixed(4), PeriodDebit: row.PeriodDebit.StringFixed(4),
			PeriodCredit: row.PeriodCredit.StringFixed(4), Closing: row.Closing.StringFixed(4),
		})
	}
	httpx.OK(c, trialBalanceResponse{
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Rows: rows, TotalDebit: report.TotalDebit.StringFixed(4), TotalCredit: report.TotalCredit.StringFixed(4),
	})
}

func (h *Handler) generalLedger(c *gin.Context) {
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.GeneralLedger(c.Request.Context(), c.Query("account_id"), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	entries := make([]generalLedgerEntryResponse, 0, len(report.Entries))
	for _, entry := range report.Entries {
		entries = append(entries, generalLedgerEntryResponse{
			VoucherID: entry.VoucherID, VoucherCode: entry.VoucherCode,
			VoucherDate: entry.VoucherDate.Format("2006-01-02"), Summary: entry.Summary,
			Debit: entry.Debit.StringFixed(4), Credit: entry.Credit.StringFixed(4),
			Balance: entry.Balance.StringFixed(4),
		})
	}
	httpx.OK(c, generalLedgerResponse{
		Account:   toAccountResponse(report.Account),
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Opening: report.Opening.StringFixed(4), Entries: entries, Closing: report.Closing.StringFixed(4),
	})
}

func (h *Handler) balanceSheet(c *gin.Context) {
	asOf, err := parseOptionalDate(c.Query("as_of"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid as_of")
		return
	}
	report, err := h.reports.BalanceSheet(c.Request.Context(), asOf)
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, balanceSheetResponse{
		AsOf:   report.AsOf.Format("2006-01-02"),
		Assets: toReportLines(report.Assets), Liabilities: toReportLines(report.Liabilities),
		Equity:      toReportLines(report.Equity),
		TotalAssets: report.TotalAssets.StringFixed(4), TotalLiabilities: report.TotalLiabilities.StringFixed(4),
		TotalEquity: report.TotalEquity.StringFixed(4), CurrentProfit: report.CurrentProfit.StringFixed(4),
		TotalLiabilitiesAndEquity: report.TotalLiabilitiesAndEquity.StringFixed(4),
	})
}

func (h *Handler) auxiliaryBalance(c *gin.Context) {
	dimension := c.Query("dimension")
	switch dimension {
	case "partner", "department", "project":
	default:
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid dimension")
		return
	}
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.AuxiliaryBalance(c.Request.Context(), dimension, c.Query("account_id"), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	rows := make([]auxiliaryBalanceRowResponse, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, auxiliaryBalanceRowResponse{
			AccountID: row.AccountID, AccountCode: row.AccountCode, AccountName: row.AccountName,
			DimensionID: row.DimensionID, DimensionCode: row.DimensionCode, DimensionName: row.DimensionName,
			Opening: row.Opening.StringFixed(4), PeriodDebit: row.PeriodDebit.StringFixed(4),
			PeriodCredit: row.PeriodCredit.StringFixed(4), Closing: row.Closing.StringFixed(4),
		})
	}
	httpx.OK(c, auxiliaryBalanceResponse{
		Dimension: report.Dimension,
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Rows: rows, TotalDebit: report.TotalDebit.StringFixed(4), TotalCredit: report.TotalCredit.StringFixed(4),
	})
}

func (h *Handler) vatSummary(c *gin.Context) {
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.VATSummary(c.Request.Context(), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	rows := make([]vatSummaryRowResponse, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, vatSummaryRowResponse{
			Month: row.Month, OutputTax: row.OutputTax.StringFixed(4),
			InputTax: row.InputTax.StringFixed(4), NetPayable: row.NetPayable.StringFixed(4),
		})
	}
	httpx.OK(c, vatSummaryResponse{
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Rows: rows, TotalOutputTax: report.TotalOutputTax.StringFixed(4),
		TotalInputTax: report.TotalInputTax.StringFixed(4), TotalNetPayable: report.TotalNetPayable.StringFixed(4),
	})
}

func (h *Handler) incomeStatement(c *gin.Context) {
	startDate, endDate, err := parseRange(c)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeBadRequest, "invalid date range")
		return
	}
	report, err := h.reports.IncomeStatement(c.Request.Context(), startDate, endDate)
	if err != nil {
		h.handleError(c, err)
		return
	}
	httpx.OK(c, incomeStatementResponse{
		StartDate: report.StartDate.Format("2006-01-02"), EndDate: report.EndDate.Format("2006-01-02"),
		Revenue: toReportLines(report.Revenue), Expenses: toReportLines(report.Expenses),
		TotalRevenue: report.TotalRevenue.StringFixed(4), TotalExpenses: report.TotalExpenses.StringFixed(4),
		NetProfit: report.NetProfit.StringFixed(4),
	})
}

func parseVoucherInput(c *gin.Context) (app.VoucherInput, error) {
	var request voucherRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return app.VoucherInput{}, err
	}
	date, err := parseOptionalDate(request.VoucherDate)
	if err != nil {
		return app.VoucherInput{}, err
	}
	lines := make([]app.VoucherLineInput, 0, len(request.Lines))
	for _, line := range request.Lines {
		debit := decimal.Zero
		if value := strings.TrimSpace(line.Debit); value != "" {
			parsed, debitErr := decimal.NewFromString(value)
			if debitErr != nil {
				return app.VoucherInput{}, debitErr
			}
			debit = parsed
		}
		credit := decimal.Zero
		if value := strings.TrimSpace(line.Credit); value != "" {
			parsed, creditErr := decimal.NewFromString(value)
			if creditErr != nil {
				return app.VoucherInput{}, creditErr
			}
			credit = parsed
		}
		lines = append(lines, app.VoucherLineInput{
			AccountID: line.AccountID, Summary: line.Summary, Debit: debit, Credit: credit,
			PartnerID: line.PartnerID, DepartmentID: line.DepartmentID, ProjectCode: line.ProjectCode,
		})
	}
	return app.VoucherInput{VoucherDate: date, Memo: request.Memo, Lines: lines}, nil
}

func parseOptionalDate(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", value)
}

func parseRange(c *gin.Context) (time.Time, time.Time, error) {
	startDate, err := parseOptionalDate(c.Query("start_date"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDate, err := parseOptionalDate(c.Query("end_date"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return startDate, endDate, nil
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
	case errors.Is(err, domain.ErrUnbalanced):
		httpx.Error(c, http.StatusBadRequest, "UNBALANCED", "voucher debits and credits must be equal")
	case errors.Is(err, domain.ErrAccountNotLeaf):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_NOT_LEAF", "account must be a leaf")
	case errors.Is(err, domain.ErrAccountDisabled):
		httpx.Error(c, http.StatusBadRequest, "ACCOUNT_DISABLED", "account is disabled")
	case errors.Is(err, domain.ErrPeriodClosed):
		httpx.Error(c, http.StatusConflict, "PERIOD_CLOSED", "accounting period is closed")
	case errors.Is(err, domain.ErrYearNotClosed):
		httpx.Error(c, http.StatusConflict, "YEAR_NOT_CLOSED", "all periods in the year must be closed")
	case errors.Is(err, domain.ErrYearClosed):
		httpx.Error(c, http.StatusConflict, "YEAR_CLOSED", "year is already closed")
	case errors.Is(err, domain.ErrYearOutOfOrder):
		httpx.Error(c, http.StatusConflict, "YEAR_OUT_OF_ORDER", "years must be closed in chronological order")
	case errors.Is(err, domain.ErrPeriodCloseMissing):
		httpx.Error(c, http.StatusConflict, "PERIOD_CLOSE_MISSING", "every closed period must have a period close record")
	case errors.Is(err, domain.ErrInvalidStatus):
		httpx.Error(c, http.StatusConflict, "INVALID_STATUS", "voucher status does not allow this operation")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(c, http.StatusConflict, "CONFLICT", "accounting record is in conflict")
	default:
		h.log.Error("general ledger request failed",
			"error", err,
			"request_id", c.GetString(httpmw.RequestIDKey),
			"path", c.Request.URL.Path,
		)
		httpx.Error(c, http.StatusInternalServerError, httpx.CodeInternal, "internal server error")
	}
}

func toAccountResponse(item *domain.Account) accountResponse {
	return accountResponse{
		ID: item.ID, Code: item.Code, Name: item.Name, Type: item.Type, Direction: item.Direction,
		ParentID: item.ParentID, Level: item.Level, IsLeaf: item.IsLeaf, Status: item.Status,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toPeriodResponse(item *domain.Period) periodResponse {
	return periodResponse{
		ID: item.ID, Year: item.Year, Month: item.Month,
		StartDate: item.StartDate.Format("2006-01-02"), EndDate: item.EndDate.Format("2006-01-02"),
		Status: item.Status, ClosedAt: item.ClosedAt, ClosedBy: item.ClosedBy,
	}
}

func toVoucherResponse(item *domain.Voucher) voucherResponse {
	lines := make([]voucherLineResponse, 0, len(item.Lines))
	for _, line := range item.Lines {
		lines = append(lines, voucherLineResponse{
			ID: line.ID, LineNo: line.LineNo, AccountID: line.AccountID,
			AccountCode: line.AccountCode, AccountName: line.AccountName, Summary: line.Summary,
			Debit: line.Debit.StringFixed(4), Credit: line.Credit.StringFixed(4),
			PartnerID: line.PartnerID, DepartmentID: line.DepartmentID, ProjectCode: line.ProjectCode,
		})
	}
	return voucherResponse{
		ID: item.ID, Code: item.Code, VoucherDate: item.VoucherDate.Format("2006-01-02"),
		PeriodID: item.PeriodID, VoucherType: item.VoucherType, SourceType: item.SourceType,
		SourceID: item.SourceID, SourceCode: item.SourceCode, Status: item.Status, Memo: item.Memo,
		CreatedBy: item.CreatedBy, PostedBy: item.PostedBy, PostedAt: item.PostedAt,
		ReversalOfVoucherID: item.ReversalOfVoucherID, TotalAmount: item.TotalAmount.StringFixed(4),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Lines: lines,
	}
}

func toReportLines(items []*domain.ReportLine) []reportLineResponse {
	lines := make([]reportLineResponse, 0, len(items))
	for _, item := range items {
		lines = append(lines, reportLineResponse{
			Code: item.Code, Name: item.Name, Amount: item.Amount.StringFixed(4),
		})
	}
	return lines
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
