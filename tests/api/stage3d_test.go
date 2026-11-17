//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func findAny(items []any, key, value string) map[string]any {
	for _, item := range items {
		row, ok := item.(map[string]any)
		if ok && text(row, key) == value {
			return row
		}
	}
	return nil
}

func TestStage3D(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, -1)
	startDate := monthStart.Format("2006-01-02")
	endDate := monthEnd.Format("2006-01-02")

	accounts := getArray(admin, "/finance/accounts", http.StatusOK)
	accountByCode := map[string]map[string]any{}
	for _, account := range accounts {
		accountByCode[text(account, "code")] = account
	}
	for _, code := range []string{"1001", "1002", "6001", "6602", "4103", "4104"} {
		if accountByCode[code] == nil {
			t.Fatalf("account %s not found", code)
		}
	}
	cashID := text(accountByCode["1001"], "id")
	revenueID := text(accountByCode["6001"], "id")
	expenseID := text(accountByCode["6602"], "id")
	profitID := text(accountByCode["4103"], "id")

	dimensions := admin.mustGet("/finance/dimensions", http.StatusOK)
	partnerItems, _ := dimensions["partners"].([]any)
	departmentItems, _ := dimensions["departments"].([]any)
	partner := findAny(partnerItems, "code", "CUS-DEMO-001")
	department := findAny(departmentItems, "code", "DEPT-EAST")
	if partner == nil || department == nil {
		t.Fatal("demo partner or department dimension missing")
	}
	if findAny(departmentItems, "code", "HQ") != nil {
		t.Fatal("company organization must not be listed as a department dimension")
	}
	partnerID := text(partner, "id")
	departmentID := text(department, "id")

	periods := getArray(admin, "/finance/periods?year="+strconv.Itoa(now.Year()), http.StatusOK)
	var currentPeriod map[string]any
	for _, period := range periods {
		if month, ok := period["month"].(float64); ok && int(month) == int(now.Month()) {
			currentPeriod = period
			break
		}
	}
	if currentPeriod == nil {
		t.Fatalf("current period month %d not found", now.Month())
	}
	periodID := text(currentPeriod, "id")
	if text(currentPeriod, "status") == "closed" {
		admin.mustPost("/finance/periods/"+periodID+"/reopen", nil, http.StatusOK)
	}

	projectCode := fmt.Sprintf("SD-%d", now.UnixNano())
	t.Run("auxiliary dimensions and balance", func(t *testing.T) {
		voucher := admin.mustPost("/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"memo":         "stage3d auxiliary voucher",
			"lines": []map[string]any{
				{
					"account_id": expenseID, "summary": "aux expense", "debit": "100.00", "credit": "0",
					"partner_id": partnerID, "department_id": departmentID, "project_code": projectCode,
				},
				{"account_id": cashID, "summary": "aux cash", "debit": "0", "credit": "100.00"},
			},
		}, http.StatusCreated)
		admin.mustPost("/finance/vouchers/"+text(voucher, "id")+"/post", nil, http.StatusOK)

		cases := []struct {
			dimension string
			code      string
		}{
			{"partner", "CUS-DEMO-001"},
			{"department", "DEPT-EAST"},
			{"project", projectCode},
		}
		for _, item := range cases {
			report := admin.mustGet(
				"/finance/reports/auxiliary-balance?dimension="+item.dimension+
					"&account_id="+expenseID+"&start_date="+startDate+"&end_date="+endDate,
				http.StatusOK,
			)
			rows, _ := report["rows"].([]any)
			row := findAny(rows, "dimension_code", item.code)
			if row == nil {
				t.Fatalf("auxiliary balance %s row %s not found", item.dimension, item.code)
			}
			if got := number(t, row, "period_debit"); got < 100 {
				t.Fatalf("auxiliary balance %s debit = %v, want >= 100", item.dimension, got)
			}
		}

		nextDay := monthEnd.AddDate(0, 0, 1).Format("2006-01-02")
		openingReport := admin.mustGet(
			"/finance/reports/auxiliary-balance?dimension=project&account_id="+expenseID+
				"&start_date="+nextDay+"&end_date="+nextDay,
			http.StatusOK,
		)
		openingRows, _ := openingReport["rows"].([]any)
		openingRow := findAny(openingRows, "dimension_code", projectCode)
		if openingRow == nil {
			t.Fatal("auxiliary opening balance row not found")
		}
		if got := number(t, openingRow, "opening"); got < 100 {
			t.Fatalf("auxiliary opening balance = %v, want >= 100", got)
		}

		reversalProject := projectCode + "-REV"
		reversalVoucher := admin.mustPost("/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"memo":         "stage3d project reversal voucher",
			"lines": []map[string]any{
				{
					"account_id": expenseID, "summary": "aux reversal expense", "debit": "77.00", "credit": "0",
					"project_code": reversalProject,
				},
				{"account_id": cashID, "summary": "aux reversal cash", "debit": "0", "credit": "77.00"},
			},
		}, http.StatusCreated)
		admin.mustPost("/finance/vouchers/"+text(reversalVoucher, "id")+"/post", nil, http.StatusOK)
		admin.mustPost("/finance/vouchers/"+text(reversalVoucher, "id")+"/reverse", nil, http.StatusCreated)

		reversedReport := admin.mustGet(
			"/finance/reports/auxiliary-balance?dimension=project&account_id="+expenseID+
				"&start_date="+startDate+"&end_date="+endDate,
			http.StatusOK,
		)
		reversedRows, _ := reversedReport["rows"].([]any)
		reversedRow := findAny(reversedRows, "dimension_code", reversalProject)
		if reversedRow != nil {
			if got := number(t, reversedRow, "closing"); !nearlyEqual(got, 0) {
				t.Fatalf("reversed project auxiliary closing = %v, want 0", got)
			}
		}
	})

	t.Run("period close generates profit and loss voucher and reopen reverses it", func(t *testing.T) {
		before := admin.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		beforeExpense := findTrialRow(t, before, "6602")
		if got := number(t, beforeExpense, "closing"); got < 100 {
			t.Fatalf("expense closing before close = %v, want >= 100", got)
		}

		admin.mustPost("/finance/periods/"+periodID+"/close", nil, http.StatusOK)

		after := admin.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		afterExpense := findTrialRow(t, after, "6602")
		if got := number(t, afterExpense, "closing"); !nearlyEqual(got, 0) {
			t.Fatalf("expense closing after close = %v, want 0", got)
		}
		afterProfit := findTrialRow(t, after, "4103")
		if got := number(t, afterProfit, "closing"); got == 0 {
			t.Fatal("profit account closing after close is 0")
		}
		verifyReportScope(t, admin, startDate, endDate)

		income := admin.mustGet("/finance/reports/income-statement?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		if got := number(t, income, "total_expenses"); got < 100 {
			t.Fatalf("income statement expenses after close = %v, want >= 100", got)
		}

		closingVouchers := listItems(admin,
			"/finance/vouchers?voucher_type=period_close&start_date="+startDate+"&end_date="+endDate+"&page_size=200",
			http.StatusOK,
		)
		closing := findItem(closingVouchers, "source_code", fmt.Sprintf("%04d-%02d", now.Year(), now.Month()))
		if closing == nil {
			t.Fatal("period close voucher not found")
		}
		detail := admin.mustGet("/finance/vouchers/"+text(closing, "id"), http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if findAny(lines, "account_id", profitID) == nil {
			t.Fatal("period close voucher missing profit account line")
		}

		admin.mustPost("/finance/periods/"+periodID+"/reopen", nil, http.StatusOK)
		reopened := admin.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		reopenedExpense := findTrialRow(t, reopened, "6602")
		if got := number(t, reopenedExpense, "closing"); got < 100 {
			t.Fatalf("expense closing after reopen = %v, want >= 100", got)
		}
		reversalVouchers := listItems(admin,
			"/finance/vouchers?voucher_type=period_close_reversal&start_date="+startDate+"&end_date="+endDate+"&page_size=200",
			http.StatusOK,
		)
		reversal := findItem(reversalVouchers, "source_code", fmt.Sprintf("%04d-%02d", now.Year(), now.Month()))
		if reversal == nil {
			t.Fatal("period close reversal voucher not found")
		}

		admin.mustPost("/finance/periods/"+periodID+"/close", nil, http.StatusOK)
		reclosed := admin.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
		reclosedExpense := findTrialRow(t, reclosed, "6602")
		if got := number(t, reclosedExpense, "closing"); !nearlyEqual(got, 0) {
			t.Fatalf("expense closing after re-close = %v, want 0", got)
		}
		admin.mustPost("/finance/periods/"+periodID+"/reopen", nil, http.StatusOK)
	})

	t.Run("data scope and permissions", func(t *testing.T) {
		demoPassword := env("TEST_DEMO_PASSWORD", "Demo@123456")
		dept := newClient(t).login("demo_dept", demoPassword)
		deptVouchers := dept.mustGet("/finance/vouchers?page_size=100", http.StatusOK)
		if got, _ := deptVouchers["total"].(float64); got != 0 {
			t.Fatalf("demo_dept voucher total = %v, want 0", got)
		}
		dept.expectError(http.MethodPost, "/finance/vouchers", map[string]any{
			"voucher_date": endDate,
			"lines": []map[string]any{
				{"account_id": cashID, "debit": "1.00", "credit": "0"},
				{"account_id": profitID, "debit": "0", "credit": "1.00"},
			},
		}, http.StatusForbidden, "FORBIDDEN")
		dept.expectError(http.MethodPost, "/finance/periods/ensure", map[string]any{"year": now.Year()}, http.StatusForbidden, "FORBIDDEN")
		admin.expectError(http.MethodGet, "/finance/reports/auxiliary-balance?dimension=partner&account_id=not-a-uuid", nil, http.StatusBadRequest, "BAD_REQUEST")
		unauthorized := newClient(t)
		unauthorized.expectError(http.MethodGet, "/finance/reports/auxiliary-balance?dimension=partner", nil, http.StatusUnauthorized, "UNAUTHORIZED")
	})

	t.Run("year close is chronological and transfers each year once", func(t *testing.T) {
		years := cleanYears(t, admin, 6)
		if len(years) < 6 {
			t.Skip("no six consecutive clean years left; run this test against a freshly migrated database")
		}
		first, second, zeroYear := years[0], years[1], years[2]
		priorOpen, laterOpen, incomplete := years[3], years[4], years[5]

		ensureTestYear(t, admin, first)
		postYearRevenue(t, admin, first, "100.00", cashID, revenueID)
		closeOpenPeriods(t, admin, first)

		status := admin.mustGet(yearClosePath(first), http.StatusOK)
		if closed, _ := status["closed"].(bool); closed {
			t.Fatalf("year %d unexpectedly closed before close", first)
		}
		admin.mustPost(yearClosePath(first), nil, http.StatusOK)
		status = admin.mustGet(yearClosePath(first), http.StatusOK)
		if closed, _ := status["closed"].(bool); !closed {
			t.Fatalf("year %d close status = %v", first, status)
		}
		if text(status, "voucher_code") == "" {
			t.Fatalf("year %d close voucher missing: %v", first, status)
		}
		admin.expectError(http.MethodPost, yearClosePath(first), nil, http.StatusConflict, "YEAR_CLOSED")
		firstPeriods := getArray(admin, periodsPath(first), http.StatusOK)
		admin.expectError(http.MethodPost, "/finance/periods/"+text(firstPeriods[0], "id")+"/reopen", nil, http.StatusConflict, "YEAR_CLOSED")

		ensureTestYear(t, admin, second)
		postYearRevenue(t, admin, second, "50.00", cashID, revenueID)
		closeOpenPeriods(t, admin, second)
		secondBefore := admin.mustGet(
			"/finance/reports/trial-balance?start_date="+fmt.Sprintf("%04d-01-01", second)+"&end_date="+fmt.Sprintf("%04d-12-31", second),
			http.StatusOK,
		)
		retainedBeforeSecond := number(t, findTrialRow(t, secondBefore, "4104"), "closing")
		admin.mustPost(yearClosePath(second), nil, http.StatusOK)
		nextStart := fmt.Sprintf("%04d-01-01", second+1)
		report := admin.mustGet("/finance/reports/trial-balance?start_date="+nextStart+"&end_date="+nextStart, http.StatusOK)
		if got := number(t, findTrialRow(t, report, "4103"), "closing"); !nearlyEqual(got, 0) {
			t.Fatalf("profit closing after %d year close = %v, want 0", second, got)
		}
		if got := number(t, findTrialRow(t, report, "4104"), "closing") - retainedBeforeSecond; !nearlyEqual(got, -50) {
			t.Fatalf("retained earnings delta after %d year close = %v, want -50 (prior-year profit must not be transferred twice)", second, got)
		}

		ensureTestYear(t, admin, zeroYear)
		closeOpenPeriods(t, admin, zeroYear)
		zeroBefore := admin.mustGet(
			"/finance/reports/trial-balance?start_date="+fmt.Sprintf("%04d-01-01", zeroYear)+"&end_date="+fmt.Sprintf("%04d-12-31", zeroYear),
			http.StatusOK,
		)
		retainedBeforeZero := number(t, findTrialRow(t, zeroBefore, "4104"), "closing")
		admin.mustPost(yearClosePath(zeroYear), nil, http.StatusOK)
		zeroStatus := admin.mustGet(yearClosePath(zeroYear), http.StatusOK)
		if closed, _ := zeroStatus["closed"].(bool); !closed {
			t.Fatalf("zero-balance year %d close status = %v", zeroYear, zeroStatus)
		}
		if text(zeroStatus, "voucher_code") != "" {
			t.Fatalf("zero-balance year %d should not generate a voucher: %v", zeroYear, zeroStatus)
		}
		zeroEnd := fmt.Sprintf("%04d-01-01", zeroYear+1)
		afterZero := admin.mustGet("/finance/reports/trial-balance?start_date="+zeroEnd+"&end_date="+zeroEnd, http.StatusOK)
		if got := number(t, findTrialRow(t, afterZero, "4104"), "closing") - retainedBeforeZero; !nearlyEqual(got, 0) {
			t.Fatalf("retained earnings delta after zero-balance year = %v, want 0", got)
		}

		ensureTestYear(t, admin, priorOpen)
		closeOpenPeriods(t, admin, priorOpen)
		ensureTestYear(t, admin, laterOpen)
		closeOpenPeriods(t, admin, laterOpen)
		admin.expectError(http.MethodPost, yearClosePath(laterOpen), nil, http.StatusConflict, "YEAR_OUT_OF_ORDER")

		ensureTestYear(t, admin, incomplete)
		admin.expectError(http.MethodPost, yearClosePath(incomplete), nil, http.StatusConflict, "YEAR_NOT_CLOSED")
	})
}

