//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStage3K(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	now := time.Now()
	today := now.Format("2006-01-02")
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	ensureTestYear(t, admin, now.Year())
	periods := getArray(admin, periodsPath(now.Year()), http.StatusOK)
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
	if text(currentPeriod, "status") == "closed" {
		admin.mustPost("/finance/periods/"+text(currentPeriod, "id")+"/reopen", nil, http.StatusOK)
	}

	customer := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-001&page_size=20", http.StatusOK), "code", "CUS-DEMO-001")
	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	vat0 := findItem(listItems(admin, "/masterdata/tax-rates?keyword=VAT0&page_size=20", http.StatusOK), "code", "VAT0")
	if customer == nil || supplier == nil || warehouse == nil || material == nil || vat0 == nil {
		t.Fatal("demo master data missing")
	}
	customerID := text(customer, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	materialID := text(material, "id")
	vat0ID := text(vat0, "id")

	ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

	t.Run("full reversal receivable and payable", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

		t.Run("receivable", func(t *testing.T) {
			_, _, _, invoiceID, _ := createShippedOrder(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"1", "100.00", true, "stage3k-full-ar",
			)
			invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
			invoiceCode := text(invoice, "code")
			payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, "stage3k-full-ar-payment")
			paymentID := text(payment, "id")
			paymentCode := text(payment, "code")
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})
			stage3KAssertFullReversal(t, admin, paymentID, invoiceID, paymentCode, invoiceCode, "100.0000", today, "stage3k full receivable reversal")
		})

		t.Run("payable", func(t *testing.T) {
			invoiceID := stage3KCreatePayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "100.00", today, "stage3k-full-ap")
			invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
			invoiceCode := text(invoice, "code")
			payment := stage3KCreatePayment(t, admin, "payment", supplierID, "CNY", "", "100.00", today, "stage3k-full-ap-payment")
			paymentID := text(payment, "id")
			paymentCode := text(payment, "code")
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})
			stage3KAssertFullReversal(t, admin, paymentID, invoiceID, paymentCode, invoiceCode, "100.0000", today, "stage3k full payable reversal")
		})
	})

	t.Run("multi invoice reversal and reallocation", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceAID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-multi-a",
		)
		_, _, _, invoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-multi-b",
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "200.00", today, "stage3k-multi-payment")
		paymentID := text(payment, "id")
		stage3KAllocate(t, admin, paymentID,
			stage3JAllocation{invoiceAID, "100.00"},
			stage3JAllocation{invoiceBID, "100.00"},
		)

		reversed := stage3KUnallocate(t, admin, paymentID, today, "stage3k reverse invoice b", invoiceBID)
		stage3KExpect(t, reversed, "allocated_amount", "100.0000")
		stage3KExpect(t, reversed, "base_allocated_amount", "100.0000")
		stage3KExpect(t, reversed, "status", "partially_allocated")
		allocations := stage3KObjects(t, reversed, "allocations")
		if len(allocations) != 1 || text(allocations[0], "invoice_id") != invoiceAID {
			t.Fatalf("allocations after first reversal = %v, want only invoice %s", allocations, invoiceAID)
		}
		reversals := stage3KObjects(t, reversed, "reversals")
		if len(reversals) != 1 || text(reversals[0], "invoice_id") != invoiceBID {
			t.Fatalf("reversals after first reversal = %v, want one for %s", reversals, invoiceBID)
		}

		invoiceA := admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
		stage3KExpect(t, invoiceA, "paid_amount", "100.0000")
		stage3KExpect(t, invoiceA, "status", "paid")
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3KExpect(t, invoiceB, "paid_amount", "0.0000")
		stage3KExpect(t, invoiceB, "base_paid_amount", "0.0000")
		stage3KExpect(t, invoiceB, "outstanding_amount", "100.0000")
		stage3KExpect(t, invoiceB, "status", "open")

		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceBID, "100.00"})
		reallocated := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		stage3KExpect(t, reallocated, "allocated_amount", "200.0000")
		stage3KExpect(t, reallocated, "status", "allocated")
		invoiceB = admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3KExpect(t, invoiceB, "paid_amount", "100.0000")
		stage3KExpect(t, invoiceB, "status", "paid")

		reversedAgain := stage3KUnallocate(t, admin, paymentID, today, "stage3k reverse invoice b again", invoiceBID)
		stage3KExpect(t, reversedAgain, "allocated_amount", "100.0000")
		stage3KExpect(t, reversedAgain, "status", "partially_allocated")
		if got := len(stage3KObjects(t, reversedAgain, "allocations")); got != 1 {
			t.Fatalf("allocations after second reversal = %d, want 1", got)
		}
		reversals = stage3KObjects(t, reversedAgain, "reversals")
		if len(reversals) != 2 {
			t.Fatalf("reversals after second reversal = %d, want 2", len(reversals))
		}
		for _, reversal := range reversals {
			stage3KExpect(t, reversal, "invoice_id", invoiceBID)
			stage3KExpect(t, reversal, "amount", "100.0000")
			if codes := stage3KStringArray(t, reversal, "fx_reversal_voucher_codes"); len(codes) != 0 {
				t.Fatalf("CNY reversal fx codes = %v, want empty", codes)
			}
		}
		invoiceB = admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3KExpect(t, invoiceB, "paid_amount", "0.0000")
		stage3KExpect(t, invoiceB, "status", "open")
		invoiceA = admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
		stage3KExpect(t, invoiceA, "status", "paid")
	})

	t.Run("two payments one invoice reversal", func(t *testing.T) {
		_, _, _, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "CNY", "", today, "stage3k-two-payments-invoice",
		)
		payment1 := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "40.00", today, "stage3k-two-payments-40")
		payment1ID := text(payment1, "id")
		payment2 := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "60.00", today, "stage3k-two-payments-60")
		payment2ID := text(payment2, "id")
		stage3KAllocate(t, admin, payment1ID, stage3JAllocation{invoiceID, "40.00"})
		stage3KAllocate(t, admin, payment2ID, stage3JAllocation{invoiceID, "60.00"})

		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoice, "paid_amount", "100.0000")
		stage3KExpect(t, invoice, "status", "paid")

		first := stage3KUnallocate(t, admin, payment1ID, today, "stage3k two payments first", invoiceID)
		stage3KExpect(t, first, "allocated_amount", "0.0000")
		stage3KExpect(t, first, "status", "unallocated")
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoice, "paid_amount", "60.0000")
		stage3KExpect(t, invoice, "base_paid_amount", "60.0000")
		stage3KExpect(t, invoice, "outstanding_amount", "40.0000")
		stage3KExpect(t, invoice, "status", "partially_paid")
		payment2Detail := admin.mustGet("/finance/payments/"+payment2ID, http.StatusOK)
		stage3KExpect(t, payment2Detail, "allocated_amount", "60.0000")
		stage3KExpect(t, payment2Detail, "status", "allocated")
		if got := len(stage3KObjects(t, payment2Detail, "allocations")); got != 1 {
			t.Fatalf("second payment allocations after first reversal = %d, want 1", got)
		}

		second := stage3KUnallocate(t, admin, payment2ID, today, "stage3k two payments second", invoiceID)
		stage3KExpect(t, second, "allocated_amount", "0.0000")
		stage3KExpect(t, second, "status", "unallocated")
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoice, "paid_amount", "0.0000")
		stage3KExpect(t, invoice, "base_paid_amount", "0.0000")
		stage3KExpect(t, invoice, "outstanding_amount", "100.0000")
		stage3KExpect(t, invoice, "status", "open")
		payment2Detail = admin.mustGet("/finance/payments/"+payment2ID, http.StatusOK)
		stage3KExpect(t, payment2Detail, "allocated_amount", "0.0000")
		stage3KExpect(t, payment2Detail, "status", "unallocated")
	})

	t.Run("rounding parity multi invoice", func(t *testing.T) {
		const rate = "7.12345678"
		t.Run("one allocation call", func(t *testing.T) {
			invoiceAID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "1.0000", rate, today, "stage3k-rounding-one-a")
			invoiceBID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "1.0000", rate, today, "stage3k-rounding-one-b")
			payment := stage3KCreatePayment(t, admin, "payment", supplierID, "USD", rate, "2.0000", today, "stage3k-rounding-one-payment")
			paymentID := text(payment, "id")
			stage3KAllocate(t, admin, paymentID,
				stage3JAllocation{invoiceAID, "1.0000"},
				stage3JAllocation{invoiceBID, "1.0000"},
			)

			detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
			stage3KExpect(t, detail, "base_amount", "14.2469")
			stage3KExpect(t, detail, "allocated_amount", "2.0000")
			stage3KExpect(t, detail, "base_allocated_amount", "14.2470")
			stage3KExpect(t, detail, "status", "allocated")
			allocations := stage3KObjects(t, detail, "allocations")
			if len(allocations) != 2 {
				t.Fatalf("allocation rows = %d, want 2", len(allocations))
			}
			baseTotal := 0.0
			for _, allocation := range allocations {
				stage3KExpect(t, allocation, "amount", "1.0000")
				stage3KExpect(t, allocation, "base_amount", "7.1235")
				stage3KExpect(t, allocation, "invoice_base_amount", "7.1235")
				baseTotal += number(t, allocation, "base_amount")
			}
			if !nearlyEqual(baseTotal, 14.2470) {
				t.Fatalf("allocation base total = %v, want 14.2470", baseTotal)
			}
			for _, invoiceID := range []string{invoiceAID, invoiceBID} {
				invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
				stage3KExpect(t, invoice, "paid_amount", "1.0000")
				stage3KExpect(t, invoice, "base_paid_amount", "7.1235")
				stage3KExpect(t, invoice, "status", "paid")
			}

			reversed := stage3KUnallocate(t, admin, paymentID, today, "stage3k rounding one reversal", invoiceAID, invoiceBID)
			stage3KExpect(t, reversed, "allocated_amount", "0.0000")
			stage3KExpect(t, reversed, "base_allocated_amount", "0.0000")
			stage3KExpect(t, reversed, "status", "unallocated")
			if got := len(stage3KObjects(t, reversed, "allocations")); got != 0 {
				t.Fatalf("allocations after reversal = %d, want 0", got)
			}
			reversals := stage3KObjects(t, reversed, "reversals")
			if len(reversals) != 2 {
				t.Fatalf("reversals = %d, want 2", len(reversals))
			}
			for _, reversal := range reversals {
				if codes := stage3KStringArray(t, reversal, "fx_reversal_voucher_codes"); len(codes) != 0 {
					t.Fatalf("same-rate reversal fx codes = %v, want empty", codes)
				}
			}
			for _, invoiceID := range []string{invoiceAID, invoiceBID} {
				invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
				stage3KExpect(t, invoice, "paid_amount", "0.0000")
				stage3KExpect(t, invoice, "base_paid_amount", "0.0000")
				stage3KExpect(t, invoice, "outstanding_amount", "1.0000")
				stage3KExpect(t, invoice, "status", "open")
			}
		})

		t.Run("two allocation calls reversed one by one", func(t *testing.T) {
			invoiceAID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "1.0000", rate, today, "stage3k-rounding-two-a")
			invoiceBID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "1.0000", rate, today, "stage3k-rounding-two-b")
			payment := stage3KCreatePayment(t, admin, "payment", supplierID, "USD", rate, "2.0000", today, "stage3k-rounding-two-payment")
			paymentID := text(payment, "id")
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceAID, "1.0000"})
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceBID, "1.0000"})

			first := stage3KUnallocate(t, admin, paymentID, today, "stage3k rounding two first", invoiceAID)
			stage3KExpect(t, first, "allocated_amount", "1.0000")
			stage3KExpect(t, first, "base_allocated_amount", "7.1235")
			stage3KExpect(t, first, "status", "partially_allocated")
			invoiceA := admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
			stage3KExpect(t, invoiceA, "paid_amount", "0.0000")
			stage3KExpect(t, invoiceA, "base_paid_amount", "0.0000")
			stage3KExpect(t, invoiceA, "status", "open")
			invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
			stage3KExpect(t, invoiceB, "paid_amount", "1.0000")
			stage3KExpect(t, invoiceB, "status", "paid")

			second := stage3KUnallocate(t, admin, paymentID, today, "stage3k rounding two second", invoiceBID)
			stage3KExpect(t, second, "allocated_amount", "0.0000")
			stage3KExpect(t, second, "base_allocated_amount", "0.0000")
			stage3KExpect(t, second, "status", "unallocated")
			if got := len(stage3KObjects(t, second, "allocations")); got != 0 {
				t.Fatalf("allocations after second reversal = %d, want 0", got)
			}
			if got := len(stage3KObjects(t, second, "reversals")); got != 2 {
				t.Fatalf("reversals after second reversal = %d, want 2", got)
			}
			invoiceB = admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
			stage3KExpect(t, invoiceB, "paid_amount", "0.0000")
			stage3KExpect(t, invoiceB, "base_paid_amount", "0.0000")
			stage3KExpect(t, invoiceB, "outstanding_amount", "1.0000")
			stage3KExpect(t, invoiceB, "status", "open")
		})
	})

	t.Run("fx settlement reversal and multiple settlements", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

		invoiceID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "100.00", "7.2", today, "stage3k-fx-multi-payable")
		payment := stage3KCreatePayment(t, admin, "payment", supplierID, "USD", "7.3", "100.00", today, "stage3k-fx-multi-payment")
		paymentID := text(payment, "id")
		paymentCode := text(payment, "code")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "40.00"})
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "60.00"})

		before := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		stage3KExpect(t, before, "allocated_amount", "100.0000")
		stage3KExpect(t, before, "base_allocated_amount", "730.0000")
		stage3KExpect(t, before, "status", "allocated")
		allocationRows := stage3KObjects(t, before, "allocations")
		if len(allocationRows) != 1 {
			t.Fatalf("aggregated allocation rows = %d, want 1", len(allocationRows))
		}
		stage3KExpect(t, allocationRows[0], "amount", "100.0000")
		stage3KExpect(t, allocationRows[0], "base_amount", "730.0000")
		stage3KExpect(t, allocationRows[0], "invoice_base_amount", "720.0000")
		invoiceBefore := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoiceBefore, "paid_amount", "100.0000")
		stage3KExpect(t, invoiceBefore, "base_paid_amount", "720.0000")
		stage3KExpect(t, invoiceBefore, "status", "paid")

		originalVouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, currentStart, today), paymentCode)
		if len(originalVouchers) != 2 {
			t.Fatalf("original FX settlement vouchers = %d, want 2", len(originalVouchers))
		}
		originalsBySource := make(map[string]map[string]any, len(originalVouchers))
		for _, voucher := range originalVouchers {
			sourceID := text(voucher, "source_id")
			if sourceID == "" {
				t.Fatalf("original FX settlement voucher %s source_id is empty", text(voucher, "code"))
			}
			originalsBySource[sourceID] = voucher
			lines := stage3KVoucherLines(t, admin, voucher)
			assertVoucherBalanced(t, lines)
		}

		reversed := stage3KUnallocate(t, admin, paymentID, today, "stage3k fx multi reversal", invoiceID)
		stage3KExpect(t, reversed, "allocated_amount", "0.0000")
		stage3KExpect(t, reversed, "base_allocated_amount", "0.0000")
		stage3KExpect(t, reversed, "status", "unallocated")
		if got := len(stage3KObjects(t, reversed, "allocations")); got != 0 {
			t.Fatalf("allocations after FX reversal = %d, want 0", got)
		}
		reversals := stage3KObjects(t, reversed, "reversals")
		if len(reversals) != 1 {
			t.Fatalf("FX reversal records = %d, want 1", len(reversals))
		}
		reversal := reversals[0]
		stage3KExpect(t, reversal, "amount", "100.0000")
		stage3KExpect(t, reversal, "base_amount", "730.0000")
		stage3KExpect(t, reversal, "invoice_base_amount", "720.0000")
		codes := stage3KStringArray(t, reversal, "fx_reversal_voucher_codes")
		if len(codes) != 2 {
			t.Fatalf("fx_reversal_voucher_codes = %v, want 2 codes", codes)
		}
		if codes[0] == codes[1] {
			t.Fatalf("fx_reversal_voucher_codes has duplicate code: %v", codes)
		}

		reversalVouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement_reversal", paymentCode, currentStart, today), paymentCode)
		if len(reversalVouchers) != 2 {
			t.Fatalf("FX settlement reversal vouchers = %d, want 2", len(reversalVouchers))
		}
		byCode := make(map[string]map[string]any, len(reversalVouchers))
		for _, voucher := range reversalVouchers {
			byCode[text(voucher, "code")] = voucher
		}
		for _, code := range codes {
			voucher := byCode[code]
			if voucher == nil {
				t.Fatalf("reversal voucher %s not found in list", code)
			}
			stage3KExpect(t, voucher, "voucher_type", "fx_settlement_reversal")
			stage3KExpect(t, voucher, "source_type", "fx_settlement_reversal")
			stage3KExpect(t, voucher, "source_code", paymentCode)
			if !strings.Contains(text(voucher, "memo"), "汇兑结算反核销") {
				t.Fatalf("reversal voucher memo = %q, want 汇兑结算反核销", text(voucher, "memo"))
			}
			sourceID := text(voucher, "source_id")
			if sourceID == "" {
				t.Fatal("reversal voucher source_id is empty")
			}
			original := originalsBySource[sourceID]
			if original == nil {
				t.Fatalf("reversal voucher source_id %s does not match an original settlement", sourceID)
			}
			originalLines := stage3KVoucherLines(t, admin, original)
			originalTotal := number(t, original, "total_amount")
			if got := number(t, findLineByAccount(t, originalLines, "660301"), "debit"); !nearlyEqual(got, originalTotal) {
				t.Fatalf("original payable FX loss debit = %v, want %v", got, originalTotal)
			}
			if got := number(t, findLineByAccount(t, originalLines, "2202"), "credit"); !nearlyEqual(got, originalTotal) {
				t.Fatalf("original payable AP credit = %v, want %v", got, originalTotal)
			}
			reversalLines := stage3KVoucherLines(t, admin, voucher)
			assertVoucherBalanced(t, reversalLines)
			reversalTotal := number(t, voucher, "total_amount")
			if got := number(t, findLineByAccount(t, reversalLines, "2202"), "debit"); !nearlyEqual(got, reversalTotal) {
				t.Fatalf("reversal payable AP debit = %v, want %v", got, reversalTotal)
			}
			if got := number(t, findLineByAccount(t, reversalLines, "660302"), "credit"); !nearlyEqual(got, reversalTotal) {
				t.Fatalf("reversal payable FX gain credit = %v, want %v", got, reversalTotal)
			}
			stage3KAssertOppositeVoucherLines(t, originalLines, reversalLines)
			if !nearlyEqual(reversalTotal, originalTotal) {
				t.Fatalf("reversal total = %s, original total = %s, want equal", text(voucher, "total_amount"), text(original, "total_amount"))
			}
		}

		remainingOriginals := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, currentStart, today), paymentCode)
		if len(remainingOriginals) != 2 {
			t.Fatalf("original FX settlement vouchers after reversal = %d, want 2", len(remainingOriginals))
		}
		for _, voucher := range remainingOriginals {
			if text(voucher, "status") != "posted" {
				t.Fatalf("original FX settlement voucher %s status = %q, want posted", text(voucher, "code"), text(voucher, "status"))
			}
			if _, ok := originalsBySource[text(voucher, "source_id")]; !ok {
				t.Fatalf("original FX settlement voucher source %s changed", text(voucher, "source_id"))
			}
		}

		invoiceAfter := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoiceAfter, "paid_amount", "0.0000")
		stage3KExpect(t, invoiceAfter, "base_paid_amount", "0.0000")
		stage3KExpect(t, invoiceAfter, "outstanding_amount", "100.0000")
		stage3KExpect(t, invoiceAfter, "status", "open")

		// Receivable direction, positive adjustment: AR debit / FX gain credit.
		stage3KExerciseFXReceiptReversal(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"7.2", "7.3", today, "stage3k-fx-ar-gain",
			"1122", "660302", "660301", "1122",
		)
		// Receivable direction, negative adjustment: FX loss debit / AR credit.
		stage3KExerciseFXReceiptReversal(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"7.2", "7.1", today, "stage3k-fx-ar-loss",
			"660301", "1122", "1122", "660302",
		)
	})

	t.Run("credited invoice reversal status", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, shipmentID, shipmentLineID, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"5", "20.00", true, "stage3k-credited",
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "40.00", today, "stage3k-credited-payment")
		paymentID := text(payment, "id")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "40.00"})

		returned := postReturn(t, admin, shipmentID, shipmentLineID, "3", today, "stage3k-credited-return")
		noteID := text(returned, "credit_note_id")
		if noteID == "" {
			t.Fatalf("credit note id missing: %v", returned)
		}
		invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoice, "paid_amount", "40.0000")
		stage3KExpect(t, invoice, "credited_amount", "60.0000")
		stage3KExpect(t, invoice, "outstanding_amount", "0.0000")
		stage3KExpect(t, invoice, "status", "paid")
		noteBefore := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		stage3KExpect(t, noteBefore, "amount", "60.0000")
		stage3KExpect(t, noteBefore, "applied_amount", "60.0000")
		stage3KExpect(t, noteBefore, "remaining_amount", "0.0000")
		stage3KExpect(t, noteBefore, "status", "applied")
		noteAllocationsBefore := stage3KObjects(t, noteBefore, "allocations")
		if len(noteAllocationsBefore) != 1 {
			t.Fatalf("credit note allocations before reversal = %d, want 1", len(noteAllocationsBefore))
		}

		reversed := stage3KUnallocate(t, admin, paymentID, today, "stage3k credited reversal", invoiceID)
		stage3KExpect(t, reversed, "allocated_amount", "0.0000")
		stage3KExpect(t, reversed, "status", "unallocated")
		invoice = admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, invoice, "paid_amount", "0.0000")
		stage3KExpect(t, invoice, "base_paid_amount", "0.0000")
		stage3KExpect(t, invoice, "credited_amount", "60.0000")
		stage3KExpect(t, invoice, "base_credited_amount", "60.0000")
		stage3KExpect(t, invoice, "outstanding_amount", "40.0000")
		stage3KExpect(t, invoice, "status", "partially_paid")

		noteAfter := admin.mustGet("/finance/credit-notes/"+noteID, http.StatusOK)
		for _, key := range []string{"amount", "applied_amount", "applied_base_amount", "refunded_amount", "remaining_amount", "remaining_base_amount", "status"} {
			if text(noteAfter, key) != text(noteBefore, key) {
				t.Fatalf("credit note %s after reversal = %q, before = %q", key, text(noteAfter, key), text(noteBefore, key))
			}
		}
		noteAllocationsAfter := stage3KObjects(t, noteAfter, "allocations")
		if len(noteAllocationsAfter) != len(noteAllocationsBefore) {
			t.Fatalf("credit note allocations after reversal = %d, want %d", len(noteAllocationsAfter), len(noteAllocationsBefore))
		}
		invoiceDetail := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := len(stage3KObjects(t, invoiceDetail, "credit_note_allocations")); got != 1 {
			t.Fatalf("invoice credit note allocations after reversal = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, invoiceDetail, "payment_allocation_reversals")); got != 1 {
			t.Fatalf("invoice payment allocation reversals = %d, want 1", got)
		}
	})

	t.Run("closed period reversal rolls back", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		marchStart := fmt.Sprintf("%04d-03-01", year)
		marchEnd := fmt.Sprintf("%04d-03-31", year)
		reversalDate := fmt.Sprintf("%04d-03-16", year)
		paymentDate := fmt.Sprintf("%04d-03-15", year)

		invoiceID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "100.00", "7.2", paymentDate, fmt.Sprintf("stage3k-closed-%d", year))
		payment := stage3KCreatePayment(t, admin, "payment", supplierID, "USD", "7.3", "100.00", paymentDate, fmt.Sprintf("stage3k-closed-payment-%d", year))
		paymentID := text(payment, "id")
		paymentCode := text(payment, "code")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})

		beforePayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		beforeInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, beforePayment, "status", "allocated")
		stage3KExpect(t, beforePayment, "allocated_amount", "100.0000")
		stage3KExpect(t, beforeInvoice, "status", "paid")

		var marchPeriod map[string]any
		for _, period := range getArray(admin, periodsPath(year), http.StatusOK) {
			if month, ok := period["month"].(float64); ok && int(month) == 3 {
				marchPeriod = period
				break
			}
		}
		if marchPeriod == nil {
			t.Fatalf("March %d period not found", year)
		}
		marchPeriodID := text(marchPeriod, "id")
		if text(marchPeriod, "status") == "closed" {
			admin.mustPost("/finance/periods/"+marchPeriodID+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+marchPeriodID+"/close", nil, http.StatusOK)
		vouchersBefore := admin.mustGet("/finance/vouchers?start_date="+marchStart+"&end_date="+marchEnd+"&page_size=200", http.StatusOK)
		beforeTotal, _ := vouchersBefore["total"].(float64)

		admin.expectError(http.MethodPost, "/finance/payments/"+paymentID+"/unallocate", stage3KUnallocateBody([]string{invoiceID}, reversalDate, "stage3k closed period reversal"), http.StatusConflict, "PERIOD_CLOSED")

		vouchersAfter := admin.mustGet("/finance/vouchers?start_date="+marchStart+"&end_date="+marchEnd+"&page_size=200", http.StatusOK)
		afterTotal, _ := vouchersAfter["total"].(float64)
		if afterTotal != beforeTotal {
			t.Fatalf("voucher total after closed period rejection = %v, before = %v, want unchanged", afterTotal, beforeTotal)
		}

		afterPayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		stage3KAssertSameFields(t, "payment after closed period rejection", beforePayment, afterPayment,
			"allocated_amount", "base_allocated_amount", "status")
		if got := len(stage3KObjects(t, afterPayment, "allocations")); got != 1 {
			t.Fatalf("allocations after closed period rejection = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, afterPayment, "reversals")); got != 0 {
			t.Fatalf("reversals after closed period rejection = %d, want 0", got)
		}
		afterInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KAssertSameFields(t, "invoice after closed period rejection", beforeInvoice, afterInvoice,
			"paid_amount", "base_paid_amount", "outstanding_amount", "status")

		reversalVouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement_reversal", paymentCode, marchStart, marchEnd), paymentCode)
		if len(reversalVouchers) != 0 {
			t.Fatalf("FX reversal vouchers after closed period rejection = %d, want 0", len(reversalVouchers))
		}
		originalVouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, marchStart, marchEnd), paymentCode)
		if len(originalVouchers) != 1 {
			t.Fatalf("original FX settlement vouchers after closed period rejection = %d, want 1", len(originalVouchers))
		}

		admin.mustPost("/finance/periods/"+marchPeriodID+"/reopen", nil, http.StatusOK)
		recovered := stage3KUnallocate(t, admin, paymentID, reversalDate, "stage3k closed period recovered", invoiceID)
		stage3KExpect(t, recovered, "allocated_amount", "0.0000")
		stage3KExpect(t, recovered, "status", "unallocated")
		recoveredReversals := stage3KObjects(t, recovered, "reversals")
		if len(recoveredReversals) != 1 {
			t.Fatalf("reversals after reopen = %d, want 1", len(recoveredReversals))
		}
		if codes := stage3KStringArray(t, recoveredReversals[0], "fx_reversal_voucher_codes"); len(codes) != 1 {
			t.Fatalf("FX reversal codes after reopen = %v, want 1", codes)
		}
		recoveredInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, recoveredInvoice, "paid_amount", "0.0000")
		stage3KExpect(t, recoveredInvoice, "base_paid_amount", "0.0000")
		stage3KExpect(t, recoveredInvoice, "status", "open")
	})

	t.Run("closed period reversal without FX rolls back", func(t *testing.T) {
		year := cleanRevaluationYear(t, admin)
		ensureTestYear(t, admin, year)
		marchStart := fmt.Sprintf("%04d-03-01", year)
		marchEnd := fmt.Sprintf("%04d-03-31", year)
		paymentDate := fmt.Sprintf("%04d-03-15", year)
		reversalDate := fmt.Sprintf("%04d-03-16", year)

		_, _, _, invoiceID := stage3JCreateShippedInvoice(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", "CNY", "", paymentDate, fmt.Sprintf("stage3k-closed-cny-%d", year),
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", paymentDate, fmt.Sprintf("stage3k-closed-cny-payment-%d", year))
		paymentID := text(payment, "id")
		paymentCode := text(payment, "code")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})

		// A same-currency allocation at rate 1 has no FX settlement, so the
		// rejected reversal below can only be stopped by CheckPeriodOpen.
		if got := len(stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, marchStart, marchEnd), paymentCode)); got != 0 {
			t.Fatalf("CNY allocation FX settlement vouchers = %d, want 0", got)
		}

		beforePayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		beforeInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KExpect(t, beforePayment, "status", "allocated")
		stage3KExpect(t, beforePayment, "allocated_amount", "100.0000")
		stage3KExpect(t, beforeInvoice, "status", "paid")
		stage3KExpect(t, beforeInvoice, "paid_amount", "100.0000")

		var marchPeriod map[string]any
		for _, period := range getArray(admin, periodsPath(year), http.StatusOK) {
			if month, ok := period["month"].(float64); ok && int(month) == 3 {
				marchPeriod = period
				break
			}
		}
		if marchPeriod == nil {
			t.Fatalf("March %d period not found", year)
		}
		marchPeriodID := text(marchPeriod, "id")
		if text(marchPeriod, "status") == "closed" {
			admin.mustPost("/finance/periods/"+marchPeriodID+"/reopen", nil, http.StatusOK)
		}
		admin.mustPost("/finance/periods/"+marchPeriodID+"/close", nil, http.StatusOK)
		vouchersBefore := admin.mustGet("/finance/vouchers?start_date="+marchStart+"&end_date="+marchEnd+"&page_size=200", http.StatusOK)
		beforeTotal, _ := vouchersBefore["total"].(float64)

		admin.expectError(http.MethodPost, "/finance/payments/"+paymentID+"/unallocate", stage3KUnallocateBody([]string{invoiceID}, reversalDate, "stage3k closed CNY reversal"), http.StatusConflict, "PERIOD_CLOSED")

		vouchersAfter := admin.mustGet("/finance/vouchers?start_date="+marchStart+"&end_date="+marchEnd+"&page_size=200", http.StatusOK)
		afterTotal, _ := vouchersAfter["total"].(float64)
		if afterTotal != beforeTotal {
			t.Fatalf("voucher total after closed period rejection = %v, before = %v, want unchanged", afterTotal, beforeTotal)
		}
		if got := len(stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement_reversal", paymentCode, marchStart, marchEnd), paymentCode)); got != 0 {
			t.Fatalf("FX reversal vouchers after closed period rejection = %d, want 0", got)
		}

		afterPayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		stage3KAssertSameFields(t, "payment after closed CNY rejection", beforePayment, afterPayment,
			"allocated_amount", "base_allocated_amount", "status")
		if got := len(stage3KObjects(t, afterPayment, "allocations")); got != 1 {
			t.Fatalf("allocations after closed CNY rejection = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, afterPayment, "reversals")); got != 0 {
			t.Fatalf("reversals after closed CNY rejection = %d, want 0", got)
		}
		afterInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		stage3KAssertSameFields(t, "invoice after closed CNY rejection", beforeInvoice, afterInvoice,
			"paid_amount", "base_paid_amount", "outstanding_amount", "status")
		if got := len(stage3KObjects(t, afterInvoice, "payment_allocation_reversals")); got != 0 {
			t.Fatalf("invoice payment allocation reversals after closed CNY rejection = %d, want 0", got)
		}

		admin.mustPost("/finance/periods/"+marchPeriodID+"/reopen", nil, http.StatusOK)
	})

	t.Run("validation", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceAID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-validation-a",
		)
		_, _, _, invoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-validation-b",
		)
		payableID := stage3KCreatePayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "100.00", today, "stage3k-validation-payable")
		paymentA := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, "stage3k-validation-payment-a")
		paymentAID := text(paymentA, "id")
		path := "/finance/payments/" + paymentAID + "/unallocate"
		stage3KAllocate(t, admin, paymentAID, stage3JAllocation{invoiceAID, "100.00"})

		beforePayment := admin.mustGet("/finance/payments/"+paymentAID, http.StatusOK)

		admin.expectError(http.MethodPost, path, stage3KUnallocateBody(nil, today, "stage3k empty"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceAID}, today, ""), http.StatusBadRequest, "REASON_REQUIRED")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceAID}, today, "   "), http.StatusBadRequest, "REASON_REQUIRED")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{" "}, today, "stage3k blank invoice"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{"not-a-uuid"}, today, "stage3k malformed"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{
			invoiceAID,
			strings.ToUpper(invoiceAID),
			"{" + invoiceAID + "}",
		}, today, "stage3k duplicate"), http.StatusBadRequest, "DUPLICATE_INVOICE")
		admin.expectError(http.MethodPost, "/finance/payments/00000000-0000-0000-0000-000000000199/unallocate", stage3KUnallocateBody([]string{invoiceAID}, today, "stage3k unknown payment"), http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceAID, "00000000-0000-0000-0000-0000000000ff"}, today, "stage3k unknown invoice"), http.StatusNotFound, "NOT_FOUND")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceBID}, today, "stage3k not allocated"), http.StatusConflict, "ALLOCATION_NOT_FOUND")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{payableID}, today, "stage3k type mismatch"), http.StatusBadRequest, "PAYMENT_TYPE_MISMATCH")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceAID}, now.AddDate(0, 0, -1).Format("2006-01-02"), "stage3k before payment"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceAID}, "not-a-date", "stage3k bad date"), http.StatusBadRequest, "BAD_REQUEST")

		paymentU := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, "stage3k-validation-unallocated")
		admin.expectError(http.MethodPost, "/finance/payments/"+text(paymentU, "id")+"/unallocate", stage3KUnallocateBody([]string{invoiceAID}, today, "stage3k payment unallocated"), http.StatusConflict, "PAYMENT_NOT_ALLOCATED")

		afterPayment := admin.mustGet("/finance/payments/"+paymentAID, http.StatusOK)
		stage3KAssertSameFields(t, "payment after rejected validations", beforePayment, afterPayment,
			"allocated_amount", "base_allocated_amount", "status")
		if got := len(stage3KObjects(t, afterPayment, "allocations")); got != 1 {
			t.Fatalf("payment A allocations after rejected validations = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, afterPayment, "reversals")); got != 0 {
			t.Fatalf("payment A reversals after rejected validations = %d, want 0", got)
		}
		invoiceA := admin.mustGet("/finance/invoices/"+invoiceAID, http.StatusOK)
		stage3KExpect(t, invoiceA, "paid_amount", "100.0000")
		stage3KExpect(t, invoiceA, "status", "paid")
		invoiceB := admin.mustGet("/finance/invoices/"+invoiceBID, http.StatusOK)
		stage3KExpect(t, invoiceB, "status", "open")

		_, _, _, invoiceCID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-validation-c",
		)
		_, _, _, invoiceDID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-validation-d",
		)
		paymentMulti := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "200.00", today, "stage3k-validation-multi")
		paymentMultiID := text(paymentMulti, "id")
		stage3KAllocate(t, admin, paymentMultiID,
			stage3JAllocation{invoiceCID, "100.00"},
			stage3JAllocation{invoiceDID, "100.00"},
		)
		stage3KUnallocate(t, admin, paymentMultiID, today, "stage3k already reversed first", invoiceCID)
		admin.expectError(http.MethodPost, "/finance/payments/"+paymentMultiID+"/unallocate", stage3KUnallocateBody([]string{invoiceCID}, today, "stage3k already reversed"), http.StatusConflict, "ALLOCATION_NOT_FOUND")
		afterMulti := admin.mustGet("/finance/payments/"+paymentMultiID, http.StatusOK)
		stage3KExpect(t, afterMulti, "allocated_amount", "100.0000")
		stage3KExpect(t, afterMulti, "status", "partially_allocated")
		if got := len(stage3KObjects(t, afterMulti, "reversals")); got != 1 {
			t.Fatalf("payment multi reversals after duplicate reversal rejection = %d, want 1", got)
		}

		// Closed-period rejection and rollback are covered by the
		// "closed period reversal rolls back" subtest.
	})

	t.Run("concurrency", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)

		t.Run("concurrent double reversal succeeds once", func(t *testing.T) {
			invoiceID := stage3KCreateUSDPayable(t, admin, supplierID, warehouseID, materialID, vat0ID, "100.00", "7.2", today, "stage3k-concurrent-same-invoice")
			payment := stage3KCreatePayment(t, admin, "payment", supplierID, "USD", "7.3", "100.00", today, "stage3k-concurrent-same-payment")
			paymentID := text(payment, "id")
			paymentCode := text(payment, "code")
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "40.00"})
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "60.00"})

			path := "/finance/payments/" + paymentID + "/unallocate"
			body := stage3KUnallocateBody([]string{invoiceID}, today, "stage3k concurrent double")
			results := stage3JRunConcurrent(admin, []stage3JRequest{{path: path, body: body}, {path: path, body: body}})
			successes := 0
			conflicts := 0
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent reversal %d failed: %v", i, result.err)
				}
				switch result.status {
				case http.StatusOK:
					successes++
				case http.StatusConflict:
					if result.code != "PAYMENT_NOT_ALLOCATED" && result.code != "ALLOCATION_NOT_FOUND" {
						t.Fatalf("concurrent reversal loser code = %q, want PAYMENT_NOT_ALLOCATED or ALLOCATION_NOT_FOUND", result.code)
					}
					conflicts++
				default:
					t.Fatalf("concurrent reversal %d status = %d, code = %q", i, result.status, result.code)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("concurrent double reversal success/conflict = %d/%d, want 1/1", successes, conflicts)
			}

			detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
			stage3KExpect(t, detail, "status", "unallocated")
			stage3KExpect(t, detail, "allocated_amount", "0.0000")
			reversals := stage3KObjects(t, detail, "reversals")
			if len(reversals) != 1 {
				t.Fatalf("concurrent double reversal records = %d, want 1", len(reversals))
			}
			codes := stage3KStringArray(t, reversals[0], "fx_reversal_voucher_codes")
			if len(codes) != 2 {
				t.Fatalf("concurrent double reversal FX codes = %v, want 2", codes)
			}
			vouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement_reversal", paymentCode, currentStart, today), paymentCode)
			if len(vouchers) != 2 {
				t.Fatalf("concurrent double reversal vouchers = %d, want 2", len(vouchers))
			}
		})

		t.Run("concurrent different invoices both succeed", func(t *testing.T) {
			_, _, _, invoiceAID, _ := createShippedOrder(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"1", "100.00", true, "stage3k-concurrent-different-a",
			)
			_, _, _, invoiceBID, _ := createShippedOrder(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"1", "100.00", true, "stage3k-concurrent-different-b",
			)
			payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "200.00", today, "stage3k-concurrent-different-payment")
			paymentID := text(payment, "id")
			stage3KAllocate(t, admin, paymentID,
				stage3JAllocation{invoiceAID, "100.00"},
				stage3JAllocation{invoiceBID, "100.00"},
			)
			path := "/finance/payments/" + paymentID + "/unallocate"
			results := stage3JRunConcurrent(admin, []stage3JRequest{
				{path: path, body: stage3KUnallocateBody([]string{invoiceAID}, today, "stage3k concurrent invoice a")},
				{path: path, body: stage3KUnallocateBody([]string{invoiceBID}, today, "stage3k concurrent invoice b")},
			})
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent different invoice %d failed: %v", i, result.err)
				}
				if result.status != http.StatusOK {
					t.Fatalf("concurrent different invoice %d status = %d, code = %q", i, result.status, result.code)
				}
			}
			detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
			stage3KExpect(t, detail, "allocated_amount", "0.0000")
			stage3KExpect(t, detail, "status", "unallocated")
			if got := len(stage3KObjects(t, detail, "allocations")); got != 0 {
				t.Fatalf("allocations after concurrent different invoices = %d, want 0", got)
			}
			if got := len(stage3KObjects(t, detail, "reversals")); got != 2 {
				t.Fatalf("reversals after concurrent different invoices = %d, want 2", got)
			}
			for _, invoiceID := range []string{invoiceAID, invoiceBID} {
				invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
				stage3KExpect(t, invoice, "paid_amount", "0.0000")
				stage3KExpect(t, invoice, "status", "open")
			}
		})

		t.Run("concurrent allocate and unallocate invariant", func(t *testing.T) {
			_, _, _, invoiceID, _ := createShippedOrder(
				t, admin, customerID, warehouseID, materialID, vat0ID,
				"1", "100.00", true, "stage3k-concurrent-mixed",
			)
			payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "200.00", today, "stage3k-concurrent-mixed-payment")
			paymentID := text(payment, "id")
			stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})

			path := "/finance/payments/" + paymentID + "/unallocate"
			results := stage3JRunConcurrent(admin, []stage3JRequest{
				{path: "/finance/payments/" + paymentID + "/allocate", body: map[string]any{
					"allocations": []map[string]any{{"invoice_id": invoiceID, "amount": "100.00"}},
				}},
				{path: path, body: stage3KUnallocateBody([]string{invoiceID}, today, "stage3k concurrent mixed")},
			})
			successes := 0
			for i, result := range results {
				if result.err != nil {
					t.Fatalf("concurrent mixed request %d failed: %v", i, result.err)
				}
				switch result.status {
				case http.StatusOK:
					successes++
				case http.StatusBadRequest:
					if result.code != "OVER_ALLOCATE" {
						t.Fatalf("concurrent mixed request %d code = %q, want OVER_ALLOCATE", i, result.code)
					}
				case http.StatusConflict:
					if result.code != "PAYMENT_ALLOCATED" && result.code != "PAYMENT_NOT_ALLOCATED" && result.code != "ALLOCATION_NOT_FOUND" && result.code != "INVOICE_PAID" {
						t.Fatalf("concurrent mixed request %d code = %q", i, result.code)
					}
				default:
					t.Fatalf("concurrent mixed request %d status = %d, code = %q", i, result.status, result.code)
				}
			}
			if successes == 0 {
				t.Fatal("concurrent allocate/unallocate had no successful request")
			}

			detail := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
			allocations := stage3KObjects(t, detail, "allocations")
			allocated := 0.0
			baseAllocated := 0.0
			for _, allocation := range allocations {
				allocated += number(t, allocation, "amount")
				baseAllocated += number(t, allocation, "base_amount")
			}
			if !nearlyEqual(number(t, detail, "allocated_amount"), allocated) {
				t.Fatalf("payment allocated = %v, allocation rows = %v", number(t, detail, "allocated_amount"), allocated)
			}
			if !nearlyEqual(number(t, detail, "base_allocated_amount"), baseAllocated) {
				t.Fatalf("payment base allocated = %v, allocation rows = %v", number(t, detail, "base_allocated_amount"), baseAllocated)
			}
			if got := len(stage3KObjects(t, detail, "reversals")); got != 1 {
				t.Fatalf("concurrent mixed reversal records = %d, want 1", got)
			}
			amount := number(t, detail, "amount")
			wantStatus := "unallocated"
			if allocated > 0 {
				if nearlyEqual(allocated, amount) {
					wantStatus = "allocated"
				} else {
					wantStatus = "partially_allocated"
				}
			}
			stage3KExpect(t, detail, "status", wantStatus)
			if !nearlyEqual(number(t, detail, "unallocated_amount"), amount-allocated) {
				t.Fatalf("payment unallocated = %v, want %v", number(t, detail, "unallocated_amount"), amount-allocated)
			}

			invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
			invoiceAllocations := stage3KObjects(t, invoice, "allocations")
			invoicePaid := 0.0
			for _, allocation := range invoiceAllocations {
				invoicePaid += number(t, allocation, "amount")
			}
			if !nearlyEqual(number(t, invoice, "paid_amount"), invoicePaid) {
				t.Fatalf("invoice paid = %v, invoice allocation rows = %v", number(t, invoice, "paid_amount"), invoicePaid)
			}
			wantInvoiceStatus := "open"
			if invoicePaid > 0 {
				wantInvoiceStatus = "paid"
			}
			stage3KExpect(t, invoice, "status", wantInvoiceStatus)
		})
	})

	t.Run("detail and scope/permissions", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-detail",
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, "stage3k-detail-payment")
		paymentID := text(payment, "id")
		paymentCode := text(payment, "code")
		invoiceCode := text(admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK), "code")

		emptyPayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		if got := len(stage3KObjects(t, emptyPayment, "allocations")); got != 0 {
			t.Fatalf("empty payment allocations = %d, want 0", got)
		}
		if got := len(stage3KObjects(t, emptyPayment, "reversals")); got != 0 {
			t.Fatalf("empty payment reversals = %d, want 0", got)
		}
		emptyInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := len(stage3KObjects(t, emptyInvoice, "payment_allocation_reversals")); got != 0 {
			t.Fatalf("empty invoice payment allocation reversals = %d, want 0", got)
		}

		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})
		allocatedPayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		if got := len(stage3KObjects(t, allocatedPayment, "allocations")); got != 1 {
			t.Fatalf("allocated payment allocation rows = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, allocatedPayment, "reversals")); got != 0 {
			t.Fatalf("allocated payment reversal rows = %d, want 0", got)
		}
		allocatedInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		if got := len(stage3KObjects(t, allocatedInvoice, "allocations")); got != 1 {
			t.Fatalf("allocated invoice payment allocations = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, allocatedInvoice, "payment_allocation_reversals")); got != 0 {
			t.Fatalf("allocated invoice payment allocation reversals = %d, want 0", got)
		}

		reversed := stage3KUnallocate(t, admin, paymentID, today, "stage3k detail reversal", invoiceID)
		reversals := stage3KObjects(t, reversed, "reversals")
		if len(reversals) != 1 {
			t.Fatalf("payment reversal rows = %d, want 1", len(reversals))
		}
		stage3KExpect(t, reversals[0], "payment_code", paymentCode)
		stage3KExpect(t, reversals[0], "invoice_code", invoiceCode)
		if got := len(stage3KObjects(t, reversed, "allocations")); got != 0 {
			t.Fatalf("payment allocations after reversal = %d, want 0", got)
		}
		invoiceDetail := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
		invoiceReversals := stage3KObjects(t, invoiceDetail, "payment_allocation_reversals")
		if len(invoiceReversals) != 1 {
			t.Fatalf("invoice payment allocation reversals = %d, want 1", len(invoiceReversals))
		}
		stage3KExpect(t, invoiceReversals[0], "payment_code", paymentCode)
		stage3KExpect(t, invoiceReversals[0], "invoice_code", invoiceCode)

		rows := listItems(admin, "/finance/payments?keyword="+url.QueryEscape("stage3k-detail-payment")+"&page_size=20", http.StatusOK)
		row := findByID(rows, paymentID)
		if row == nil {
			t.Fatalf("payment %s not found in list", paymentID)
		}
		if _, ok := row["allocations"]; ok {
			t.Fatal("payment list row unexpectedly contains allocations")
		}
		if _, ok := row["reversals"]; ok {
			t.Fatal("payment list row unexpectedly contains reversals")
		}

		path := "/finance/payments/" + paymentID + "/unallocate"
		body := stage3KUnallocateBody([]string{invoiceID}, today, "stage3k permission")
		anonymous := newClient(t)
		anonymous.expectError(http.MethodPost, path, body, http.StatusUnauthorized, "UNAUTHORIZED")

		dept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))
		dept.expectError(http.MethodPost, path, body, http.StatusForbidden, "FORBIDDEN")

		status, payload := dept.request(http.MethodGet, "/finance/payments/"+paymentID, nil, dept.token)
		if status != http.StatusForbidden && status != http.StatusNotFound {
			t.Fatalf("demo_dept payment detail status = %d, want 403/404, payload = %v", status, payload)
		}
		status, payload = dept.request(http.MethodGet, "/finance/invoices/"+invoiceID, nil, dept.token)
		if status != http.StatusForbidden && status != http.StatusNotFound {
			t.Fatalf("demo_dept invoice detail status = %d, want 403/404, payload = %v", status, payload)
		}
		status, payload = dept.request(http.MethodGet, "/finance/payments?page_size=200", nil, dept.token)
		switch status {
		case http.StatusForbidden:
		case http.StatusOK:
			rawItems, _ := payload.Data["items"].([]any)
			for _, raw := range rawItems {
				item, ok := raw.(map[string]any)
				if ok && text(item, "id") == paymentID {
					t.Fatalf("demo_dept can see admin payment %s outside its scope", paymentID)
				}
			}
		default:
			t.Fatalf("demo_dept payment list status = %d, want 200/403, payload = %v", status, payload)
		}

		t.Run("self scope hides cross-creator details", func(t *testing.T) {
			self := stage3KSelfScopeClient(t, admin)
			self.expectError(http.MethodGet, "/finance/payments/"+paymentID, nil, http.StatusNotFound, "NOT_FOUND")
			self.expectError(http.MethodGet, "/finance/invoices/"+invoiceID, nil, http.StatusNotFound, "NOT_FOUND")
		})

		// The self-scope account above proves negative isolation. Positive
		// cross-creator parent visibility (own payment + other creator's
		// invoice, and the reverse) remains an API regression gap: the user API
		// cannot set data_scope, and constructing that graph needs broader write
		// permissions. Admin detail above validates the parent-scoped arrays;
		// controller scoping is covered by unit tests.
	})

	t.Run("reversal date and failure rollback", func(t *testing.T) {
		ensureStage3HStock(t, admin, supplierID, warehouseID, materialID, today)
		_, _, _, invoiceID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-date",
		)
		payment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "100.00", today, "stage3k-date-payment")
		paymentID := text(payment, "id")
		stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})
		path := "/finance/payments/" + paymentID + "/unallocate"

		before := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceID}, now.AddDate(0, 0, -1).Format("2006-01-02"), "stage3k date before payment"), http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, path, stage3KUnallocateBody([]string{invoiceID}, "not-a-date", "stage3k date invalid"), http.StatusBadRequest, "BAD_REQUEST")
		afterRejected := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
		stage3KAssertSameFields(t, "payment after rejected reversal dates", before, afterRejected,
			"allocated_amount", "base_allocated_amount", "status")
		if got := len(stage3KObjects(t, afterRejected, "allocations")); got != 1 {
			t.Fatalf("allocations after rejected reversal dates = %d, want 1", got)
		}
		if got := len(stage3KObjects(t, afterRejected, "reversals")); got != 0 {
			t.Fatalf("reversals after rejected reversal dates = %d, want 0", got)
		}

		defaulted := stage3KUnallocate(t, admin, paymentID, "", "stage3k default reversal date", invoiceID)
		defaultedReversals := stage3KObjects(t, defaulted, "reversals")
		if len(defaultedReversals) != 1 {
			t.Fatalf("defaulted date reversals = %d, want 1", len(defaultedReversals))
		}
		stage3KExpect(t, defaultedReversals[0], "reversal_date", today)
		stage3KExpect(t, defaulted, "status", "unallocated")

		_, _, _, rollbackInvoiceAID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-rollback-a",
		)
		_, _, _, rollbackInvoiceBID, _ := createShippedOrder(
			t, admin, customerID, warehouseID, materialID, vat0ID,
			"1", "100.00", true, "stage3k-rollback-b",
		)
		rollbackPayment := stage3KCreatePayment(t, admin, "receipt", customerID, "CNY", "", "200.00", today, "stage3k-rollback-payment")
		rollbackPaymentID := text(rollbackPayment, "id")
		stage3KAllocate(t, admin, rollbackPaymentID,
			stage3JAllocation{rollbackInvoiceAID, "100.00"},
			stage3JAllocation{rollbackInvoiceBID, "100.00"},
		)
		rollbackBefore := admin.mustGet("/finance/payments/"+rollbackPaymentID, http.StatusOK)
		rollbackInvoiceABefore := admin.mustGet("/finance/invoices/"+rollbackInvoiceAID, http.StatusOK)
		rollbackInvoiceBBefore := admin.mustGet("/finance/invoices/"+rollbackInvoiceBID, http.StatusOK)

		admin.expectError(http.MethodPost, "/finance/payments/"+rollbackPaymentID+"/unallocate", stage3KUnallocateBody([]string{
			rollbackInvoiceAID,
			"00000000-0000-0000-0000-0000000000ee",
		}, today, "stage3k rollback unknown invoice"), http.StatusNotFound, "NOT_FOUND")

		rollbackAfter := admin.mustGet("/finance/payments/"+rollbackPaymentID, http.StatusOK)
		stage3KAssertSameFields(t, "payment after failed multi-invoice reversal", rollbackBefore, rollbackAfter,
			"allocated_amount", "base_allocated_amount", "status")
		if got := len(stage3KObjects(t, rollbackAfter, "allocations")); got != 2 {
			t.Fatalf("allocations after failed multi-invoice reversal = %d, want 2", got)
		}
		if got := len(stage3KObjects(t, rollbackAfter, "reversals")); got != 0 {
			t.Fatalf("reversals after failed multi-invoice reversal = %d, want 0", got)
		}
		rollbackInvoiceAAfter := admin.mustGet("/finance/invoices/"+rollbackInvoiceAID, http.StatusOK)
		stage3KAssertSameFields(t, "invoice A after failed multi-invoice reversal", rollbackInvoiceABefore, rollbackInvoiceAAfter,
			"paid_amount", "base_paid_amount", "outstanding_amount", "status")
		rollbackInvoiceBAfter := admin.mustGet("/finance/invoices/"+rollbackInvoiceBID, http.StatusOK)
		stage3KAssertSameFields(t, "invoice B after failed multi-invoice reversal", rollbackInvoiceBBefore, rollbackInvoiceBAfter,
			"paid_amount", "base_paid_amount", "outstanding_amount", "status")

		// FX reversal GL failure on a closed period is covered by the
		// "closed period reversal rolls back" subtest, including no partial
		// payment, invoice or voucher changes.
	})
}

