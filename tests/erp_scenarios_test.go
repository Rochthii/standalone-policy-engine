package tests

import (
	"context"
	"sync"
	"testing"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
)

// TestERP_PurchaseOrderApproval kiểm tra kịch bản duyệt Đơn mua hàng (PO) theo hạn mức tài chính và phòng ban.
func TestERP_PurchaseOrderApproval(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-procurement"

	// Khởi tạo các chính sách phân cấp mua hàng
	policies := []*parser.PolicyNode{
		// P-PO-01: Nhân viên được phép tạo PO cho phòng ban của mình
		compileHelperTest(t, compiler, "P-PO-01", `
			permit(
				principal in role:staff,
				action    == action:CREATE_PO,
				resource  == resource:purchase_order
			)
			when {
				principal.department == resource.department
			};
		`),
		// P-PO-02: Trưởng phòng duyệt PO thuộc phòng ban mình với hạn mức <= 50.000.000 VNĐ
		compileHelperTest(t, compiler, "P-PO-02", `
			permit(
				principal in role:department_manager,
				action    == action:APPROVE_PO,
				resource  == resource:purchase_order
			)
			when {
				principal.department == resource.department &&
				resource.amount <= 50000000 &&
				context.ip_address in "10.0.0.0/8" &&
				context.request_time >= "08:00:00Z" &&
				context.request_time <= "18:00:00Z"
			};
		`),
		// P-PO-03: Giám đốc khối duyệt PO với hạn mức <= 500.000.000 VNĐ
		compileHelperTest(t, compiler, "P-PO-03", `
			permit(
				principal in role:director,
				action    == action:APPROVE_PO,
				resource  == resource:purchase_order
			)
			when {
				resource.amount <= 500000000 &&
				context.ip_address in "10.0.0.0/8"
			};
		`),
		// P-PO-04: Giám đốc tài chính (CFO) duyệt mọi hạn mức PO từ mạng nội bộ và thiết bị an toàn
		compileHelperTest(t, compiler, "P-PO-04", `
			permit(
				principal in role:cfo,
				action    == action:APPROVE_PO,
				resource  == resource:purchase_order
			)
			when {
				context.ip_address in "10.0.0.0/8" &&
				context.device_status == "secure"
			};
		`),
		// P-PO-05: Cấm tuyệt đối phê duyệt nếu thiết bị của người duyệt bị xâm hại (Compromised)
		compileHelperTest(t, compiler, "P-PO-05", `
			forbid(
				principal == any,
				action    == action:APPROVE_PO,
				resource  == any
			)
			when {
				context.device_status == "compromised"
			};
		`),
	}

	// Thiết lập phân cấp vai trò: user -> role
	inheritances := [][2]string{
		{"user:alice", "role:staff"},
		{"user:bob", "role:department_manager"},
		{"user:charlie", "role:director"},
		{"user:david", "role:cfo"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: Alice (Staff IT) tạo PO cho phòng IT -> ALLOW
	ctxAlice := map[string]string{
		"principal.department": "IT",
		"resource.department":  "IT",
	}
	res := eng.CheckPermission(ctx, tenantID, "user:alice", "CREATE_PO", "resource:purchase_order", ctxAlice)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Alice tạo PO IT, nhận được: %s (%s)", res.Decision, res.Reason)
	}

	// Case 2: Alice tạo PO cho phòng Kế toán (Finance) -> DENY (Sai phòng ban)
	ctxAliceWrongDept := map[string]string{
		"principal.department": "IT",
		"resource.department":  "FINANCE",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:alice", "CREATE_PO", "resource:purchase_order", ctxAliceWrongDept)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY cho Alice tạo PO khác phòng ban, nhận được ALLOW")
	}

	// Case 3: Bob (Manager IT) duyệt PO 30.000.000 VNĐ phòng IT trong giờ hành chính từ IP 10.1.2.3 -> ALLOW
	ctxBobAllow := map[string]string{
		"principal.department":  "IT",
		"resource.department":   "IT",
		"resource.amount":       "30000000",
		"context.ip_address":    "10.1.2.3",
		"context.request_time":  "14:30:00Z",
		"context.device_status": "secure",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:bob", "APPROVE_PO", "resource:purchase_order", ctxBobAllow)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Bob duyệt PO 30M, nhận được: %s (%s)", res.Decision, res.Reason)
	}

	// Case 4: Bob duyệt PO 70.000.000 VNĐ -> DENY (Vượt quá hạn mức 50M của Manager)
	ctxBobExceedLimit := map[string]string{
		"principal.department":  "IT",
		"resource.department":   "IT",
		"resource.amount":       "70000000",
		"context.ip_address":    "10.1.2.3",
		"context.request_time":  "14:30:00Z",
		"context.device_status": "secure",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:bob", "APPROVE_PO", "resource:purchase_order", ctxBobExceedLimit)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi Bob duyệt PO 70M vượt hạn mức, nhận được ALLOW")
	}

	// Case 5: Bob duyệt PO ngoài giờ hành chính (21:00:00Z) -> DENY
	ctxBobAfterHours := map[string]string{
		"principal.department":  "IT",
		"resource.department":   "IT",
		"resource.amount":       "30000000",
		"context.ip_address":    "10.1.2.3",
		"context.request_time":  "21:00:00Z",
		"context.device_status": "secure",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:bob", "APPROVE_PO", "resource:purchase_order", ctxBobAfterHours)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi Bob duyệt ngoài giờ, nhận được ALLOW")
	}

	// Case 6: Charlie (Director) duyệt PO 200.000.000 VNĐ -> ALLOW
	ctxCharlie := map[string]string{
		"resource.amount":       "200000000",
		"context.ip_address":    "10.5.0.1",
		"context.device_status": "secure",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:charlie", "APPROVE_PO", "resource:purchase_order", ctxCharlie)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Charlie duyệt PO 200M, nhận được: %s (%s)", res.Decision, res.Reason)
	}

	// Case 7: David (CFO) duyệt PO 2.000.000.000 VNĐ -> ALLOW
	ctxDavid := map[string]string{
		"resource.amount":       "2000000000",
		"context.ip_address":    "10.0.1.1",
		"context.device_status": "secure",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:david", "APPROVE_PO", "resource:purchase_order", ctxDavid)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho David (CFO) duyệt PO 2 tỷ, nhận được: %s (%s)", res.Decision, res.Reason)
	}

	// Case 8: David (CFO) duyệt nhưng máy bị mã độc (device_status == "compromised") -> DENY (Forbid override)
	ctxDavidCompromised := map[string]string{
		"resource.amount":       "2000000000",
		"context.ip_address":    "10.0.1.1",
		"context.device_status": "compromised",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:david", "APPROVE_PO", "resource:purchase_order", ctxDavidCompromised)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi máy bị compromised, nhận được ALLOW")
	}
}

