//go:build integration

package api_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"
)

type stage4BHTTPResult struct {
	status int
	code   string
	err    error
}

var stage4BHTTPClient = &http.Client{Timeout: 30 * time.Second}

func TestStage4B(t *testing.T) {
	admin := newClient(t).login(
		env("TEST_ADMIN_USERNAME", "admin"),
		env("TEST_ADMIN_PASSWORD", "Admin@123456"),
	)
	demoAdmin := newClient(t).login("demo_admin", env("TEST_DEMO_PASSWORD", "Demo@123456"))
	demoDept := newClient(t).login("demo_dept", env("TEST_DEMO_PASSWORD", "Demo@123456"))

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

	me := admin.mustGet("/auth/me", http.StatusOK)
	adminUser, _ := me["user"].(map[string]any)
	adminID := text(adminUser, "id")
	tenantID := text(adminUser, "tenant_id")
	if adminID == "" || tenantID == "" {
		t.Fatalf("admin identity missing from /auth/me: %v", me)
	}
	demoMe := demoAdmin.mustGet("/auth/me", http.StatusOK)
	demoUser, _ := demoMe["user"].(map[string]any)
	demoAdminID := text(demoUser, "id")
	if demoAdminID == "" {
		t.Fatalf("demo_admin identity missing from /auth/me: %v", demoMe)
	}

	var (
		sales100FlowID  string
		sales100OrderID string
		sales200FlowID  string
		sales200OrderID string
		sales300FlowID  string
		sales300OrderID string
		sales400FlowID  string
		sales400OrderID string
		sales500FlowID  string
		sales500OrderID string
		sales600FlowID  string
		sales600OrderID string
		sales700FlowID  string
		sales700OrderID string
		purchaseFlowID  string
		purchaseOrderID string
	)

	// 1. No approval flow exists yet. Submitting and approving must keep the
	// legacy direct-approval behaviour for both document types.
	t.Run("legacy no-flow sales and purchase orders", func(t *testing.T) {
		sales := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "10.00")
		salesID := text(sales, "id")
		if got := text(sales, "status"); got != "draft" {
			t.Fatalf("created sales order status = %q, want draft", got)
		}
		submitted := demoAdmin.mustPost("/sales/orders/"+salesID+"/submit", nil, http.StatusOK)
		if stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("legacy sales submit approval_pending = true, want false")
		}
		if got := len(stage4BRequests(t, demoAdmin, "sales_order", salesID)); got != 0 {
			t.Fatalf("legacy sales approval requests = %d, want 0", got)
		}
		approved := demoAdmin.mustPost("/sales/orders/"+salesID+"/approve", nil, http.StatusOK)
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("legacy sales approve approval_pending = true, want false")
		}
		if got := text(demoAdmin.mustGet("/sales/orders/"+salesID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("legacy sales order status = %q, want approved", got)
		}

		purchase := stage4BCreatePurchaseOrder(t, demoAdmin, supplierID, warehouseID, materialID, vat0ID, "10.00")
		purchaseID := text(purchase, "id")
		purchaseSubmitted := demoAdmin.mustPost("/purchase/orders/"+purchaseID+"/submit", nil, http.StatusOK)
		if stage4BBool(t, purchaseSubmitted, "approval_pending") {
			t.Fatalf("legacy purchase submit approval_pending = true, want false")
		}
		if got := len(stage4BRequests(t, demoAdmin, "purchase_order", purchaseID)); got != 0 {
			t.Fatalf("legacy purchase approval requests = %d, want 0", got)
		}
		demoAdmin.mustPost("/purchase/orders/"+purchaseID+"/approve", nil, http.StatusOK)
		if got := text(demoAdmin.mustGet("/purchase/orders/"+purchaseID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("legacy purchase order status = %q, want approved", got)
		}
	})

	// 2. First matching flow: amount >= 100, one user step (admin).
	t.Run("create sales approval flow and submit pending", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-100",
			"sales_order",
			"100.00",
			100,
			[]map[string]any{stage4BUserStep(1, "Stage 4B admin approver", adminID)},
		))
		sales100FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "100.00")
		sales100OrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+sales100OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 100 submit approval_pending = false, want true")
		}
		if got := text(demoAdmin.mustGet("/sales/orders/"+sales100OrderID, http.StatusOK), "status"); got != "submitted" {
			t.Fatalf("sales 100 order status = %q, want submitted", got)
		}

		pending := stage4BPending(t, admin)
		if findItem(pending, "document_id", sales100OrderID) == nil {
			t.Fatalf("pending list for admin does not contain sales order %s", sales100OrderID)
		}

		requests := stage4BRequests(t, admin, "sales_order", sales100OrderID)
		if len(requests) != 1 {
			t.Fatalf("sales 100 approval requests = %d, want 1", len(requests))
		}
		request := requests[0]
		if got := text(request, "status"); got != "pending" {
			t.Fatalf("sales 100 request status = %q, want pending", got)
		}
		if got := text(request, "flow_id"); got != sales100FlowID {
			t.Fatalf("sales 100 request flow_id = %q, want %q", got, sales100FlowID)
		}
		if got := text(request, "document_code"); got == "" {
			t.Fatal("sales 100 request document_code is empty")
		}
		if got := text(request, "base_amount"); got != "100.0000" {
			t.Fatalf("sales 100 request base_amount = %q, want 100.0000", got)
		}
	})

	// 3. The requester cannot approve their own request; admin can.
	t.Run("requester cannot approve and admin approves", func(t *testing.T) {
		path := "/sales/orders/" + sales100OrderID + "/approve"
		// The service checks self-approval before approver membership, so the
		// requester gets SELF_APPROVAL (design section 10).
		demoAdmin.expectError(http.MethodPost, path, map[string]any{"comment": "self approval"}, http.StatusForbidden, "SELF_APPROVAL")

		approved := stage4BApprove(t, admin, path, "stage4b approved")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("sales 100 final approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+sales100OrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("sales 100 order status after approve = %q, want approved", got)
		}

		request := stage4BRequestForDocument(t, admin, "sales_order", sales100OrderID)
		if request == nil {
			t.Fatalf("sales 100 approval request not found after approval")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 100 request status after approve = %q, want approved", got)
		}
		if got := text(request, "completed_at"); got == "" {
			t.Fatal("sales 100 request completed_at is empty")
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("sales 100 approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "action"); got != "approve" {
			t.Fatalf("sales 100 action = %q, want approve", got)
		}
		if got := text(action, "comment"); got != "stage4b approved" {
			t.Fatalf("sales 100 action comment = %q, want stage4b approved", got)
		}
		if got := text(action, "actor_id"); got != adminID {
			t.Fatalf("sales 100 action actor_id = %q, want %q", got, adminID)
		}
	})

	// 4. Two sequential user steps, both assigned to admin.
	t.Run("two step flow", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-200",
			"sales_order",
			"200.00",
			200,
			[]map[string]any{
				stage4BUserStep(1, "Stage 4B step one", adminID),
				stage4BUserStep(2, "Stage 4B step two", adminID),
			},
		))
		sales200FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "200.00")
		sales200OrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+sales200OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 200 submit approval_pending = false, want true")
		}
		path := "/sales/orders/" + sales200OrderID + "/approve"

		first := stage4BApprove(t, admin, path, "stage4b step one")
		if !stage4BBool(t, first, "approval_pending") {
			t.Fatalf("sales 200 first approve approval_pending = false, want true")
		}
		if got := text(admin.mustGet("/sales/orders/"+sales200OrderID, http.StatusOK), "status"); got != "submitted" {
			t.Fatalf("sales 200 order status after step one = %q, want submitted", got)
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", sales200OrderID)
		if request == nil {
			t.Fatalf("sales 200 approval request not found after step one")
		}
		if got := text(request, "status"); got != "pending" {
			t.Fatalf("sales 200 request status after step one = %q, want pending", got)
		}
		if got := text(request, "flow_id"); got != sales200FlowID {
			t.Fatalf("sales 200 flow_id = %q, want %q", got, sales200FlowID)
		}
		if got := stage4BInt(t, request, "current_step"); got != 2 {
			t.Fatalf("sales 200 current_step after step one = %d, want 2", got)
		}
		if got := len(stage4BArray(t, request, "actions")); got != 1 {
			t.Fatalf("sales 200 actions after step one = %d, want 1", got)
		}

		second := stage4BApprove(t, admin, path, "stage4b step two")
		if stage4BBool(t, second, "approval_pending") {
			t.Fatalf("sales 200 second approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+sales200OrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("sales 200 order status after step two = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", sales200OrderID)
		if request == nil {
			t.Fatalf("sales 200 approval request not found after step two")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 200 request status after step two = %q, want approved", got)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 2 {
			t.Fatalf("sales 200 approval actions = %d, want 2", len(actions))
		}
		for index, raw := range actions {
			action, _ := raw.(map[string]any)
			if got := text(action, "action"); got != "approve" {
				t.Fatalf("sales 200 action %d = %q, want approve", index, got)
			}
			if got := stage4BInt(t, action, "step_no"); got != index+1 {
				t.Fatalf("sales 200 action %d step_no = %d, want %d", index, got, index+1)
			}
		}
	})

	// 5. Reject returns the order to draft and records the rejection reason.
	t.Run("reject returns order to draft", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-300",
			"sales_order",
			"300.00",
			300,
			[]map[string]any{stage4BUserStep(1, "Stage 4B reject approver", adminID)},
		))
		sales300FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "300.00")
		sales300OrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+sales300OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 300 submit approval_pending = false, want true")
		}
		path := "/sales/orders/" + sales300OrderID + "/reject"
		admin.expectError(http.MethodPost, path, map[string]any{"comment": "   "}, http.StatusBadRequest, "REASON_REQUIRED")
		stage4BReject(t, admin, path, "stage4b rejected")

		if got := text(admin.mustGet("/sales/orders/"+sales300OrderID, http.StatusOK), "status"); got != "draft" {
			t.Fatalf("sales 300 order status after reject = %q, want draft", got)
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", sales300OrderID)
		if request == nil {
			t.Fatalf("sales 300 approval request not found after reject")
		}
		if got := text(request, "status"); got != "rejected" {
			t.Fatalf("sales 300 request status after reject = %q, want rejected", got)
		}
		if got := text(request, "flow_id"); got != sales300FlowID {
			t.Fatalf("sales 300 flow_id = %q, want %q", got, sales300FlowID)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("sales 300 approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "action"); got != "reject" {
			t.Fatalf("sales 300 action = %q, want reject", got)
		}
		if got := text(action, "comment"); got != "stage4b rejected" {
			t.Fatalf("sales 300 reject comment = %q, want stage4b rejected", got)
		}
		if findItem(stage4BPending(t, admin), "document_id", sales300OrderID) != nil {
			t.Fatalf("rejected sales order %s still appears in pending list", sales300OrderID)
		}
	})

	// 6. Cancelling a submitted order cancels its pending request.
	t.Run("cancel cancels pending request", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-400",
			"sales_order",
			"400.00",
			400,
			[]map[string]any{stage4BUserStep(1, "Stage 4B cancel approver", adminID)},
		))
		sales400FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "400.00")
		sales400OrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+sales400OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 400 submit approval_pending = false, want true")
		}
		demoAdmin.mustPost("/sales/orders/"+sales400OrderID+"/cancel", nil, http.StatusOK)

		if got := text(admin.mustGet("/sales/orders/"+sales400OrderID, http.StatusOK), "status"); got != "cancelled" {
			t.Fatalf("sales 400 order status after cancel = %q, want cancelled", got)
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", sales400OrderID)
		if request == nil {
			t.Fatalf("sales 400 approval request not found after cancel")
		}
		if got := text(request, "status"); got != "cancelled" {
			t.Fatalf("sales 400 request status after cancel = %q, want cancelled", got)
		}
		if got := text(request, "flow_id"); got != sales400FlowID {
			t.Fatalf("sales 400 flow_id = %q, want %q", got, sales400FlowID)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("sales 400 approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "action"); got != "cancel" {
			t.Fatalf("sales 400 action = %q, want cancel", got)
		}
		if got := text(action, "actor_id"); got != demoAdminID {
			t.Fatalf("sales 400 cancel actor_id = %q, want %q", got, demoAdminID)
		}
		if findItem(stage4BPending(t, admin), "document_id", sales400OrderID) != nil {
			t.Fatalf("cancelled sales order %s still appears in pending list", sales400OrderID)
		}
	})

	// 7. Amounts below all thresholds keep legacy direct approval.
	t.Run("below threshold stays legacy direct approval", func(t *testing.T) {
		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "50.00")
		orderID := text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		if stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("below-threshold submit approval_pending = true, want false")
		}
		if got := len(stage4BRequests(t, demoAdmin, "sales_order", orderID)); got != 0 {
			t.Fatalf("below-threshold approval requests = %d, want 0", got)
		}
		approved := demoAdmin.mustPost("/sales/orders/"+orderID+"/approve", nil, http.StatusOK)
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("below-threshold approve approval_pending = true, want false")
		}
		if got := text(demoAdmin.mustGet("/sales/orders/"+orderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("below-threshold order status = %q, want approved", got)
		}
	})

	// 8. A flow whose only step is the requester has no eligible approver.
	t.Run("self-only flow has no approver", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-500",
			"sales_order",
			"500.00",
			500,
			[]map[string]any{stage4BUserStep(1, "Stage 4B self approver", demoAdminID)},
		))
		sales500FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "500.00")
		sales500OrderID = text(order, "id")
		demoAdmin.expectError(http.MethodPost, "/sales/orders/"+sales500OrderID+"/submit", nil, http.StatusConflict, "APPROVAL_NO_APPROVER")
		if got := text(demoAdmin.mustGet("/sales/orders/"+sales500OrderID, http.StatusOK), "status"); got != "draft" {
			t.Fatalf("self-only flow order status = %q, want draft after compensation", got)
		}
		if got := len(stage4BRequests(t, admin, "sales_order", sales500OrderID)); got != 0 {
			t.Fatalf("self-only flow approval requests = %d, want 0", got)
		}
		if got := text(admin.mustGet("/system/approval-flows/"+sales500FlowID, http.StatusOK), "id"); got != sales500FlowID {
			t.Fatalf("self-only flow id = %q, want %q", got, sales500FlowID)
		}
	})

	// 9. Role-based approver. demo_company is a non-super-admin, company-scoped
	// seeded user; adding the role preserves its existing viewer role, so the
	// erp.user_roles membership check is actually exercised.
	t.Run("role approver", func(t *testing.T) {
		roleID := stage4BEnsureApproverRole(t, admin)
		approver, approverID := stage4BEnsureApproverUser(t, admin, roleID)

		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-600",
			"sales_order",
			"600.00",
			600,
			[]map[string]any{stage4BRoleStep(1, "Stage 4B role approver", roleID)},
		))
		sales600FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, admin, customerID, warehouseID, materialID, vat0ID, "600.00")
		sales600OrderID = text(order, "id")
		submitted := admin.mustPost("/sales/orders/"+sales600OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 600 submit approval_pending = false, want true")
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", sales600OrderID)
		if request == nil {
			t.Fatalf("sales 600 approval request not found")
		}
		if got := text(request, "requested_by"); got != adminID {
			t.Fatalf("sales 600 requested_by = %q, want %q", got, adminID)
		}
		if findItem(stage4BPending(t, approver), "document_id", sales600OrderID) == nil {
			t.Fatalf("pending list for role approver does not contain sales order %s", sales600OrderID)
		}

		approved := stage4BApprove(t, approver, "/sales/orders/"+sales600OrderID+"/approve", "stage4b role approved")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("sales 600 approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+sales600OrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("sales 600 order status = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", sales600OrderID)
		if request == nil {
			t.Fatalf("sales 600 approval request not found after approval")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 600 request status = %q, want approved", got)
		}
		if got := text(request, "flow_id"); got != sales600FlowID {
			t.Fatalf("sales 600 flow_id = %q, want %q", got, sales600FlowID)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("sales 600 approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "actor_id"); got != approverID {
			t.Fatalf("sales 600 action actor_id = %q, want %q", got, approverID)
		}
	})

	// 10. A role approver whose data scope is self (users created through the
	// API have no org_memberships row) can still complete an approval. The
	// approval engine authorizes the role step, and the order transitions then
	// bypass the approver's data scope.
	t.Run("self-scoped approver completes approval", func(t *testing.T) {
		approver, approverID, roleID := stage4BEnsureSelfScopeApprover(t, admin)

		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-self-approver-900",
			"sales_order",
			"900.00",
			900,
			[]map[string]any{stage4BRoleStep(1, "Stage 4B self approver", roleID)},
		))
		flowID := text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "900.00")
		orderID := text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 900 submit approval_pending = false, want true")
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", orderID)
		if request == nil {
			t.Fatalf("sales 900 approval request not found")
		}
		if got := text(request, "flow_id"); got != flowID {
			t.Fatalf("sales 900 flow_id = %q, want %q", got, flowID)
		}
		if findItem(stage4BPending(t, approver), "document_id", orderID) == nil {
			t.Fatalf("pending list for self-scoped approver does not contain sales order %s", orderID)
		}

		approved := stage4BApprove(t, approver, "/sales/orders/"+orderID+"/approve", "stage4b self-scoped approved")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("sales 900 approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+orderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("sales 900 order status = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", orderID)
		if request == nil {
			t.Fatalf("sales 900 approval request not found after approval")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 900 request status = %q, want approved", got)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("sales 900 approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "actor_id"); got != approverID {
			t.Fatalf("sales 900 action actor_id = %q, want %q", got, approverID)
		}
	})

	// 11. Two concurrent approvals: exactly one succeeds and one action is
	// recorded.
	t.Run("concurrent approval succeeds once", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-sales-700",
			"sales_order",
			"700.00",
			700,
			[]map[string]any{stage4BUserStep(1, "Stage 4B concurrent approver", adminID)},
		))
		sales700FlowID = text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "700.00")
		sales700OrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+sales700OrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 700 submit approval_pending = false, want true")
		}

		path := "/sales/orders/" + sales700OrderID + "/approve"
		results := make([]stage4BHTTPResult, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for index := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				status, code, _, err := stage4BRawPost(admin, path, map[string]any{"comment": "stage4b concurrent"})
				results[i] = stage4BHTTPResult{status: status, code: code, err: err}
			}(index)
		}
		close(start)
		wg.Wait()

		successes := 0
		conflicts := 0
		for index, result := range results {
			if result.err != nil {
				t.Fatalf("concurrent approval %d failed: %v", index, result.err)
			}
			switch result.status {
			case http.StatusOK:
				successes++
			case http.StatusConflict:
				switch result.code {
				case "ALREADY_ACTED", "INVALID_STATUS", "NO_PENDING_APPROVAL":
					conflicts++
				default:
					t.Fatalf("concurrent approval %d conflict code = %q", index, result.code)
				}
			default:
				t.Fatalf("concurrent approval %d status = %d, code = %q", index, result.status, result.code)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent approval success/conflict = %d/%d, want 1/1", successes, conflicts)
		}
		if got := text(admin.mustGet("/sales/orders/"+sales700OrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("sales 700 order status after concurrent approval = %q, want approved", got)
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", sales700OrderID)
		if request == nil {
			t.Fatalf("sales 700 approval request not found")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 700 request status = %q, want approved", got)
		}
		if got := text(request, "flow_id"); got != sales700FlowID {
			t.Fatalf("sales 700 flow_id = %q, want %q", got, sales700FlowID)
		}
		if got := len(stage4BArray(t, request, "actions")); got != 1 {
			t.Fatalf("sales 700 approval actions = %d, want 1", got)
		}
	})

	// 12. Users without approval permissions cannot configure flows, read
	// pending work, or approve orders.
	t.Run("approval permissions", func(t *testing.T) {
		demoDept.expectError(http.MethodGet, "/system/approval-flows", nil, http.StatusForbidden, "FORBIDDEN")
		demoDept.expectError(http.MethodGet, "/approvals/pending", nil, http.StatusForbidden, "FORBIDDEN")
		demoDept.expectError(http.MethodGet, "/approvals/requests", nil, http.StatusForbidden, "FORBIDDEN")
		demoDept.expectError(http.MethodPost, "/system/approval-flows", stage4BFlowBody(
			"stage4b-denied",
			"sales_order",
			"0",
			1,
			[]map[string]any{stage4BUserStep(1, "Denied", adminID)},
		), http.StatusForbidden, "FORBIDDEN")
		demoDept.expectError(http.MethodPost, "/sales/orders/"+sales100OrderID+"/approve", map[string]any{"comment": "denied"}, http.StatusForbidden, "FORBIDDEN")
	})

	// 13. Flows with any approval request (including history) cannot be edited
	// or deleted, but can still be deactivated and reactivated.
	t.Run("flow history protection", func(t *testing.T) {
		original := admin.mustGet("/system/approval-flows/"+sales100FlowID, http.StatusOK)
		updateBody := stage4BFlowBody(
			"stage4b-sales-100-updated",
			"sales_order",
			"100.00",
			100,
			[]map[string]any{stage4BUserStep(1, "Stage 4B admin approver", adminID)},
		)
		admin.expectError(http.MethodPut, "/system/approval-flows/"+sales100FlowID, updateBody, http.StatusConflict, "FLOW_IN_USE")
		admin.expectError(http.MethodDelete, "/system/approval-flows/"+sales100FlowID, nil, http.StatusConflict, "FLOW_IN_USE")
		current := admin.mustGet("/system/approval-flows/"+sales100FlowID, http.StatusOK)
		if got, want := text(current, "name"), text(original, "name"); got != want {
			t.Fatalf("flow name after rejected update = %q, want %q", got, want)
		}

		deactivated := admin.mustPost("/system/approval-flows/"+sales100FlowID+"/active", map[string]any{"active": false}, http.StatusOK)
		if stage4BBool(t, deactivated, "active") {
			t.Fatalf("flow active after deactivation = true, want false")
		}
		current = admin.mustGet("/system/approval-flows/"+sales100FlowID, http.StatusOK)
		if stage4BBool(t, current, "active") {
			t.Fatalf("flow active after deactivation = true, want false")
		}
		if got, want := text(current, "name"), text(original, "name"); got != want {
			t.Fatalf("flow name after deactivation = %q, want %q", got, want)
		}

		restored := admin.mustPost("/system/approval-flows/"+sales100FlowID+"/active", map[string]any{"active": true}, http.StatusOK)
		if !stage4BBool(t, restored, "active") {
			t.Fatalf("flow active after restore = false, want true")
		}
	})

	// 14. Purchase orders use the same approval workflow integration.
	t.Run("purchase order flow", func(t *testing.T) {
		flow := stage4BCreateFlow(t, admin, stage4BFlowBody(
			"stage4b-purchase-100",
			"purchase_order",
			"100.00",
			100,
			[]map[string]any{stage4BUserStep(1, "Stage 4B purchase approver", adminID)},
		))
		purchaseFlowID = text(flow, "id")

		order := stage4BCreatePurchaseOrder(t, demoAdmin, supplierID, warehouseID, materialID, vat0ID, "100.00")
		purchaseOrderID = text(order, "id")
		submitted := demoAdmin.mustPost("/purchase/orders/"+purchaseOrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("purchase 100 submit approval_pending = false, want true")
		}
		if got := text(demoAdmin.mustGet("/purchase/orders/"+purchaseOrderID, http.StatusOK), "status"); got != "submitted" {
			t.Fatalf("purchase 100 order status = %q, want submitted", got)
		}
		request := stage4BRequestForDocument(t, admin, "purchase_order", purchaseOrderID)
		if request == nil {
			t.Fatalf("purchase 100 approval request not found")
		}
		if got := text(request, "flow_id"); got != purchaseFlowID {
			t.Fatalf("purchase 100 flow_id = %q, want %q", got, purchaseFlowID)
		}
		approved := stage4BApprove(t, admin, "/purchase/orders/"+purchaseOrderID+"/approve", "stage4b purchase approved")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("purchase 100 approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/purchase/orders/"+purchaseOrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("purchase 100 order status after approve = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "purchase_order", purchaseOrderID)
		if request == nil {
			t.Fatalf("purchase 100 approval request not found after approval")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("purchase 100 request status = %q, want approved", got)
		}
	})

	// 15. Pending/history detail contract, including empty arrays.
	t.Run("pending and request detail contract", func(t *testing.T) {
		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "800.00")
		orderID := text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("sales 800 submit approval_pending = false, want true")
		}

		requests := stage4BRequests(t, admin, "sales_order", orderID)
		if len(requests) != 1 {
			t.Fatalf("sales 800 approval requests = %d, want 1", len(requests))
		}
		request := requests[0]
		if got := text(request, "tenant_id"); got != tenantID {
			t.Fatalf("sales 800 request tenant_id = %q, want %q", got, tenantID)
		}
		if got := text(request, "org_id"); got == "" {
			t.Fatal("sales 800 request org_id is empty")
		}
		if got := text(request, "status"); got != "pending" {
			t.Fatalf("sales 800 request status = %q, want pending", got)
		}
		if got := text(request, "amount"); got != "800.0000" {
			t.Fatalf("sales 800 request amount = %q, want 800.0000", got)
		}
		if got := text(request, "base_amount"); got != "800.0000" {
			t.Fatalf("sales 800 request base_amount = %q, want 800.0000", got)
		}
		steps := stage4BArray(t, request, "steps")
		if len(steps) != 1 {
			t.Fatalf("sales 800 request steps = %d, want 1", len(steps))
		}
		step, ok := steps[0].(map[string]any)
		if !ok {
			t.Fatalf("sales 800 request first step payload = %v", steps[0])
		}
		if got := stage4BInt(t, step, "step_no"); got != 1 {
			t.Fatalf("sales 800 request first step_no = %d, want 1", got)
		}
		if got := len(stage4BArray(t, request, "actions")); got != 0 {
			t.Fatalf("sales 800 request actions before approve = %d, want 0", got)
		}

		detail := admin.mustGet("/approvals/requests/"+text(request, "id"), http.StatusOK)
		if got := text(detail, "id"); got != text(request, "id") {
			t.Fatalf("request detail id = %q, want %q", got, text(request, "id"))
		}
		if got := len(stage4BArray(t, detail, "steps")); got != 1 {
			t.Fatalf("request detail steps = %d, want 1", got)
		}
		if got := len(stage4BArray(t, detail, "actions")); got != 0 {
			t.Fatalf("request detail actions before approve = %d, want 0", got)
		}

		pending := stage4BPending(t, admin)
		if findItem(pending, "document_id", orderID) == nil {
			t.Fatalf("pending list does not contain sales order %s", orderID)
		}
		for _, item := range pending {
			if got := text(item, "tenant_id"); got != tenantID {
				t.Fatalf("pending request tenant_id = %q, want %q", got, tenantID)
			}
		}

		listing := admin.mustGet("/approvals/requests?document_type=sales_order&document_id="+url.QueryEscape(orderID)+"&page_size=50", http.StatusOK)
		if got := stage4BInt(t, listing, "total"); got < 1 {
			t.Fatalf("request list total = %d, want >= 1", got)
		}

		approved := stage4BApprove(t, admin, "/sales/orders/"+orderID+"/approve", "stage4b detail approved")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("sales 800 approve approval_pending = true, want false")
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", orderID)
		if request == nil {
			t.Fatalf("sales 800 approval request not found after approval")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("sales 800 request status after approve = %q, want approved", got)
		}
		if got := len(stage4BArray(t, request, "actions")); got != 1 {
			t.Fatalf("sales 800 approval actions = %d, want 1", got)
		}
		if got := text(request, "completed_at"); got == "" {
			t.Fatal("sales 800 completed_at is empty after approval")
		}
	})

	// 16. Seeded dept user has no approval read permission. Tenant isolation is
	// asserted on every request above; org-scoped pending visibility cannot be
	// exercised because the user API does not set data_scope or org memberships.
	t.Run("data scope and tenant boundaries", func(t *testing.T) {
		demoDept.expectError(http.MethodGet, "/approvals/pending", nil, http.StatusForbidden, "FORBIDDEN")
	})

	// 17. Org-specific flows win over tenant-wide flows regardless of priority;
	// deactivated flows no longer match; remaining ties follow priority desc.
	t.Run("flow matching precedence and deactivation", func(t *testing.T) {
		hq := findItem(getArray(admin, "/system/orgs", http.StatusOK), "code", "HQ")
		if hq == nil {
			t.Fatal("HQ organization not found")
		}
		hqID := text(hq, "id")

		tenantFlow := stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-tenant-2000",
			"",
			2000,
			[]map[string]any{stage4BUserStep(1, "Stage 4B tenant approver", adminID)},
		))
		tenantFlowID := text(tenantFlow, "id")
		hqFlow := stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-hq-2001",
			hqID,
			2001,
			[]map[string]any{stage4BUserStep(1, "Stage 4B HQ approver", adminID)},
		))
		hqFlowID := text(hqFlow, "id")

		firstOrder := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "1.00")
		firstOrderID := text(firstOrder, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+firstOrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("always flow first submit approval_pending = false, want true")
		}
		firstRequest := stage4BRequestForDocument(t, admin, "sales_order", firstOrderID)
		if firstRequest == nil {
			t.Fatalf("always flow first approval request not found")
		}
		if got := text(firstRequest, "flow_id"); got != hqFlowID {
			t.Fatalf("org-specific flow matching = %q, want HQ flow %q", got, hqFlowID)
		}

		deactivated := admin.mustPost("/system/approval-flows/"+hqFlowID+"/active", map[string]any{"active": false}, http.StatusOK)
		if stage4BBool(t, deactivated, "active") {
			t.Fatalf("HQ flow active after deactivation = true, want false")
		}

		secondOrder := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "1.00")
		secondOrderID := text(secondOrder, "id")
		submitted = demoAdmin.mustPost("/sales/orders/"+secondOrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("deactivated flow submit approval_pending = false, want true")
		}
		secondRequest := stage4BRequestForDocument(t, admin, "sales_order", secondOrderID)
		if secondRequest == nil {
			t.Fatalf("deactivated flow approval request not found")
		}
		if got := text(secondRequest, "flow_id"); got != tenantFlowID {
			t.Fatalf("deactivated flow matching = %q, want tenant flow %q", got, tenantFlowID)
		}

		stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-tenant-3000",
			"",
			3000,
			[]map[string]any{stage4BUserStep(1, "Stage 4B priority 3000 approver", adminID)},
		))
		priority3001 := stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-tenant-3001",
			"",
			3001,
			[]map[string]any{stage4BUserStep(1, "Stage 4B priority 3001 approver", adminID)},
		))
		priority3001ID := text(priority3001, "id")

		thirdOrder := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "1.00")
		thirdOrderID := text(thirdOrder, "id")
		submitted = demoAdmin.mustPost("/sales/orders/"+thirdOrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("priority flow submit approval_pending = false, want true")
		}
		thirdRequest := stage4BRequestForDocument(t, admin, "sales_order", thirdOrderID)
		if thirdRequest == nil {
			t.Fatalf("priority flow approval request not found")
		}
		if got := text(thirdRequest, "flow_id"); got != priority3001ID {
			t.Fatalf("priority flow matching = %q, want flow %q", got, priority3001ID)
		}

		for _, orderID := range []string{firstOrderID, secondOrderID, thirdOrderID} {
			demoAdmin.mustPost("/sales/orders/"+orderID+"/cancel", nil, http.StatusOK)
			if got := text(admin.mustGet("/sales/orders/"+orderID, http.StatusOK), "status"); got != "cancelled" {
				t.Fatalf("order %s status after cleanup cancel = %q, want cancelled", orderID, got)
			}
		}
	})

	// 18. Non-super-admin approvers cannot act outside their step even with the
	// approve permission; super admins remain the fallback. Multi-step approve
	// responses must expose the next pending step.
	t.Run("super admin fallback and not approver", func(t *testing.T) {
		users := listItems(admin, "/system/users?keyword="+url.QueryEscape("demo_company")+"&page_size=20", http.StatusOK)
		company := findItem(users, "username", "demo_company")
		if company == nil {
			t.Fatal("demo_company user not found")
		}
		companyID := text(company, "id")

		otherApprover, _, _ := stage4BEnsureSelfScopeApprover(t, admin)

		flow := stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-company-4000",
			"",
			4000,
			[]map[string]any{stage4BUserStep(1, "Stage 4B company approver", companyID)},
		))
		flowID := text(flow, "id")

		order := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "1.00")
		orderID := text(order, "id")
		submitted := demoAdmin.mustPost("/sales/orders/"+orderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("company approver submit approval_pending = false, want true")
		}
		request := stage4BRequestForDocument(t, admin, "sales_order", orderID)
		if request == nil {
			t.Fatalf("company approver approval request not found")
		}
		if got := text(request, "flow_id"); got != flowID {
			t.Fatalf("company approver flow_id = %q, want %q", got, flowID)
		}

		path := "/sales/orders/" + orderID + "/approve"
		otherApprover.expectError(http.MethodPost, path, map[string]any{"comment": "not the step approver"}, http.StatusForbidden, "NOT_APPROVER")
		if got := text(admin.mustGet("/sales/orders/"+orderID, http.StatusOK), "status"); got != "submitted" {
			t.Fatalf("order status after NOT_APPROVER = %q, want submitted", got)
		}

		approved := stage4BApprove(t, admin, path, "stage4b super admin fallback")
		if stage4BBool(t, approved, "approval_pending") {
			t.Fatalf("super admin approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+orderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("order status after super admin approve = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", orderID)
		if request == nil {
			t.Fatalf("company approver approval request not found after approve")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("company approver request status = %q, want approved", got)
		}
		actions := stage4BArray(t, request, "actions")
		if len(actions) != 1 {
			t.Fatalf("company approver approval actions = %d, want 1", len(actions))
		}
		action, _ := actions[0].(map[string]any)
		if got := text(action, "actor_id"); got != adminID {
			t.Fatalf("company approver action actor_id = %q, want super admin %q", got, adminID)
		}

		twoStep := stage4BCreateFlow(t, admin, stage4BAlwaysFlowBody(
			"stage4b-always-two-step-5000",
			"",
			5000,
			[]map[string]any{
				stage4BUserStep(1, "Stage 4B two step one", adminID),
				stage4BUserStep(2, "Stage 4B two step two", adminID),
			},
		))
		twoStepID := text(twoStep, "id")

		twoStepOrder := stage4BCreateSalesOrder(t, demoAdmin, customerID, warehouseID, materialID, vat0ID, "1.00")
		twoStepOrderID := text(twoStepOrder, "id")
		submitted = demoAdmin.mustPost("/sales/orders/"+twoStepOrderID+"/submit", nil, http.StatusOK)
		if !stage4BBool(t, submitted, "approval_pending") {
			t.Fatalf("two-step submit approval_pending = false, want true")
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", twoStepOrderID)
		if request == nil {
			t.Fatalf("two-step approval request not found")
		}
		if got := text(request, "flow_id"); got != twoStepID {
			t.Fatalf("two-step flow_id = %q, want %q", got, twoStepID)
		}

		twoStepPath := "/sales/orders/" + twoStepOrderID + "/approve"
		first := stage4BApprove(t, admin, twoStepPath, "stage4b two step first")
		if !stage4BBool(t, first, "approval_pending") {
			t.Fatalf("two-step first approve approval_pending = false, want true")
		}
		if got := text(first, "status"); got != "pending" {
			t.Fatalf("two-step first approve status = %q, want pending", got)
		}
		if got := stage4BInt(t, first, "current_step"); got != 2 {
			t.Fatalf("two-step first approve current_step = %d, want 2", got)
		}

		second := stage4BApprove(t, admin, twoStepPath, "stage4b two step second")
		if stage4BBool(t, second, "approval_pending") {
			t.Fatalf("two-step second approve approval_pending = true, want false")
		}
		if got := text(admin.mustGet("/sales/orders/"+twoStepOrderID, http.StatusOK), "status"); got != "approved" {
			t.Fatalf("two-step order status after final approve = %q, want approved", got)
		}
		request = stage4BRequestForDocument(t, admin, "sales_order", twoStepOrderID)
		if request == nil {
			t.Fatalf("two-step approval request not found after final approve")
		}
		if got := text(request, "status"); got != "approved" {
			t.Fatalf("two-step request status after final approve = %q, want approved", got)
		}
	})
}