func stage3KExpect(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	if got := text(m, key); got != want {
		t.Fatalf("%s = %q, want %q (payload = %v)", key, got, want, m)
	}
}

func stage3KObjects(t *testing.T, data map[string]any, key string) []map[string]any {
	t.Helper()
	return stage3JObjects(t, data, key)
}

func stage3KStringArray(t *testing.T, data map[string]any, key string) []string {
	t.Helper()
	raw, ok := data[key]
	if !ok {
		t.Fatalf("field %s is missing", key)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("field %s = %v, want array", key, raw)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("field %s entry = %v, want string", key, item)
		}
		values = append(values, value)
	}
	return values
}

func stage3KUnallocateBody(invoiceIDs []string, date, reason string) map[string]any {
	ids := invoiceIDs
	if ids == nil {
		ids = []string{}
	}
	body := map[string]any{
		"invoice_ids": ids,
		"reason":      reason,
	}
	if date != "" {
		body["reversal_date"] = date
	}
	return body
}

func stage3KCreatePayment(
	t *testing.T, admin *client,
	paymentType, partnerID, currency, exchangeRate, amount, date, reference string,
) map[string]any {
	t.Helper()
	body := map[string]any{
		"payment_type": paymentType,
		"partner_id":   partnerID,
		"payment_date": date,
		"currency":     currency,
		"amount":       amount,
		"method":       "bank",
		"reference":    reference,
	}
	if exchangeRate != "" {
		body["exchange_rate"] = exchangeRate
	}
	return admin.mustPost("/finance/payments", body, http.StatusCreated)
}

