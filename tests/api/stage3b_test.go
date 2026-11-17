//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const apiPrefix = "/api/v1"

type client struct {
	t       *testing.T
	baseURL string
	token   string
}

type apiPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newClient(t *testing.T) *client {
	t.Helper()
	baseURL := strings.TrimRight(env("TEST_API_BASE_URL", "http://localhost:18080"), "/")
	return &client{t: t, baseURL: baseURL}
}

func (c *client) login(username, password string) *client {
	c.t.Helper()
	status, payload := c.request(http.MethodPost, "/auth/login", map[string]any{
		"username": username,
		"password": password,
	}, "")
	if status != http.StatusOK {
		c.t.Fatalf("login %s status = %d, payload = %v", username, status, payload)
	}
	token, _ := payload.Data["access_token"].(string)
	if token == "" {
		c.t.Fatalf("login %s returned empty access token: %v", username, payload)
	}
	return &client{t: c.t, baseURL: c.baseURL, token: token}
}

func (c *client) request(method, path string, body any, token string) (int, apiPayload) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+apiPrefix+path, reader)
	if err != nil {
		c.t.Fatalf("build request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var payload apiPayload
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			c.t.Fatalf("%s %s returned non-JSON body: %s", method, path, string(raw))
		}
	}
	return resp.StatusCode, payload
}

func (c *client) get(path string) (int, apiPayload) {
	return c.request(http.MethodGet, path, nil, c.token)
}

func (c *client) post(path string, body any) (int, apiPayload) {
	return c.request(http.MethodPost, path, body, c.token)
}

func (c *client) put(path string, body any) (int, apiPayload) {
	return c.request(http.MethodPut, path, body, c.token)
}

func (c *client) mustGet(path string, wantStatus int) map[string]any {
	c.t.Helper()
	status, payload := c.get(path)
	if status != wantStatus {
		c.t.Fatalf("GET %s status = %d, want %d, payload = %v", path, status, wantStatus, payload)
	}
	return payload.Data
}

func (c *client) mustPost(path string, body any, wantStatus int) map[string]any {
	c.t.Helper()
	status, payload := c.post(path, body)
	if status != wantStatus {
		c.t.Fatalf("POST %s status = %d, want %d, payload = %v", path, status, wantStatus, payload)
	}
	return payload.Data
}

func (c *client) expectError(method, path string, body any, wantStatus int, wantCode string) {
	c.t.Helper()
	status, payload := c.request(method, path, body, c.token)
	if status != wantStatus {
		c.t.Fatalf("%s %s status = %d, want %d, payload = %v", method, path, status, wantStatus, payload)
	}
	if wantCode != "" && payload.Code != wantCode {
		c.t.Fatalf("%s %s code = %q, want %q, payload = %v", method, path, payload.Code, wantCode, payload)
	}
}

func listItems(c *client, path string, wantStatus int) []map[string]any {
	c.t.Helper()
	data := c.mustGet(path, wantStatus)
	rawItems, _ := data["items"].([]any)
	items := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		if mapped, ok := item.(map[string]any); ok {
			items = append(items, mapped)
		}
	}
	return items
}

func text(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return value
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func number(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	raw := text(m, key)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("field %s = %q is not numeric: %v", key, raw, err)
	}
	return value
}

func findItem(items []map[string]any, key, value string) map[string]any {
	for _, item := range items {
		if text(item, key) == value {
			return item
		}
	}
	return nil
}

func findByID(items []map[string]any, id string) map[string]any {
	return findItem(items, "id", id)
}