func periodsPath(year int) string {
	return "/finance/periods?year=" + strconv.Itoa(year)
}

func yearClosePath(year int) string {
	return "/finance/years/" + strconv.Itoa(year) + "/close"
}

func ensureTestYear(t *testing.T, admin *client, year int) {
	t.Helper()
	admin.mustPost("/finance/periods/ensure", map[string]any{"year": year}, http.StatusOK)
}

func postYearRevenue(t *testing.T, admin *client, year int, amount, cashID, revenueID string) {
	t.Helper()
	periods := getArray(admin, periodsPath(year), http.StatusOK)
	var openPeriod map[string]any
	for _, period := range periods {
		if text(period, "status") == "open" {
			openPeriod = period
			break
		}
	}
	if openPeriod == nil {
		t.Fatalf("year %d has no open period for revenue voucher", year)
	}
	voucher := admin.mustPost("/finance/vouchers", map[string]any{
		"voucher_date": text(openPeriod, "start_date"),
		"memo":         fmt.Sprintf("year close revenue %d", year),
		"lines": []map[string]any{
			{"account_id": cashID, "summary": "year close cash", "debit": amount, "credit": "0"},
			{"account_id": revenueID, "summary": "year close revenue", "debit": "0", "credit": amount},
		},
	}, http.StatusCreated)
	admin.mustPost("/finance/vouchers/"+text(voucher, "id")+"/post", nil, http.StatusOK)
}