func stage3KPut(t *testing.T, admin *client, path string, body any) map[string]any {
	t.Helper()
	status, payload := admin.put(path, body)
	if status != http.StatusOK {
		t.Fatalf("PUT %s status = %d, want %d, payload = %v", path, status, http.StatusOK, payload)
	}
	return payload.Data
}

// stage3KSelfScopeClient creates (idempotently) a role and user with only the
// finance read/reverse permissions and no org_memberships row. The auth
// middleware therefore falls back to data_scope=self for this user.
func stage3KSelfScopeClient(t *testing.T, admin *client) *client {
	t.Helper()

	hq := findItem(getArray(admin, "/system/orgs", http.StatusOK), "code", "HQ")
	if hq == nil {
		t.Fatal("HQ organization not found")
	}
	hqID := text(hq, "id")

	const roleCode = "stage3k_self_scope"
	var roleID string
	if role := findItem(getArray(admin, "/system/roles", http.StatusOK), "code", roleCode); role != nil {
		roleID = text(role, "id")
	} else {
		roleID = text(admin.mustPost("/system/roles", map[string]any{
			"code": roleCode,
			"name": "Stage 3K self scope",
		}, http.StatusCreated), "id")
	}
	stage3KPut(t, admin, "/system/roles/"+roleID+"/permissions", map[string]any{
		"permission_codes": []string{
			"finance.payment.read",
			"finance.payment.reverse",
			"finance.invoice.read",
		},
	})

	const username = "stage3k_self_scope"
	users := listItems(admin, "/system/users?keyword="+url.QueryEscape(username)+"&page_size=20", http.StatusOK)
	if user := findItem(users, "username", username); user != nil {
		userID := text(user, "id")
		stage3KPut(t, admin, "/system/users/"+userID, map[string]any{
			"display_name":   "Stage 3K self scope",
			"org_id":         hqID,
			"status":         "active",
			"password":       "Demo@123456",
			"is_super_admin": false,
		})
		admin.mustPost("/system/users/"+userID+"/roles", map[string]any{"role_ids": []string{roleID}}, http.StatusOK)
	} else {
		admin.mustPost("/system/users", map[string]any{
			"username":       username,
			"display_name":   "Stage 3K self scope",
			"password":       "Demo@123456",
			"org_id":         hqID,
			"is_super_admin": false,
			"role_ids":       []string{roleID},
		}, http.StatusCreated)
	}

	return newClient(t).login(username, "Demo@123456")
}