// TestERP_SeparationOfDuties kiểm tra quy tắc Phân tách nhiệm vụ (SoD): Người lập phiếu chi không được tự ký duyệt.
func TestERP_SeparationOfDuties(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-finance"

	policies := []*parser.PolicyNode{
		// Kế toán trưởng được duyệt phiếu chi
		compileHelperTest(t, compiler, "P-SOD-01", `
			permit(
				principal in role:chief_accountant,
				action    == action:APPROVE_PAYMENT,
				resource  == resource:payment_voucher
			);
		`),
		// Luật cấm SoD: Bất kỳ ai là người tạo (creator_id) thì không được duyệt
		compileHelperTest(t, compiler, "P-SOD-02", `
			forbid(
				principal == any,
				action    == action:APPROVE_PAYMENT,
				resource  == resource:payment_voucher
			)
			when {
				principal.id == resource.creator_id
			};
		`),
	}

	inheritances := [][2]string{
		{"user:emma", "role:chief_accountant"},
		{"user:frank", "role:chief_accountant"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: Emma là người tạo phiếu V-101 (creator_id = user:emma). Emma tự duyệt -> DENY (Vi phạm SoD)
	ctxEmmaSelfApprove := map[string]string{
		"resource.creator_id": "user:emma",
	}
	res := eng.CheckPermission(ctx, tenantID, "user:emma", "APPROVE_PAYMENT", "resource:payment_voucher", ctxEmmaSelfApprove)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY vì vi phạm SoD tự duyệt phiếu của mình, nhận được ALLOW")
	}

	// Case 2: Frank duyệt phiếu V-101 do Emma tạo -> ALLOW (Đúng nguyên tắc SoD chéo)
	ctxFrankApprove := map[string]string{
		"resource.creator_id": "user:emma",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:frank", "APPROVE_PAYMENT", "resource:payment_voucher", ctxFrankApprove)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Frank duyệt phiếu do Emma tạo, nhận được: %s (%s)", res.Decision, res.Reason)
	}
}