func closeOpenPeriods(t *testing.T, admin *client, year int) {
	t.Helper()
	periods := getArray(admin, periodsPath(year), http.StatusOK)
	for _, period := range periods {
		if text(period, "status") == "open" {
			admin.mustPost("/finance/periods/"+text(period, "id")+"/close", nil, http.StatusOK)
		}
	}
}

func cleanYears(t *testing.T, admin *client, count int) []int {
	t.Helper()
	maxClosed := 0
	for year := 2100; year <= 2200; year++ {
		status := admin.mustGet(yearClosePath(year), http.StatusOK)
		if closed, _ := status["closed"].(bool); closed {
			maxClosed = year
		}
	}
	base := maxClosed + 1
	if base < 2100 {
		base = 2100
	}
	for ; base+count <= 2200; base++ {
		priorPeriods := getArray(admin, periodsPath(base-1), http.StatusOK)
		priorStatus := admin.mustGet(yearClosePath(base-1), http.StatusOK)
		priorClosed, _ := priorStatus["closed"].(bool)
		if len(priorPeriods) > 0 && !priorClosed {
			continue
		}
		years := make([]int, 0, count)
		clean := true
		for year := base; year < base+count; year++ {
			periods := getArray(admin, periodsPath(year), http.StatusOK)
			status := admin.mustGet(yearClosePath(year), http.StatusOK)
			closed, _ := status["closed"].(bool)
			if len(periods) != 0 || closed {
				clean = false
				break
			}
			years = append(years, year)
		}
		if clean {
			return years
		}
	}
	return nil
}

func verifyReportScope(t *testing.T, admin *client, startDate, endDate string) {
	t.Helper()
	demoPassword := env("TEST_DEMO_PASSWORD", "Demo@123456")
	dept := newClient(t).login("demo_dept", demoPassword)
	deptReport := dept.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
	if got := number(t, findTrialRow(t, deptReport, "4103"), "closing"); !nearlyEqual(got, 0) {
		t.Fatalf("department-scoped profit closing = %v, want 0 (company closing voucher must stay out of dept scope)", got)
	}
	if got, want := number(t, deptReport, "total_debit"), number(t, deptReport, "total_credit"); !nearlyEqual(got, want) {
		t.Fatalf("department-scoped trial balance debit = %v, credit = %v, want balanced", got, want)
	}

	company := newClient(t).login("demo_company", demoPassword)
	companyReport := company.mustGet("/finance/reports/trial-balance?start_date="+startDate+"&end_date="+endDate, http.StatusOK)
	if got := number(t, findTrialRow(t, companyReport, "4103"), "closing"); nearlyEqual(got, 0) {
		t.Fatal("company-scoped profit closing is 0, want company closing voucher visible")
	}
}