func stage4BFlowBody(name, documentType, threshold string, priority int, steps []map[string]any) map[string]any {
	return map[string]any{
		"name":             name,
		"document_type":    documentType,
		"org_id":           "",
		"condition_type":   "amount_gte",
		"threshold_amount": threshold,
		"priority":         priority,
		"active":           true,
		"steps":            steps,
	}
}

func stage4BAlwaysFlowBody(name, orgID string, priority int, steps []map[string]any) map[string]any {
	return map[string]any{
		"name":             name,
		"document_type":    "sales_order",
		"org_id":           orgID,
		"condition_type":   "always",
		"threshold_amount": "0",
		"priority":         priority,
		"active":           true,
		"steps":            steps,
	}
}

func stage4BUserStep(stepNo int, name, userID string) map[string]any {
	return map[string]any{
		"step_no":       stepNo,
		"name":          name,
		"approver_type": "user",
		"approver_id":   userID,
	}
}

func stage4BRoleStep(stepNo int, name, roleID string) map[string]any {
	return map[string]any{
		"step_no":       stepNo,
		"name":          name,
		"approver_type": "role",
		"approver_id":   roleID,
	}
}

func stage4BCreateFlow(t *testing.T, admin *client, body map[string]any) map[string]any {
	t.Helper()
	flow := admin.mustPost("/system/approval-flows", body, http.StatusCreated)
	if text(flow, "id") == "" {
		t.Fatalf("created approval flow has empty id: %v", flow)
	}
	if got := text(flow, "name"); got != text(body, "name") {
		t.Fatalf("created approval flow name = %q, want %q", got, text(body, "name"))
	}
	if got := text(flow, "document_type"); got != text(body, "document_type") {
		t.Fatalf("created approval flow document_type = %q, want %q", got, text(body, "document_type"))
	}
	steps, _ := body["steps"].([]map[string]any)
	if got := len(stage4BArray(t, flow, "steps")); got != len(steps) {
		t.Fatalf("created approval flow steps = %d, want %d", got, len(steps))
	}
	return flow
}