// TestERP_MultiBranchIsolation kiểm tra cách ly chứng từ chi nhánh và quyền kiểm toán tập đoàn.
func TestERP_MultiBranchIsolation(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-branch"

	policies := []*parser.PolicyNode{
		// Kế toán chi nhánh xem chứng từ chi nhánh mình HOẶC là kiểm toán viên tập đoàn
		compileHelperTest(t, compiler, "P-BRANCH-01", `
			permit(
				principal in role:accountant,
				action    == action:VIEW_INVOICE,
				resource  == resource:invoice
			)
			when {
				principal.branch_id == resource.branch_id ||
				principal.is_group_auditor == "true"
			};
		`),
	}

	inheritances := [][2]string{
		{"user:hn_accountant", "role:accountant"},
		{"user:hcm_accountant", "role:accountant"},
		{"user:group_auditor", "role:accountant"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: Kế toán Hà Nội xem hóa đơn Hà Nội -> ALLOW
	ctxHN := map[string]string{
		"principal.branch_id":        "BRANCH_HN",
		"resource.branch_id":         "BRANCH_HN",
		"principal.is_group_auditor": "false",
	}
	res := eng.CheckPermission(ctx, tenantID, "user:hn_accountant", "VIEW_INVOICE", "resource:invoice", ctxHN)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Kế toán HN xem hóa đơn HN, nhận được: %s", res.Decision)
	}

	// Case 2: Kế toán Hà Nội cố xem hóa đơn TP.HCM -> DENY (Cách ly chi nhánh)
	ctxHNCross := map[string]string{
		"principal.branch_id":        "BRANCH_HN",
		"resource.branch_id":         "BRANCH_HCM",
		"principal.is_group_auditor": "false",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:hn_accountant", "VIEW_INVOICE", "resource:invoice", ctxHNCross)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi Kế toán HN xem hóa đơn HCM, nhận được ALLOW")
	}

	// Case 3: Kiểm toán tập đoàn (ở HN) xem hóa đơn TP.HCM -> ALLOW (Nhờ is_group_auditor == true)
	ctxAuditor := map[string]string{
		"principal.branch_id":        "BRANCH_HN",
		"resource.branch_id":         "BRANCH_HCM",
		"principal.is_group_auditor": "true",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:group_auditor", "VIEW_INVOICE", "resource:invoice", ctxAuditor)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho Kiểm toán tập đoàn xem hóa đơn HCM, nhận được: %s", res.Decision)
	}
}