func stage3KAllocate(t *testing.T, admin *client, paymentID string, entries ...stage3JAllocation) map[string]any {
	t.Helper()
	rows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, map[string]any{"invoice_id": entry.invoiceID, "amount": entry.amount})
	}
	return admin.mustPost("/finance/payments/"+paymentID+"/allocate", map[string]any{"allocations": rows}, http.StatusOK)
}

func stage3KUnallocate(
	t *testing.T, admin *client,
	paymentID, date, reason string, invoiceIDs ...string,
) map[string]any {
	t.Helper()
	return admin.mustPost(
		"/finance/payments/"+paymentID+"/unallocate",
		stage3KUnallocateBody(invoiceIDs, date, reason),
		http.StatusOK,
	)
}

func stage3KCreatePayable(
	t *testing.T, admin *client,
	supplierID, warehouseID, materialID, taxID, unitPrice, date, remark string,
) string {
	t.Helper()
	_, _, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
		t, admin, supplierID, warehouseID, materialID, taxID,
		"1", unitPrice, true, "CNY", "", date, date, remark,
	)
	return invoiceID
}

func stage3KCreateUSDPayable(
	t *testing.T, admin *client,
	supplierID, warehouseID, materialID, taxID, unitPrice, exchangeRate, date, remark string,
) string {
	t.Helper()
	_, _, _, invoiceID, _ := stage3ICreatePurchaseReceipt(
		t, admin, supplierID, warehouseID, materialID, taxID,
		"1", unitPrice, true, "USD", exchangeRate, date, date, remark,
	)
	return invoiceID
}

