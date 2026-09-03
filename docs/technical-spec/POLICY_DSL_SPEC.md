# POLICY_DSL_SPEC.md — Policy DSL Grammar & P2P Seed Policies

## 1. Formal EBNF Grammar (Cedar-like Policy DSL)

```ebnf
PolicyFile       ::= { Policy } EOF ;

Policy           ::= Effect "(" ScopeList ")" [ ConditionClause ] [ AdviceClause ] ";" ;

Effect           ::= "permit" | "forbid" ;

ScopeList        ::= PrincipalClause "," ActionClause "," ResourceClause ;

PrincipalClause  ::= "principal" ScopeOp EntityRef ;
ActionClause     ::= "action" ScopeOp EntityRef ;
ResourceClause   ::= "resource" ScopeOp EntityRef ;

ScopeOp          ::= "==" | "in" ;
EntityRef        ::= Identifier ":" ( StringLiteral | Identifier ) | "any" ;

ConditionClause  ::= ( "when" | "unless" ) "{" Expression "}" ;

Expression       ::= LogicalOrExpr ;
LogicalOrExpr    ::= LogicalAndExpr { "||" LogicalAndExpr } ;
LogicalAndExpr   ::= EqualityExpr { "&&" EqualityExpr } ;
EqualityExpr     ::= RelationalExpr [ ( "==" | "!=" ) RelationalExpr ] ;
RelationalExpr   ::= MembershipExpr [ ( "<" | "<=" | ">" | ">=" ) MembershipExpr ] ;
MembershipExpr   ::= PrimaryExpr [ ( "in" | "contains" ) PrimaryExpr ] ;

PrimaryExpr      ::= VariableRef 
                   | Literal 
                   | "!" PrimaryExpr 
                   | "(" Expression ")" ;

VariableRef      ::= ( "principal" | "action" | "resource" | "context" ) "." Identifier ;

Literal          ::= StringLiteral 
                   | IntegerLiteral 
                   | BooleanLiteral 
                   | IPAddressLiteral 
                   | CIDRLiteral ;

BooleanLiteral   ::= "true" | "false" ;

/* Phase 2 Academic Extension (Roadmap): Declarative Obligation Block */
AdviceClause     ::= "advice" "{" { StringLiteral [ "," ] } "}" ;
```

---

## 2. Operator Conventions & Engine Resolution Semantics

| Operator | Left Operand Type | Right Operand Type | Resolution Engine Logic | Source Location |
|---|---|---|---|---|
| `==` / `!=` | `String` / `Int64` / `Bool` / `IP` | Identical Type | Exact value comparison | [`evaluator.go:409`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L409) |
| `<` / `<=` / `>` / `>=` | `Int64` | `Int64` | Signed 64-bit integer comparison (`strconv.ParseInt`) | [`evaluator.go:343`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L343) |
| **`in`** | `ValueTypeIP` | `ValueTypeIPNet` | **CIDR Subnet containment**: `IPNet.Contains(IP)` | [`evaluator.go:366`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L366) |
| **`in`** | `ValueTypeString` | `ValueTypeString` | **Role DAG Transitive Closure**: `RoleDAG.IsDescendant(parent, child)` | [`evaluator.go:368`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L368) |
| **`contains`** | `ValueTypeString` (CSV) | `ValueTypeString` (Item) | **Array Membership**: Splits Left by comma `,` and checks exact string match | [`evaluator.go:387`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/engine/evaluator.go#L387) |

---

## 3. Production P2P Seed Policies (`policies.cedar`)

```cedar
// ============================================================================
// RULE 1: Nhân viên được phép tạo Purchase Order (Draft)
// ============================================================================
permit(
    principal in role:staff,
    action    == action:CREATE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.device_status == "clean"
};

// ============================================================================
// RULE 2: Trưởng phòng được duyệt Purchase Order <= $5,000 trong phòng ban
// ============================================================================
permit(
    principal in role:department_manager,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount <= 5000 &&
    principal.department == resource.department
};

// ============================================================================
// RULE 3: Cấm phê duyệt nếu vượt quá hạn mức $5,000 của Trưởng phòng
// (Kích hoạt Obligation REQUIRE_HUMAN_APPROVAL tại Decision Synthesizer)
// ============================================================================
forbid(
    principal in role:department_manager,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 5000
};

// ============================================================================
// RULE 4: AI Agent được tự động duyệt PO nhỏ trong trần tự hành (<= $2,000)
// ============================================================================
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

// ============================================================================
// RULE 5: Rào chắn cấm AI tự động duyệt PO vượt trần tự hành (> $2,000)
// (Chống Prompt Injection & Kích hoạt Human-in-the-Loop)
// ============================================================================
forbid(
    principal == agent:procurement_copilot,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.amount > 2000
};

// ============================================================================
// RULE 6: BẢO TOÀN PHÂN TÁCH TRÁCH NHIỆM TỔNG QUÁT (GENERALIZED SOD)
// Nghiêm cấm mọi hành vi tự duyệt (kể cả mượn AI Agent để duyệt hộ)
// Sử dụng toán tử CONTAINS kiểm tra chuỗi phân tách bởi dấu phẩy
// ============================================================================
forbid(
    principal == any,
    action    == action:APPROVE_PURCHASE_ORDER,
    resource  == any
)
when {
    context.delegation_chain contains resource.creator_id
};
```