// TestERP_PayrollShielding kiểm tra bảo mật dữ liệu lương nhạy cảm trong ERP.
func TestERP_PayrollShielding(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-hr"

	policies := []*parser.PolicyNode{
		// Cho phép chuyên viên nhân sự xem bảng lương từ dải mạng an toàn
		compileHelperTest(t, compiler, "P-HR-01", `
			permit(
				principal in role:hr_specialist,
				action    == action:VIEW_PAYROLL,
				resource  == resource:payroll
			)
			when {
				context.ip_address in "10.10.0.0/16" &&
				context.request_time >= "08:00:00Z" &&
				context.request_time <= "18:00:00Z"
			};
		`),
		// Cấm tuyệt đối truy cập bảng lương nếu kết nối từ Wifi công cộng
		compileHelperTest(t, compiler, "P-HR-02", `
			forbid(
				principal == any,
				action    == any,
				resource  == resource:payroll
			)
			when {
				context.network_type == "public_wifi"
			};
		`),
	}

	inheritances := [][2]string{
		{"user:grace", "role:hr_specialist"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: Chuyên viên HR xem bảng lương lúc 10:00 từ IP an toàn 10.10.5.20 -> ALLOW
	ctxValid := map[string]string{
		"context.ip_address":   "10.10.5.20",
		"context.request_time": "10:00:00Z",
		"context.network_type": "vpn_internal",
	}
	res := eng.CheckPermission(ctx, tenantID, "user:grace", "VIEW_PAYROLL", "resource:payroll", ctxValid)
	if res.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho HR xem bảng lương hợp lệ, nhận được: %s (%s)", res.Decision, res.Reason)
	}

	// Case 2: Chuyên viên HR truy cập lúc 23:00 (Nửa đêm) -> DENY
	ctxMidnight := map[string]string{
		"context.ip_address":   "10.10.5.20",
		"context.request_time": "23:00:00Z",
		"context.network_type": "vpn_internal",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:grace", "VIEW_PAYROLL", "resource:payroll", ctxMidnight)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi HR truy cập ngoài giờ, nhận được ALLOW")
	}

	// Case 3: Chuyên viên HR truy cập từ Public Wifi -> DENY (Forbid override)
	ctxPublicWifi := map[string]string{
		"context.ip_address":   "10.10.5.20",
		"context.request_time": "10:00:00Z",
		"context.network_type": "public_wifi",
	}
	res = eng.CheckPermission(ctx, tenantID, "user:grace", "VIEW_PAYROLL", "resource:payroll", ctxPublicWifi)
	if res.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY khi dùng Public Wifi, nhận được ALLOW")
	}
}

