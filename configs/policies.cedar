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
    action    == action:APPROVE_PURCHASE_ORDER,
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
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 5000
};

// RULE 4: AI Agent được tự động duyệt PO nhỏ trong trần tự hành (<= $2,000)
permit(
    principal == agent:procurement_copilot,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.tool_context == "tool:auto_confirm_po" &&
    context.amount <= 2000 &&
    context.execution_mode == "autonomous_run"
};

// RULE 5: Rào chắn cấm AI tự động duyệt PO vượt trần tự hành (> $2,000)
// (Chống Prompt Injection & Kích hoạt Human-in-the-Loop)
forbid(
    principal == agent:procurement_copilot,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 2000
};

// RULE 6: BẢO TOÀN PHÂN TÁCH TRÁCH NHIỆM TỔNG QUÁT (GENERALIZED SOD)
// Nghiêm cấm mọi hành vi tự duyệt (kể cả mượn AI Agent để duyệt hộ)
// Sử dụng toán tử CONTAINS kiểm tra chuỗi phân tách bởi dấu phẩy
forbid(
    principal == any,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.delegation_chain contains resource.creator_id
};