func stage3KFindVouchers(admin *client, voucherType, keyword, startDate, endDate string) []map[string]any {
	path := "/finance/vouchers?voucher_type=" + url.QueryEscape(voucherType) +
		"&keyword=" + url.QueryEscape(keyword) +
		"&page_size=100"
	if startDate != "" {
		path += "&start_date=" + url.QueryEscape(startDate)
	}
	if endDate != "" {
		path += "&end_date=" + url.QueryEscape(endDate)
	}
	return listItems(admin, path, http.StatusOK)
}

func stage3KSourceVouchers(items []map[string]any, sourceCode string) []map[string]any {
	matches := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if text(item, "source_code") == sourceCode {
			matches = append(matches, item)
		}
	}
	return matches
}

func stage3KVoucherLines(t *testing.T, admin *client, voucher map[string]any) []any {
	t.Helper()
	detail := admin.mustGet("/finance/vouchers/"+text(voucher, "id"), http.StatusOK)
	lines, _ := detail["lines"].([]any)
	if len(lines) == 0 {
		t.Fatalf("voucher %s has no lines", text(voucher, "code"))
	}
	return lines
}

func stage3KFindVoucherByCode(t *testing.T, admin *client, code string) map[string]any {
	t.Helper()
	items := listItems(admin, "/finance/vouchers?keyword="+url.QueryEscape(code)+"&page_size=100", http.StatusOK)
	for _, item := range items {
		if text(item, "code") == code {
			return item
		}
	}
	t.Fatalf("voucher %s not found", code)
	return nil
}