// TestERP_RoleHierarchyDAG kiểm tra phân cấp chức danh đa tầng trong doanh nghiệp ERP.
func TestERP_RoleHierarchyDAG(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-dag"

	// Chính sách: Quyền xem báo cáo chiến lược doanh nghiệp chỉ cấp cho role:director
	policies := []*parser.PolicyNode{
		compileHelperTest(t, compiler, "P-DAG-01", `
			permit(
				principal in role:director,
				action    == action:VIEW_STRATEGY,
				resource  == resource:strategic_plan
			);
		`),
	}

	// Phân cấp vai trò đa tầng: CEO -> VP -> Director -> Manager -> Staff
	inheritances := [][2]string{
		{"user:ceo", "role:c_level"},
		{"role:c_level", "role:vp"},
		{"role:vp", "role:director"},
		{"role:director", "role:manager"},
		{"role:manager", "role:staff"},
		{"user:staff_john", "role:staff"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: CEO kế thừa gián tiếp role:director qua C-Level và VP -> ALLOW (Bao đóng bắc cầu O(1))
	resCEO := eng.CheckPermission(ctx, tenantID, "user:ceo", "VIEW_STRATEGY", "resource:strategic_plan", nil)
	if resCEO.Decision != engine.DecisionAllow {
		t.Errorf("Mong đợi ALLOW cho CEO kế thừa Director, nhận được: %s (%s)", resCEO.Decision, resCEO.Reason)
	}

	// Case 2: Nhân viên John chỉ có role:staff (dưới Director) -> DENY
	resJohn := eng.CheckPermission(ctx, tenantID, "user:staff_john", "VIEW_STRATEGY", "resource:strategic_plan", nil)
	if resJohn.Decision != engine.DecisionDeny {
		t.Errorf("Mong đợi DENY cho nhân viên John xem báo cáo chiến lược, nhận được ALLOW")
	}
}

// TestERP_ExplainDecision kiểm tra tính năng giải thích quyết định trả về chính sách khớp chính xác.
func TestERP_ExplainDecision(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-explain"

	policies := []*parser.PolicyNode{
		compileHelperTest(t, compiler, "P-EXP-01", `
			permit(
				principal == user:"alice",
				action    == action:READ,
				resource  == resource:contract
			)
			when {
				principal.department == resource.department
			};
		`),
		compileHelperTest(t, compiler, "P-EXP-02", `
			forbid(
				principal == user:"alice",
				action    == any,
				resource  == resource:contract
			)
			when {
				context.security_clearance < 3
			};
		`),
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, nil); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	ctx := context.Background()

	// Case 1: Alice có clearance >= 3 -> ALLOW, Explanation = ["P-EXP-01"]
	ctxPass := map[string]string{
		"principal.department":       "LEGAL",
		"resource.department":        "LEGAL",
		"context.security_clearance": "4",
	}
	resPass := eng.CheckPermission(ctx, tenantID, "user:alice", "READ", "resource:contract", ctxPass)
	if resPass.Decision != engine.DecisionAllow {
		t.Fatalf("Mong đợi ALLOW, nhận được: %s", resPass.Decision)
	}
	if len(resPass.Explanations) != 1 || resPass.Explanations[0] != "P-EXP-01" {
		t.Errorf("Mong đợi giải thích là ['P-EXP-01'], nhận được: %v", resPass.Explanations)
	}

	// Case 2: Alice có clearance < 3 -> DENY, Explanation = ["P-EXP-02"] (Forbid)
	ctxForbid := map[string]string{
		"principal.department":       "LEGAL",
		"resource.department":        "LEGAL",
		"context.security_clearance": "2",
	}
	resForbid := eng.CheckPermission(ctx, tenantID, "user:alice", "READ", "resource:contract", ctxForbid)
	if resForbid.Decision != engine.DecisionDeny {
		t.Fatalf("Mong đợi DENY, nhận được: %s", resForbid.Decision)
	}
	if len(resForbid.Explanations) != 1 || resForbid.Explanations[0] != "P-EXP-02" {
		t.Errorf("Mong đợi giải thích cấm là ['P-EXP-02'], nhận được: %v", resForbid.Explanations)
	}
}

// TestERP_ConcurrentEvaluation kiểm tra an toàn đa luồng và hiệu năng cao khi hàng trăm Goroutines gọi đồng thời.
func TestERP_ConcurrentEvaluation(t *testing.T) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-conc"

	policies := []*parser.PolicyNode{
		compileHelperTest(t, compiler, "P-CONC-01", `
			permit(
				principal in role:operator,
				action    == action:UPDATE_STOCK,
				resource  == resource:warehouse_item
			)
			when {
				principal.warehouse_id == resource.warehouse_id &&
				context.ip_address in "10.0.0.0/8"
			};
		`),
	}

	inheritances := [][2]string{
		{"user:operator_1", "role:operator"},
	}

	if err := eng.UpdateTenantPolicies(tenantID, policies, inheritances); err != nil {
		t.Fatalf("UpdateTenantPolicies thất bại: %v", err)
	}

	concurrency := 100
	iterations := 500
	var wg sync.WaitGroup
	wg.Add(concurrency)

	ctx := context.Background()
	reqCtx := map[string]string{
		"principal.warehouse_id": "WH_NORTH",
		"resource.warehouse_id":  "WH_NORTH",
		"context.ip_address":     "10.2.3.4",
	}

	for c := 0; c < concurrency; c++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				res := eng.CheckPermission(ctx, tenantID, "user:operator_1", "UPDATE_STOCK", "resource:warehouse_item", reqCtx)
				if res.Decision != engine.DecisionAllow {
					t.Errorf("Mong đợi ALLOW trong luồng đồng thời, nhận được: %s (%s)", res.Decision, res.Reason)
					return
				}
			}
		}()
	}

	wg.Wait()
}