func stage4BCreateSalesOrder(
	t *testing.T,
	c *client,
	customerID, warehouseID, materialID, taxID, amount string,
) map[string]any {
	t.Helper()
	return c.mustPost("/sales/orders", map[string]any{
		"customer_id":  customerID,
		"warehouse_id": warehouseID,
		"order_date":   time.Now().Format("2006-01-02"),
		"currency":     "CNY",
		"remark":       stage4BUnique("stage4b-sales"),
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": "1", "unit_price": amount, "tax_rate_id": taxID},
		},
	}, http.StatusCreated)
}

func stage4BCreatePurchaseOrder(
	t *testing.T,
	c *client,
	supplierID, warehouseID, materialID, taxID, amount string,
) map[string]any {
	t.Helper()
	return c.mustPost("/purchase/orders", map[string]any{
		"supplier_id":  supplierID,
		"warehouse_id": warehouseID,
		"order_date":   time.Now().Format("2006-01-02"),
		"currency":     "CNY",
		"remark":       stage4BUnique("stage4b-purchase"),
		"lines": []map[string]any{
			{"material_id": materialID, "quantity": "1", "unit_price": amount, "tax_rate_id": taxID},
		},
	}, http.StatusCreated)
}

func stage4BApprove(t *testing.T, c *client, path, comment string) map[string]any {
	t.Helper()
	return c.mustPost(path, map[string]any{"comment": comment}, http.StatusOK)
}