func stage3KAssertOppositeVoucherLines(t *testing.T, originalLines, reversalLines []any) {
	t.Helper()
	if len(originalLines) != len(reversalLines) {
		t.Fatalf("original voucher lines = %d, reversal lines = %d, want equal", len(originalLines), len(reversalLines))
	}
	for _, raw := range originalLines {
		original, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("original voucher line payload = %v", raw)
		}
		wantDebit := number(t, original, "credit")
		wantCredit := number(t, original, "debit")
		matched := false
		for _, rawReversal := range reversalLines {
			reversal, ok := rawReversal.(map[string]any)
			if !ok {
				t.Fatalf("reversal voucher line payload = %v", rawReversal)
			}
			if text(reversal, "partner_id") != text(original, "partner_id") {
				continue
			}
			if nearlyEqual(number(t, reversal, "debit"), wantDebit) &&
				nearlyEqual(number(t, reversal, "credit"), wantCredit) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("no opposite reversal line for account=%s partner=%s debit=%s credit=%s",
				text(original, "account_code"), text(original, "partner_id"),
				text(original, "debit"), text(original, "credit"))
		}
	}
}

func stage3KAssertSameFields(t *testing.T, label string, before, after map[string]any, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if text(before, key) != text(after, key) {
			t.Fatalf("%s %s = %q, before = %q", label, key, text(after, key), text(before, key))
		}
	}
}