// BenchmarkERP_PurchaseOrderEvaluation đo lường tốc độ đánh giá quyết định cho kịch bản PO thực tế.
func BenchmarkERP_PurchaseOrderEvaluation(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-bench-po"

	policies := []*parser.PolicyNode{
		compileHelperBench(b, compiler, "P-PO-01", `
			permit(
				principal in role:department_manager,
				action    == action:APPROVE_PO,
				resource  == resource:purchase_order
			)
			when {
				principal.department == resource.department &&
				resource.amount <= 50000000 &&
				context.ip_address in "10.0.0.0/8" &&
				context.request_time >= "08:00:00Z" &&
				context.request_time <= "18:00:00Z"
			};
		`),
	}

	inheritances := [][2]string{
		{"user:manager_bob", "role:department_manager"},
	}

	_ = eng.UpdateTenantPolicies(tenantID, policies, inheritances)

	ctx := context.Background()
	reqCtx := map[string]string{
		"principal.department": "IT",
		"resource.department":  "IT",
		"resource.amount":      "25000000",
		"context.ip_address":   "10.1.2.3",
		"context.request_time": "14:00:00Z",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := eng.CheckPermission(ctx, tenantID, "user:manager_bob", "APPROVE_PO", "resource:purchase_order", reqCtx)
		if res.Decision != engine.DecisionAllow {
			b.Fatalf("Mong đợi ALLOW, nhận được: %v", res.Decision)
		}
	}
}

// BenchmarkERP_ConcurrentMultiTenant đo lường thông lượng đa khách thuê và đa luồng.
func BenchmarkERP_ConcurrentMultiTenant(b *testing.B) {
	eng := engine.NewEngine()
	compiler := parser.NewCompiler()
	tenantID := "erp-tenant-bench-conc"

	policies := []*parser.PolicyNode{
		compileHelperBench(b, compiler, "P-PO-01", `
			permit(
				principal in role:department_manager,
				action    == action:APPROVE_PO,
				resource  == resource:purchase_order
			)
			when {
				principal.department == resource.department &&
				resource.amount <= 50000000 &&
				context.ip_address in "10.0.0.0/8"
			};
		`),
	}

	inheritances := [][2]string{
		{"user:manager_bob", "role:department_manager"},
	}

	_ = eng.UpdateTenantPolicies(tenantID, policies, inheritances)

	ctx := context.Background()
	reqCtx := map[string]string{
		"principal.department": "IT",
		"resource.department":  "IT",
		"resource.amount":      "25000000",
		"context.ip_address":   "10.1.2.3",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := eng.CheckPermission(ctx, tenantID, "user:manager_bob", "APPROVE_PO", "resource:purchase_order", reqCtx)
			if res.Decision != engine.DecisionAllow {
				b.Fatalf("Mong đợi ALLOW, nhận được: %v", res.Decision)
			}
		}
	})
}

// Helper biên dịch DSL trong benchmark
func compileHelperBench(b *testing.B, c *parser.Compiler, id, dsl string) *parser.PolicyNode {
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()

	if len(p.Errors()) > 0 {
		b.Fatalf("Lỗi parse DSL [%s]: %v", id, p.Errors())
	}

	nodes[0].ID = id
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		b.Fatalf("Lỗi compile DSL [%s]: %v", id, err)
	}

	return compiled
}

// Helper biên dịch DSL trong test
func compileHelperTest(t *testing.T, c *parser.Compiler, id, dsl string) *parser.PolicyNode {
	l := parser.NewLexer(dsl)
	p := parser.NewParser(l)
	nodes := p.Parse()

	if len(p.Errors()) > 0 {
		t.Fatalf("Lỗi parse DSL [%s]: %v", id, p.Errors())
	}
	if len(nodes) == 0 {
		t.Fatalf("Không có node nào được parse từ DSL [%s]", id)
	}

	nodes[0].ID = id
	compiled, err := c.Compile(nodes[0])
	if err != nil {
		t.Fatalf("Lỗi compile DSL [%s]: %v", id, err)
	}

	return compiled
}