func TestStage3B(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)

	customer := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-001&page_size=20", http.StatusOK), "code", "CUS-DEMO-001")
	if customer == nil {
		t.Fatal("demo customer CUS-DEMO-001 not found")
	}
	customer2 := findItem(listItems(admin, "/masterdata/partners?keyword=CUS-DEMO-002&page_size=20", http.StatusOK), "code", "CUS-DEMO-002")
	if customer2 == nil {
		t.Fatal("demo customer CUS-DEMO-002 not found")
	}
	supplier := findItem(listItems(admin, "/masterdata/partners?keyword=SUP-DEMO-001&page_size=20", http.StatusOK), "code", "SUP-DEMO-001")
	if supplier == nil {
		t.Fatal("demo supplier SUP-DEMO-001 not found")
	}
	warehouse := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-SH&page_size=20", http.StatusOK), "code", "WH-SH")
	if warehouse == nil {
		t.Fatal("demo warehouse WH-SH not found")
	}
	consignment := findItem(listItems(admin, "/masterdata/warehouses?keyword=WH-CONSIGN&page_size=20", http.StatusOK), "code", "WH-CONSIGN")
	if consignment == nil {
		t.Fatal("demo warehouse WH-CONSIGN not found")
	}
	material := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-004&page_size=20", http.StatusOK), "code", "MAT-DEMO-004")
	if material == nil {
		t.Fatal("demo material MAT-DEMO-004 not found")
	}
	emptyMaterial := findItem(listItems(admin, "/masterdata/materials?keyword=MAT-DEMO-020&page_size=20", http.StatusOK), "code", "MAT-DEMO-020")
	if emptyMaterial == nil {
		t.Fatal("demo material MAT-DEMO-020 not found")
	}

	customerID := text(customer, "id")
	customer2ID := text(customer2, "id")
	supplierID := text(supplier, "id")
	warehouseID := text(warehouse, "id")
	consignmentID := text(consignment, "id")
	materialID := text(material, "id")
	emptyMaterialID := text(emptyMaterial, "id")

	var payableInvoiceID string
	t.Run("stock purchase for sales", func(t *testing.T) {
		order := admin.mustPost("/purchase/orders", map[string]any{
			"supplier_id":  supplierID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-stock-purchase",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "20", "unit_price": "50.00"},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/purchase/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/purchase/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/purchase/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		if len(lines) != 1 {
			t.Fatalf("purchase order lines = %d, want 1", len(lines))
		}
		line := lines[0].(map[string]any)
		receipt := admin.mustPost("/purchase/orders/"+orderID+"/receive", map[string]any{
			"receipt_date": time.Now().Format("2006-01-02"),
			"remark":       "stage3b-stock-receipt",
			"lines": []map[string]any{
				{"order_line_id": text(line, "id"), "quantity": "20"},
			},
		}, http.StatusCreated)
		receiptID := text(receipt, "id")

		invoices := listItems(admin, "/finance/invoices?invoice_type=payable&page_size=200", http.StatusOK)
		payable := findItem(invoices, "source_id", receiptID)
		if payable == nil {
			t.Fatalf("payable invoice not found for receipt %s", receiptID)
		}
		payableInvoiceID = text(payable, "id")
		if got := number(t, payable, "total_amount"); got != 1000 {
			t.Fatalf("payable total = %v, want 1000", got)
		}

		receipts := listItems(admin, "/purchase/receipts?order_id="+orderID, http.StatusOK)
		receiptRow := findByID(receipts, receiptID)
		if receiptRow == nil {
			t.Fatalf("purchase receipt %s not found in receipt list", receiptID)
		}
		if text(receiptRow, "payable_code") == "" {
			t.Fatal("purchase receipt list payable_code is empty")
		}
		if text(receiptRow, "payable_due_date") == "" {
			t.Fatal("purchase receipt list payable_due_date is empty")
		}
	})

	baseline := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
	baselineItems, _ := baseline["items"].([]any)
	if len(baselineItems) != 1 {
		t.Fatalf("baseline inventory rows = %d, want 1", len(baselineItems))
	}
	baselineOnHand := number(t, baselineItems[0].(map[string]any), "on_hand")
	if baselineOnHand < 10 {
		t.Fatalf("baseline on hand = %v, want >= 10", baselineOnHand)
	}
	baselineAvgCost := number(t, baselineItems[0].(map[string]any), "avg_cost")
	if baselineAvgCost <= 0 {
		t.Fatalf("baseline avg_cost = %v, want > 0", baselineAvgCost)
	}

	var salesOrderID string
	var firstShipmentID string
	var firstInvoiceID string
	var secondShipmentID string
	var secondInvoiceID string

	t.Run("sales order lifecycle and partial shipment", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-sales-order-1",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "10", "unit_price": "120.00"},
			},
		}, http.StatusCreated)
		salesOrderID = text(order, "id")
		if got := text(order, "status"); got != "draft" {
			t.Fatalf("created status = %q, want draft", got)
		}
		if got := number(t, order, "total_amount"); got != 1200 {
			t.Fatalf("created total = %v, want 1200", got)
		}

		admin.mustPost("/sales/orders/"+salesOrderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+salesOrderID+"/approve", nil, http.StatusOK)
		admin.expectError(http.MethodPost, "/sales/orders/"+salesOrderID+"/approve", nil, http.StatusConflict, "INVALID_STATUS")

		detail := admin.mustGet("/sales/orders/"+salesOrderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		lineID := text(lines[0].(map[string]any), "id")

		first := admin.mustPost("/sales/orders/"+salesOrderID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"remark":        "stage3b-shipment-1",
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "4"},
			},
		}, http.StatusCreated)
		firstShipmentID = text(first, "id")
		if got := number(t, first, "total_amount"); got != 480 {
			t.Fatalf("first shipment total = %v, want 480", got)
		}
		if text(first, "invoice_code") == "" {
			t.Fatal("first shipment invoice_code is empty")
		}
		expectedCost := baselineAvgCost * 4
		if got := number(t, first, "total_cost"); !nearlyEqual(got, expectedCost) {
			t.Fatalf("first shipment total_cost = %v, want %v", got, expectedCost)
		}
		firstLines, _ := first["lines"].([]any)
		firstLine := firstLines[0].(map[string]any)
		if got := number(t, firstLine, "unit_cost"); !nearlyEqual(got, baselineAvgCost) {
			t.Fatalf("first shipment unit_cost = %v, want %v", got, baselineAvgCost)
		}
		if got := number(t, firstLine, "cost_amount"); !nearlyEqual(got, expectedCost) {
			t.Fatalf("first shipment cost_amount = %v, want %v", got, expectedCost)
		}
		ledger := listItems(admin,
			"/inventory/transactions?warehouse_id="+warehouseID+"&material_id="+materialID+"&page_size=200",
			http.StatusOK,
		)
		var outbound map[string]any
		for _, item := range ledger {
			if text(item, "biz_id") == firstShipmentID {
				outbound = item
				break
			}
		}
		if outbound == nil {
			t.Fatalf("outbound inventory transaction not found for shipment %s", firstShipmentID)
		}
		if got := text(outbound, "direction"); got != "out" {
			t.Fatalf("outbound direction = %q, want out", got)
		}
		if got := number(t, outbound, "unit_cost"); !nearlyEqual(got, baselineAvgCost) {
			t.Fatalf("outbound unit_cost = %v, want %v", got, baselineAvgCost)
		}
		if got := number(t, outbound, "amount"); !nearlyEqual(got, expectedCost) {
			t.Fatalf("outbound amount = %v, want %v", got, expectedCost)
		}

		afterPartial := admin.mustGet("/sales/orders/"+salesOrderID, http.StatusOK)
		if got := text(afterPartial, "status"); got != "partially_shipped" {
			t.Fatalf("status after partial = %q, want partially_shipped", got)
		}
		afterLines, _ := afterPartial["lines"].([]any)
		afterLine := afterLines[0].(map[string]any)
		if got := number(t, afterLine, "shipped_qty"); got != 4 {
			t.Fatalf("shipped_qty = %v, want 4", got)
		}
		if got := number(t, afterLine, "remaining_qty"); got != 6 {
			t.Fatalf("remaining_qty = %v, want 6", got)
		}

		balance := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
		balanceItems, _ := balance["items"].([]any)
		if got := number(t, balanceItems[0].(map[string]any), "on_hand"); got != baselineOnHand-4 {
			t.Fatalf("on_hand after partial = %v, want %v", got, baselineOnHand-4)
		}

		invoices := listItems(admin, "/finance/invoices?invoice_type=receivable&partner_id="+customerID+"&page_size=200", http.StatusOK)
		invoice := findItem(invoices, "source_id", firstShipmentID)
		if invoice == nil {
			t.Fatalf("receivable invoice not found for shipment %s", firstShipmentID)
		}
		firstInvoiceID = text(invoice, "id")
		if got := number(t, invoice, "total_amount"); got != 480 {
			t.Fatalf("first receivable total = %v, want 480", got)
		}
		if got := text(invoice, "status"); got != "open" {
			t.Fatalf("first receivable status = %q, want open", got)
		}

		shipments := listItems(admin, "/sales/shipments?order_id="+salesOrderID, http.StatusOK)
		shipmentRow := findByID(shipments, firstShipmentID)
		if shipmentRow == nil {
			t.Fatalf("shipment %s not found in shipment list", firstShipmentID)
		}
		if text(shipmentRow, "invoice_code") == "" {
			t.Fatal("shipment list invoice_code is empty")
		}
		if text(shipmentRow, "invoice_due_date") == "" {
			t.Fatal("shipment list invoice_due_date is empty")
		}

		second := admin.mustPost("/sales/orders/"+salesOrderID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"remark":        "stage3b-shipment-2",
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "6"},
			},
		}, http.StatusCreated)
		secondShipmentID = text(second, "id")
		if got := number(t, second, "total_amount"); got != 720 {
			t.Fatalf("second shipment total = %v, want 720", got)
		}

		final := admin.mustGet("/sales/orders/"+salesOrderID, http.StatusOK)
		if got := text(final, "status"); got != "shipped" {
			t.Fatalf("final status = %q, want shipped", got)
		}

		balance = admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
		balanceItems, _ = balance["items"].([]any)
		if got := number(t, balanceItems[0].(map[string]any), "on_hand"); got != baselineOnHand-10 {
			t.Fatalf("on_hand after full = %v, want %v", got, baselineOnHand-10)
		}

		invoices = listItems(admin, "/finance/invoices?invoice_type=receivable&partner_id="+customerID+"&page_size=200", http.StatusOK)
		invoice2 := findItem(invoices, "source_id", secondShipmentID)
		if invoice2 == nil {
			t.Fatalf("second receivable invoice not found for shipment %s", secondShipmentID)
		}
		secondInvoiceID = text(invoice2, "id")
		if got := number(t, invoice2, "total_amount"); got != 720 {
			t.Fatalf("second receivable total = %v, want 720", got)
		}
	})

	t.Run("over shipment duplicate lines and insufficient stock", func(t *testing.T) {
		order := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customer2ID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-over-ship",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "10", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		orderID := text(order, "id")
		admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		lines, _ := detail["lines"].([]any)
		lineID := text(lines[0].(map[string]any), "id")

		beforeBalance := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
		beforeItems, _ := beforeBalance["items"].([]any)
		beforeOnHand := number(t, beforeItems[0].(map[string]any), "on_hand")

		admin.expectError(http.MethodPost, "/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "11"},
			},
		}, http.StatusConflict, "OVER_SHIPMENT")

		admin.expectError(http.MethodPost, "/sales/orders/"+orderID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": lineID, "quantity": "10"},
				{"order_line_id": lineID, "quantity": "10"},
			},
		}, http.StatusConflict, "OVER_SHIPMENT")

		after := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
		afterLines, _ := after["lines"].([]any)
		if got := number(t, afterLines[0].(map[string]any), "shipped_qty"); got != 0 {
			t.Fatalf("shipped_qty after rejected shipments = %v, want 0", got)
		}
		afterBalance := admin.mustGet("/inventory/balances?warehouse_id="+warehouseID+"&material_id="+materialID, http.StatusOK)
		afterItems, _ := afterBalance["items"].([]any)
		if got := number(t, afterItems[0].(map[string]any), "on_hand"); got != beforeOnHand {
			t.Fatalf("on_hand after rejected shipments = %v, want %v", got, beforeOnHand)
		}
		shipments := admin.mustGet("/sales/shipments?order_id="+orderID+"&page_size=100", http.StatusOK)
		if got, _ := shipments["total"].(float64); got != 0 {
			t.Fatalf("shipment total after rejected shipments = %v, want 0", got)
		}

		emptyOrder := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customer2ID,
			"warehouse_id": consignmentID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-insufficient-stock",
			"lines": []map[string]any{
				{"material_id": emptyMaterialID, "quantity": "1", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		emptyOrderID := text(emptyOrder, "id")
		admin.mustPost("/sales/orders/"+emptyOrderID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+emptyOrderID+"/approve", nil, http.StatusOK)
		emptyDetail := admin.mustGet("/sales/orders/"+emptyOrderID, http.StatusOK)
		emptyLines, _ := emptyDetail["lines"].([]any)
		admin.expectError(http.MethodPost, "/sales/orders/"+emptyOrderID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(emptyLines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusConflict, "INSUFFICIENT_STOCK")
	})

	t.Run("invalid status and cancel rules", func(t *testing.T) {
		draft := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-draft-ship",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		draftID := text(draft, "id")
		draftDetail := admin.mustGet("/sales/orders/"+draftID, http.StatusOK)
		draftLines, _ := draftDetail["lines"].([]any)
		admin.expectError(http.MethodPost, "/sales/orders/"+draftID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(draftLines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusConflict, "INVALID_STATUS")

		cancelOrder := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-cancel-approved",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		cancelID := text(cancelOrder, "id")
		admin.mustPost("/sales/orders/"+cancelID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+cancelID+"/approve", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+cancelID+"/cancel", nil, http.StatusOK)
		cancelledDetail := admin.mustGet("/sales/orders/"+cancelID, http.StatusOK)
		cancelledLines, _ := cancelledDetail["lines"].([]any)
		admin.expectError(http.MethodPost, "/sales/orders/"+cancelID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(cancelledLines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusConflict, "INVALID_STATUS")

		partialOrder := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-cancel-partial",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "2", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		partialID := text(partialOrder, "id")
		admin.mustPost("/sales/orders/"+partialID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+partialID+"/approve", nil, http.StatusOK)
		partialDetail := admin.mustGet("/sales/orders/"+partialID, http.StatusOK)
		partialLines, _ := partialDetail["lines"].([]any)
		admin.mustPost("/sales/orders/"+partialID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(partialLines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/sales/orders/"+partialID+"/cancel", nil, http.StatusConflict, "INVALID_STATUS")
	})

	t.Run("receivable payment allocation", func(t *testing.T) {
		if firstInvoiceID == "" || secondInvoiceID == "" {
			t.Fatal("receivable invoices were not created by the shipment test")
		}
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "500.00",
			"method":       "bank",
			"reference":    "stage3b-ar-payment",
			"remark":       "stage3b-ar-payment",
		}, http.StatusCreated)
		paymentID := text(payment, "id")
		if got := text(payment, "status"); got != "unallocated" {
			t.Fatalf("payment status = %q, want unallocated", got)
		}

		allocated := admin.mustPost("/finance/payments/"+paymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": firstInvoiceID, "amount": "480.00"},
				{"invoice_id": secondInvoiceID, "amount": "20.00"},
			},
		}, http.StatusOK)
		if got := number(t, allocated, "allocated_amount"); got != 500 {
			t.Fatalf("payment allocated = %v, want 500", got)
		}
		if got := text(allocated, "status"); got != "allocated" {
			t.Fatalf("payment status = %q, want allocated", got)
		}

		firstInvoice := admin.mustGet("/finance/invoices/"+firstInvoiceID, http.StatusOK)
		if got := text(firstInvoice, "status"); got != "paid" {
			t.Fatalf("first invoice status = %q, want paid", got)
		}
		if got := number(t, firstInvoice, "outstanding_amount"); got != 0 {
			t.Fatalf("first invoice outstanding = %v, want 0", got)
		}

		secondInvoice := admin.mustGet("/finance/invoices/"+secondInvoiceID, http.StatusOK)
		if got := text(secondInvoice, "status"); got != "partially_paid" {
			t.Fatalf("second invoice status = %q, want partially_paid", got)
		}
		if got := number(t, secondInvoice, "paid_amount"); got != 20 {
			t.Fatalf("second invoice paid = %v, want 20", got)
		}
		if got := number(t, secondInvoice, "outstanding_amount"); got != 700 {
			t.Fatalf("second invoice outstanding = %v, want 700", got)
		}

		admin.expectError(http.MethodPost, "/finance/payments/"+paymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": secondInvoiceID, "amount": "1.00"},
			},
		}, http.StatusConflict, "PAYMENT_ALLOCATED")
	})

	t.Run("partial allocation top-up and finance validation", func(t *testing.T) {
		createShippedInvoice := func(remark, quantity, unitPrice string) (string, string) {
			order := admin.mustPost("/sales/orders", map[string]any{
				"customer_id":  customerID,
				"warehouse_id": warehouseID,
				"order_date":   time.Now().Format("2006-01-02"),
				"currency":     "CNY",
				"remark":       remark,
				"lines": []map[string]any{
					{"material_id": materialID, "quantity": quantity, "unit_price": unitPrice},
				},
			}, http.StatusCreated)
			orderID := text(order, "id")
			admin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
			admin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
			detail := admin.mustGet("/sales/orders/"+orderID, http.StatusOK)
			lines, _ := detail["lines"].([]any)
			shipment := admin.mustPost("/sales/orders/"+orderID+"/ship", map[string]any{
				"shipment_date": time.Now().Format("2006-01-02"),
				"lines": []map[string]any{
					{"order_line_id": text(lines[0].(map[string]any), "id"), "quantity": quantity},
				},
			}, http.StatusCreated)
			return text(shipment, "id"), text(shipment, "invoice_id")
		}

		_, topUpInvoiceID := createShippedInvoice("stage3b-topup-allocation", "1", "10.00")
		topUpPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "10.00",
		}, http.StatusCreated)
		topUpPaymentID := text(topUpPayment, "id")
		admin.mustPost("/finance/payments/"+topUpPaymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": topUpInvoiceID, "amount": "6.00"},
			},
		}, http.StatusOK)
		afterFirst := admin.mustGet("/finance/invoices/"+topUpInvoiceID, http.StatusOK)
		if got := text(afterFirst, "status"); got != "partially_paid" {
			t.Fatalf("top-up invoice status after first allocation = %q, want partially_paid", got)
		}
		admin.mustPost("/finance/payments/"+topUpPaymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": topUpInvoiceID, "amount": "4.00"},
			},
		}, http.StatusOK)
		afterSecond := admin.mustGet("/finance/invoices/"+topUpInvoiceID, http.StatusOK)
		if got := text(afterSecond, "status"); got != "paid" {
			t.Fatalf("top-up invoice status after second allocation = %q, want paid", got)
		}
		if got := number(t, afterSecond, "paid_amount"); got != 10 {
			t.Fatalf("top-up invoice paid_amount = %v, want 10", got)
		}

		_, overInvoiceID := createShippedInvoice("stage3b-over-allocate", "1", "10.00")
		overPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "20.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(overPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": overInvoiceID, "amount": "20.00"},
			},
		}, http.StatusBadRequest, "OVER_ALLOCATE")

		usdPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type":  "receipt",
			"partner_id":    customerID,
			"payment_date":  time.Now().Format("2006-01-02"),
			"currency":      "USD",
			"exchange_rate": "7.2",
			"amount":        "10.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(usdPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": overInvoiceID, "amount": "5.00"},
			},
		}, http.StatusBadRequest, "CURRENCY_MISMATCH")

		partnerPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customer2ID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "10.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(partnerPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": overInvoiceID, "amount": "5.00"},
			},
		}, http.StatusBadRequest, "PARTNER_MISMATCH")

		duplicatePayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "10.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(duplicatePayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": overInvoiceID, "amount": "1.00"},
				{"invoice_id": overInvoiceID, "amount": "1.00"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")

		admin.expectError(http.MethodPost, "/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "0.00004",
		}, http.StatusBadRequest, "BAD_REQUEST")
	})

	t.Run("payable payment allocation and type mismatch", func(t *testing.T) {
		if payableInvoiceID == "" {
			t.Fatal("payable invoice was not created by the stock purchase test")
		}
		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "payment",
			"partner_id":   supplierID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "1000.00",
			"method":       "bank",
			"reference":    "stage3b-ap-payment",
			"remark":       "stage3b-ap-payment",
		}, http.StatusCreated)
		paymentID := text(payment, "id")

		admin.mustPost("/finance/payments/"+paymentID+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": payableInvoiceID, "amount": "1000.00"},
			},
		}, http.StatusOK)

		payable := admin.mustGet("/finance/invoices/"+payableInvoiceID, http.StatusOK)
		if got := text(payable, "status"); got != "paid" {
			t.Fatalf("payable status = %q, want paid", got)
		}

		receiptPayment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "100.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(receiptPayment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": payableInvoiceID, "amount": "100.00"},
			},
		}, http.StatusBadRequest, "PAYMENT_TYPE_MISMATCH")
	})

	t.Run("data scope and permissions", func(t *testing.T) {
		demoPassword := env("TEST_DEMO_PASSWORD", "Demo@123456")
		dept := newClient(t).login("demo_dept", demoPassword)
		deptOrders := dept.mustGet("/sales/orders?page_size=100", http.StatusOK)
		if got, _ := deptOrders["total"].(float64); got != 0 {
			t.Fatalf("demo_dept sales order total = %v, want 0", got)
		}
		deptInvoices := dept.mustGet("/finance/invoices?page_size=100", http.StatusOK)
		if got, _ := deptInvoices["total"].(float64); got != 0 {
			t.Fatalf("demo_dept invoice total = %v, want 0", got)
		}
		deptShipments := dept.mustGet("/sales/shipments?page_size=100", http.StatusOK)
		if got, _ := deptShipments["total"].(float64); got != 0 {
			t.Fatalf("demo_dept shipment total = %v, want 0", got)
		}
		deptPayments := dept.mustGet("/finance/payments?page_size=100", http.StatusOK)
		if got, _ := deptPayments["total"].(float64); got != 0 {
			t.Fatalf("demo_dept payment total = %v, want 0", got)
		}
		dept.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "1.00"},
			},
		}, http.StatusForbidden, "FORBIDDEN")
		dept.expectError(http.MethodPost, "/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customerID,
			"amount":       "1.00",
		}, http.StatusForbidden, "FORBIDDEN")

		unknown := &client{t: t, baseURL: admin.baseURL}
		status, payload := unknown.get("/sales/orders")
		if status != http.StatusUnauthorized || payload.Code != "UNAUTHORIZED" {
			t.Fatalf("anonymous sales orders status = %d code = %q, want 401 UNAUTHORIZED", status, payload.Code)
		}
	})

	t.Run("multi-line same material and zero total invoice", func(t *testing.T) {
		multi := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customer2ID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-multi-line-same-material",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		multiID := text(multi, "id")
		admin.mustPost("/sales/orders/"+multiID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+multiID+"/approve", nil, http.StatusOK)
		multiDetail := admin.mustGet("/sales/orders/"+multiID, http.StatusOK)
		multiLines, _ := multiDetail["lines"].([]any)
		if len(multiLines) != 2 {
			t.Fatalf("multi-line order lines = %d, want 2", len(multiLines))
		}
		shipment := admin.mustPost("/sales/orders/"+multiID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(multiLines[0].(map[string]any), "id"), "quantity": "1"},
				{"order_line_id": text(multiLines[1].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		if got := number(t, shipment, "total_amount"); got != 20 {
			t.Fatalf("multi-line shipment total = %v, want 20", got)
		}

		zero := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customer2ID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-zero-total",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "0"},
			},
		}, http.StatusCreated)
		zeroID := text(zero, "id")
		admin.mustPost("/sales/orders/"+zeroID+"/submit", nil, http.StatusOK)
		admin.mustPost("/sales/orders/"+zeroID+"/approve", nil, http.StatusOK)
		zeroDetail := admin.mustGet("/sales/orders/"+zeroID, http.StatusOK)
		zeroLines, _ := zeroDetail["lines"].([]any)
		zeroShipment := admin.mustPost("/sales/orders/"+zeroID+"/ship", map[string]any{
			"shipment_date": time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"order_line_id": text(zeroLines[0].(map[string]any), "id"), "quantity": "1"},
			},
		}, http.StatusCreated)
		zeroShipmentID := text(zeroShipment, "id")
		invoices := listItems(admin, "/finance/invoices?invoice_type=receivable&partner_id="+customer2ID+"&page_size=200", http.StatusOK)
		zeroInvoice := findItem(invoices, "source_id", zeroShipmentID)
		if zeroInvoice == nil {
			t.Fatalf("zero-total receivable invoice not found for shipment %s", zeroShipmentID)
		}
		if got := text(zeroInvoice, "status"); got != "paid" {
			t.Fatalf("zero-total invoice status = %q, want paid", got)
		}
		if got := number(t, zeroInvoice, "total_amount"); got != 0 {
			t.Fatalf("zero-total invoice total = %v, want 0", got)
		}

		payment := admin.mustPost("/finance/payments", map[string]any{
			"payment_type": "receipt",
			"partner_id":   customer2ID,
			"payment_date": time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"amount":       "1.00",
		}, http.StatusCreated)
		admin.expectError(http.MethodPost, "/finance/payments/"+text(payment, "id")+"/allocate", map[string]any{
			"allocations": []map[string]any{
				{"invoice_id": text(zeroInvoice, "id"), "amount": "1.00"},
			},
		}, http.StatusConflict, "INVOICE_PAID")
	})

	t.Run("draft edit and invalid values", func(t *testing.T) {
		draft := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-draft-edit",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "10.00"},
			},
		}, http.StatusCreated)
		draftID := text(draft, "id")
		updatedStatus, updatedPayload := admin.put("/sales/orders/"+draftID, map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-draft-edit-updated",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "2", "unit_price": "30.00"},
			},
		})
		if updatedStatus != http.StatusOK {
			t.Fatalf("PUT sales order status = %d, want 200, payload = %v", updatedStatus, updatedPayload)
		}
		if got := number(t, updatedPayload.Data, "total_amount"); got != 60 {
			t.Fatalf("updated total = %v, want 60", got)
		}

		admin.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "-1", "unit_price": "10.00"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
		admin.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "1", "unit_price": "-10.00"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
	})

	t.Run("pagination filter and precision", func(t *testing.T) {
		data := admin.mustGet("/sales/orders?page_size=1", http.StatusOK)
		total, _ := data["total"].(float64)
		if total < 1 {
			t.Fatalf("sales order total = %v, want >= 1", total)
		}
		items, _ := data["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("page_size=1 returned %d items, want 1", len(items))
		}

		shipped := listItems(admin, "/sales/orders?status=shipped&page_size=200", http.StatusOK)
		if len(shipped) == 0 {
			t.Fatal("no shipped sales orders returned")
		}
		for _, item := range shipped {
			if got := text(item, "status"); got != "shipped" {
				t.Fatalf("status filter returned %q, want shipped", got)
			}
		}

		precision := admin.mustPost("/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"currency":     "CNY",
			"remark":       "stage3b-precision",
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "0.1", "unit_price": "3"},
			},
		}, http.StatusCreated)
		if got := text(precision, "total_amount"); got != "0.3000" {
			t.Fatalf("precision total = %q, want 0.3000", got)
		}

		admin.expectError(http.MethodPost, "/sales/orders", map[string]any{
			"customer_id":  customerID,
			"warehouse_id": warehouseID,
			"order_date":   time.Now().Format("2006-01-02"),
			"lines": []map[string]any{
				{"material_id": materialID, "quantity": "0", "unit_price": "1"},
			},
		}, http.StatusBadRequest, "BAD_REQUEST")
	})
}

func TestStage3BAnonymous(t *testing.T) {
	c := newClient(t)
	status, payload := c.get("/finance/invoices")
	if status != http.StatusUnauthorized {
		t.Fatalf("anonymous finance invoices status = %d, want 401, payload = %v", status, payload)
	}
}