func stage3KAssertFullReversal(
	t *testing.T, admin *client,
	paymentID, invoiceID, paymentCode, invoiceCode, amount, reversalDate, reason string,
) {
	t.Helper()
	beforePayment := admin.mustGet("/finance/payments/"+paymentID, http.StatusOK)
	stage3KExpect(t, beforePayment, "status", "allocated")
	stage3KExpect(t, beforePayment, "allocated_amount", amount)
	stage3KExpect(t, beforePayment, "base_allocated_amount", amount)
	stage3KExpect(t, beforePayment, "unallocated_amount", "0.0000")
	if got := len(stage3KObjects(t, beforePayment, "allocations")); got != 1 {
		t.Fatalf("payment allocations before reversal = %d, want 1", got)
	}
	if got := len(stage3KObjects(t, beforePayment, "reversals")); got != 0 {
		t.Fatalf("payment reversals before reversal = %d, want 0", got)
	}
	beforeInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
	stage3KExpect(t, beforeInvoice, "status", "paid")
	stage3KExpect(t, beforeInvoice, "paid_amount", amount)
	stage3KExpect(t, beforeInvoice, "base_paid_amount", amount)
	stage3KExpect(t, beforeInvoice, "outstanding_amount", "0.0000")

	reversed := stage3KUnallocate(t, admin, paymentID, reversalDate, reason, invoiceID)
	stage3KExpect(t, reversed, "id", paymentID)
	stage3KExpect(t, reversed, "code", paymentCode)
	stage3KExpect(t, reversed, "allocated_amount", "0.0000")
	stage3KExpect(t, reversed, "base_allocated_amount", "0.0000")
	stage3KExpect(t, reversed, "unallocated_amount", amount)
	stage3KExpect(t, reversed, "status", "unallocated")
	if got := len(stage3KObjects(t, reversed, "allocations")); got != 0 {
		t.Fatalf("payment allocations after reversal = %d, want 0", got)
	}
	reversals := stage3KObjects(t, reversed, "reversals")
	if len(reversals) != 1 {
		t.Fatalf("payment reversal rows = %d, want 1", len(reversals))
	}
	reversal := reversals[0]
	stage3KExpect(t, reversal, "payment_id", paymentID)
	stage3KExpect(t, reversal, "payment_code", paymentCode)
	stage3KExpect(t, reversal, "invoice_id", invoiceID)
	stage3KExpect(t, reversal, "invoice_code", invoiceCode)
	stage3KExpect(t, reversal, "amount", amount)
	stage3KExpect(t, reversal, "base_amount", amount)
	stage3KExpect(t, reversal, "invoice_base_amount", amount)
	stage3KExpect(t, reversal, "reversal_date", reversalDate)
	stage3KExpect(t, reversal, "reason", reason)
	if text(reversal, "created_by") == "" {
		t.Fatal("reversal created_by is empty")
	}
	if text(reversal, "created_at") == "" {
		t.Fatal("reversal created_at is empty")
	}
	if codes := stage3KStringArray(t, reversal, "fx_reversal_voucher_codes"); len(codes) != 0 {
		t.Fatalf("CNY reversal fx codes = %v, want empty", codes)
	}

	afterInvoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
	stage3KExpect(t, afterInvoice, "paid_amount", "0.0000")
	stage3KExpect(t, afterInvoice, "base_paid_amount", "0.0000")
	stage3KExpect(t, afterInvoice, "outstanding_amount", amount)
	stage3KExpect(t, afterInvoice, "status", "open")
	invoiceDetail := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
	if got := len(stage3KObjects(t, invoiceDetail, "allocations")); got != 0 {
		t.Fatalf("invoice active payment allocations after reversal = %d, want 0", got)
	}
	if got := len(stage3KObjects(t, invoiceDetail, "payment_allocation_reversals")); got != 1 {
		t.Fatalf("invoice payment allocation reversals after reversal = %d, want 1", got)
	}
}

