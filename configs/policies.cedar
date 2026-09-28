// ============================================================================
// STANDALONE POLICY ENGINE — P2P PROCUREMENT SEED POLICIES
// Standard Procure-to-Pay (P2P) Ruleset with Deterministic AI Guardrails & SoD
// ============================================================================

// RULE 1: Nhân viên được phép tạo Purchase Order (Draft)
permit(
    principal in role:staff,
    action    == action:CREATE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.device_status == "clean"
};

// RULE 2: Trưởng phòng được duyệt Purchase Order <= $5,000 trong phòng ban
permit(
    principal in role:department_manager,
    action    == action:CONFIRM_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount <= 5000 &&
    principal.department == resource.department
};

// RULE 3: Cấm phê duyệt nếu vượt quá hạn mức $5,000 của Trưởng phòng
// (Kích hoạt Obligation REQUIRE_HUMAN_APPROVAL tại Decision Synthesizer)
forbid(
    principal in role:department_manager,
    action    == action:CONFIRM_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 5000
}
obligation REQUIRE_HUMAN_APPROVAL "Purchase order exceeds the department-manager approval limit"
;

// RULE 4: AI Agent được tự động duyệt PO nhỏ trong trần tự hành (<= $2,000)
permit(
    principal == agent:procurement_copilot,
    action    == action:CONFIRM_PURCHASE_ORDER,
    resource  == any
)
when {
    context.tool_context == "tool:auto_confirm_po" &&
    context.amount <= 2000 &&
    context.execution_mode == "autonomous_run"
};

// RULE 5: Cho phép xem xét PO cao tiền nhưng bắt buộc AC trước khi thực thi.
// Hard FORBID khác vẫn ưu tiên và không thể bị AC vượt qua.
permit(
    principal == agent:procurement_copilot,
    action    == action:CONFIRM_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 2000 &&
    context.tool_context == "tool:auto_confirm_po" &&
    context.execution_mode == "autonomous_run"
}
obligation REQUIRE_HUMAN_APPROVAL "Autonomous agent spending requires human approval"
;

// RULE 6: BẢO TOÀN PHÂN TÁCH TRÁCH NHIỆM TỔNG QUÁT (GENERALIZED SOD)
// Nghiêm cấm mọi hành vi tự duyệt (kể cả mượn AI Agent để duyệt hộ)
// Sử dụng toán tử CONTAINS kiểm tra chuỗi phân tách bởi dấu phẩy
forbid(
    principal == any,
    action    == action:CONFIRM_PURCHASE_ORDER,
    resource  == any
)
when {
    context.delegation_chain contains resource.creator_id
};

// RULE 7: Chỉ vai trò phê duyệt mua hàng cùng phòng ban mới được qua cổng human approval.
// Odoo vẫn bắt buộc kiểm tra tenant/company và creator/delegator/agent SoD từ dữ liệu tin cậy.
permit(
    principal in role:purchase_approver,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    (context.approval_state == "pending" ||
     context.approval_state == "approved") &&
    principal.department == resource.department
};