func stage4BReject(t *testing.T, c *client, path, comment string) map[string]any {
	t.Helper()
	return c.mustPost(path, map[string]any{"comment": comment}, http.StatusOK)
}

func stage4BRequests(t *testing.T, c *client, documentType, documentID string) []map[string]any {
	t.Helper()
	path := "/approvals/requests?document_type=" + url.QueryEscape(documentType) +
		"&document_id=" + url.QueryEscape(documentID) + "&page_size=50"
	return listItems(c, path, http.StatusOK)
}

func stage4BRequestForDocument(t *testing.T, c *client, documentType, documentID string) map[string]any {
	t.Helper()
	listed := findItem(stage4BRequests(t, c, documentType, documentID), "document_id", documentID)
	if listed == nil {
		return nil
	}
	// Actions are loaded by the request detail endpoint; the list query only
	// populates steps.
	return c.mustGet("/approvals/requests/"+text(listed, "id"), http.StatusOK)
}

func stage4BPending(t *testing.T, c *client) []map[string]any {
	t.Helper()
	return listItems(c, "/approvals/pending", http.StatusOK)
}

func stage4BEnsureApproverRole(t *testing.T, admin *client) string {
	t.Helper()
	const roleCode = "stage4b_approver"
	var roleID string
	if role := findItem(getArray(admin, "/system/roles", http.StatusOK), "code", roleCode); role != nil {
		roleID = text(role, "id")
	} else {
		roleID = text(admin.mustPost("/system/roles", map[string]any{
			"code": roleCode,
			"name": "Stage 4B approver",
		}, http.StatusCreated), "id")
	}
	status, payload := admin.put("/system/roles/"+roleID+"/permissions", map[string]any{
		"permission_codes": []string{
			"sales.order.approve",
			"sales.order.read",
			"system.approval.read",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("set stage4b approver permissions status = %d, payload = %v", status, payload)
	}
	return roleID
}

func stage4BEnsureApproverUser(t *testing.T, admin *client, roleID string) (*client, string) {
	t.Helper()
	const username = "demo_company"
	users := listItems(admin, "/system/users?keyword="+url.QueryEscape(username)+"&page_size=20", http.StatusOK)
	user := findItem(users, "username", username)
	if user == nil {
		t.Fatalf("demo user %s not found", username)
	}
	userID := text(user, "id")

	existing := admin.mustGet("/system/users/"+userID+"/roles", http.StatusOK)
	roleIDs := make([]string, 0)
	if rawRoleIDs, ok := existing["role_ids"].([]any); ok {
		for _, raw := range rawRoleIDs {
			if id, ok := raw.(string); ok {
				roleIDs = append(roleIDs, id)
			}
		}
	}
	alreadyAssigned := false
	for _, id := range roleIDs {
		if id == roleID {
			alreadyAssigned = true
			break
		}
	}
	if !alreadyAssigned {
		roleIDs = append(roleIDs, roleID)
	}
	admin.mustPost("/system/users/"+userID+"/roles", map[string]any{
		"role_ids": roleIDs,
	}, http.StatusOK)
	return newClient(t).login(username, env("TEST_DEMO_PASSWORD", "Demo@123456")), userID
}

// stage4BEnsureSelfScopeApprover creates an approval role and a user with only
// the approval permissions. The user is created through the API with an org_id
// but no org_memberships row, so the auth middleware resolves data_scope=self.
func stage4BEnsureSelfScopeApprover(t *testing.T, admin *client) (*client, string, string) {
	t.Helper()

	hq := findItem(getArray(admin, "/system/orgs", http.StatusOK), "code", "HQ")
	if hq == nil {
		t.Fatal("HQ organization not found")
	}
	hqID := text(hq, "id")

	const roleCode = "stage4b_self_approver"
	var roleID string
	if role := findItem(getArray(admin, "/system/roles", http.StatusOK), "code", roleCode); role != nil {
		roleID = text(role, "id")
	} else {
		roleID = text(admin.mustPost("/system/roles", map[string]any{
			"code": roleCode,
			"name": "Stage 4B self approver",
		}, http.StatusCreated), "id")
	}
	status, payload := admin.put("/system/roles/"+roleID+"/permissions", map[string]any{
		"permission_codes": []string{
			"sales.order.approve",
			"system.approval.read",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("set stage4b self approver permissions status = %d, payload = %v", status, payload)
	}

	const username = "stage4b_self_approver"
	users := listItems(admin, "/system/users?keyword="+url.QueryEscape(username)+"&page_size=20", http.StatusOK)
	var userID string
	if user := findItem(users, "username", username); user != nil {
		userID = text(user, "id")
		status, payload := admin.put("/system/users/"+userID, map[string]any{
			"display_name":   "Stage 4B self approver",
			"org_id":         hqID,
			"status":         "active",
			"password":       "Demo@123456",
			"is_super_admin": false,
		})
		if status != http.StatusOK {
			t.Fatalf("update stage4b self approver status = %d, payload = %v", status, payload)
		}
		admin.mustPost("/system/users/"+userID+"/roles", map[string]any{
			"role_ids": []string{roleID},
		}, http.StatusOK)
	} else {
		userID = text(admin.mustPost("/system/users", map[string]any{
			"username":       username,
			"display_name":   "Stage 4B self approver",
			"password":       "Demo@123456",
			"org_id":         hqID,
			"is_super_admin": false,
			"role_ids":       []string{roleID},
		}, http.StatusCreated), "id")
	}

	return newClient(t).login(username, "Demo@123456"), userID, roleID
}

func stage4BRawPost(c *client, path string, body any) (int, string, map[string]any, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, "", nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+apiPrefix+path, reader)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := stage4BHTTPClient.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	var payload apiPayload
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return resp.StatusCode, "", nil, fmt.Errorf("decode response body: %w", err)
		}
	}
	return resp.StatusCode, payload.Code, payload.Data, nil
}

func stage4BBool(t *testing.T, m map[string]any, key string) bool {
	t.Helper()
	value, ok := m[key].(bool)
	if !ok {
		t.Fatalf("field %s = %v, want bool", key, m[key])
	}
	return value
}

func stage4BInt(t *testing.T, m map[string]any, key string) int {
	t.Helper()
	value, ok := m[key].(float64)
	if !ok {
		t.Fatalf("field %s = %v, want number", key, m[key])
	}
	return int(value)
}

func stage4BArray(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("field %s is missing", key)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("field %s = %v, want array", key, raw)
	}
	return items
}

func stage4BUnique(prefix string) string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
