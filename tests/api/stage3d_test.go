//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

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
	})

	yearCloseYear := envInt("TEST_YEAR_CLOSE_YEAR", 0)
	if yearCloseYear > 0 {
		t.Run("year close transfers profit to retained earnings", func(t *testing.T) {
			admin.mustPost("/finance/periods/ensure", map[string]any{"year": yearCloseYear}, http.StatusOK)
			yearPeriods := getArray(admin, "/finance/periods?year="+strconv.Itoa(yearCloseYear), http.StatusOK)
			var openPeriod map[string]any
			for _, period := range yearPeriods {
				if text(period, "status") == "open" {
					openPeriod = period
					break
				}
			}
			if openPeriod != nil {
				openStart := text(openPeriod, "start_date")
				voucher := admin.mustPost("/finance/vouchers", map[string]any{
					"voucher_date": openStart,
					"memo":         "stage3d year close revenue",
					"lines": []map[string]any{
						{"account_id": cashID, "summary": "year close cash", "debit": "100.00", "credit": "0"},
						{"account_id": revenueID, "summary": "year close revenue", "debit": "0", "credit": "100.00"},
					},
				}, http.StatusCreated)
				admin.mustPost("/finance/vouchers/"+text(voucher, "id")+"/post", nil, http.StatusOK)
			}
			for _, period := range yearPeriods {
				if text(period, "status") == "open" {
					admin.mustPost("/finance/periods/"+text(period, "id")+"/close", nil, http.StatusOK)
				}
			}
			status, payload := admin.post("/finance/years/"+strconv.Itoa(yearCloseYear)+"/close", nil)
			if status == http.StatusConflict && payload.Code == "YEAR_CLOSED" {
				return
			}
			if status != http.StatusOK {
				t.Fatalf("year close status = %d, payload = %v", status, payload)
			}
			nextStart := fmt.Sprintf("%04d-01-01", yearCloseYear+1)
			report := admin.mustGet(
				"/finance/reports/trial-balance?start_date="+nextStart+"&end_date="+nextStart,
				http.StatusOK,
			)
			profitRow := findTrialRow(t, report, "4103")
			retainedRow := findTrialRow(t, report, "4104")
			if got := number(t, profitRow, "closing"); !nearlyEqual(got, 0) {
				t.Fatalf("profit closing after year close = %v, want 0", got)
			}
			if got := number(t, retainedRow, "closing"); got >= 0 {
				t.Fatalf("retained earnings closing = %v, want negative credit balance", got)
			}
		})
	}
}