func stage3KExerciseFXReceiptReversal(
	t *testing.T, admin *client,
	customerID, warehouseID, materialID, taxID, invoiceRate, receiptRate, date, remark string,
	originalDebitAccount, originalCreditAccount, reversalDebitAccount, reversalCreditAccount string,
) {
	t.Helper()
	_, _, _, invoiceID := stage3JCreateShippedInvoice(
		t, admin, customerID, warehouseID, materialID, taxID,
		"1", "100.00", "USD", invoiceRate, date, remark,
	)
	payment := stage3KCreatePayment(t, admin, "receipt", customerID, "USD", receiptRate, "100.00", date, remark+"-payment")
	paymentID := text(payment, "id")
	paymentCode := text(payment, "code")
	stage3KAllocate(t, admin, paymentID, stage3JAllocation{invoiceID, "100.00"})

	originalVouchers := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, date, date), paymentCode)
	if len(originalVouchers) != 1 {
		t.Fatalf("%s original FX settlement vouchers = %d, want 1", remark, len(originalVouchers))
	}
	originalLines := stage3KVoucherLines(t, admin, originalVouchers[0])
	assertVoucherBalanced(t, originalLines)
	if got := number(t, findLineByAccount(t, originalLines, originalDebitAccount), "debit"); !nearlyEqual(got, 10) {
		t.Fatalf("%s original debit account %s = %v, want 10", remark, originalDebitAccount, got)
	}
	if got := number(t, findLineByAccount(t, originalLines, originalCreditAccount), "credit"); !nearlyEqual(got, 10) {
		t.Fatalf("%s original credit account %s = %v, want 10", remark, originalCreditAccount, got)
	}

	reversed := stage3KUnallocate(t, admin, paymentID, date, remark+" reversal", invoiceID)
	stage3KExpect(t, reversed, "status", "unallocated")
	reversals := stage3KObjects(t, reversed, "reversals")
	if len(reversals) != 1 {
		t.Fatalf("%s reversal rows = %d, want 1", remark, len(reversals))
	}
	codes := stage3KStringArray(t, reversals[0], "fx_reversal_voucher_codes")
	if len(codes) != 1 {
		t.Fatalf("%s reversal FX codes = %v, want 1", remark, codes)
	}
	reversalVoucher := stage3KFindVoucherByCode(t, admin, codes[0])
	stage3KExpect(t, reversalVoucher, "voucher_type", "fx_settlement_reversal")
	stage3KExpect(t, reversalVoucher, "source_type", "fx_settlement_reversal")
	stage3KExpect(t, reversalVoucher, "source_code", paymentCode)
	if !strings.Contains(text(reversalVoucher, "memo"), "汇兑结算反核销") {
		t.Fatalf("%s reversal memo = %q", remark, text(reversalVoucher, "memo"))
	}
	if text(reversalVoucher, "source_id") != text(originalVouchers[0], "source_id") {
		t.Fatalf("%s reversal source_id = %s, want %s", remark, text(reversalVoucher, "source_id"), text(originalVouchers[0], "source_id"))
	}
	reversalLines := stage3KVoucherLines(t, admin, reversalVoucher)
	assertVoucherBalanced(t, reversalLines)
	stage3KAssertOppositeVoucherLines(t, originalLines, reversalLines)
	if got := number(t, findLineByAccount(t, reversalLines, reversalDebitAccount), "debit"); !nearlyEqual(got, 10) {
		t.Fatalf("%s reversal debit account %s = %v, want 10", remark, reversalDebitAccount, got)
	}
	if got := number(t, findLineByAccount(t, reversalLines, reversalCreditAccount), "credit"); !nearlyEqual(got, 10) {
		t.Fatalf("%s reversal credit account %s = %v, want 10", remark, reversalCreditAccount, got)
	}

	remainingOriginals := stage3KSourceVouchers(stage3KFindVouchers(admin, "fx_settlement", paymentCode, date, date), paymentCode)
	if len(remainingOriginals) != 1 {
		t.Fatalf("%s original FX settlement vouchers after reversal = %d, want 1", remark, len(remainingOriginals))
	}

	invoice := admin.mustGet("/finance/invoices/"+invoiceID, http.StatusOK)
	stage3KExpect(t, invoice, "paid_amount", "0.0000")
	stage3KExpect(t, invoice, "base_paid_amount", "0.0000")
	stage3KExpect(t, invoice, "outstanding_amount", "100.0000")
	stage3KExpect(t, invoice, "status", "open")
}
