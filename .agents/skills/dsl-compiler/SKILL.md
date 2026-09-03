---
name: dsl-compiler
description: Expert rules for Cedar-like DSL Lexer, Pratt Parser, contains operator, and AST optimization.
---

# Policy DSL Parser & Compiler Skill

## 🎯 Mission
Parse declarative policy syntax into immutable, type-checked, optimized AST nodes in $< 10\,\mu\text{s}$.

## 🔑 Critical Implementation Rules
1. **Grammar & Operators**:
   - Clauses: `permit(...) [when/unless {...}];` and `forbid(...) [when/unless {...}];`.
   - Scopes: `principal`, `action`, `resource` using `==`, `in`, or `any`.
   - Operators: `&&`, `||`, `!`, `==`, `!=`, `>`, `<`, `>=`, `<=`, `in`, `contains`.
   - **`contains` vs `in` distinction**:
     - `contains`: Checks if a comma-separated string contains an element (e.g. `context.delegation_chain contains resource.creator_id` for SoD).
     - `in`: Checks role inheritance closure or IP in CIDR (`principal in role:manager`, `context.ip in 10.0.0.0/8`).

2. **Pratt Parser & Depth Limit**:
   - Operator precedence: `OR < AND < EQUALS < LESSGREATER < PREFIX`.
   - **AST Depth Limit**: Strictly reject expressions with depth $> 15$ to prevent stack overflow DoS.

3. **Compiler Pre-parsing & Optimization**:
   - Constant folding: Evaluate constant subtrees (e.g. `true && false`) at compile time.
   - Pre-parse IP CIDR to `net.IPNet` and DateTime to `int64` Unix timestamp for zero-alloc hot path.

## 📂 Source Files
- [`internal/parser/lexer.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/lexer.go)
- [`internal/parser/parser.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/parser.go)
- [`internal/parser/compiler.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/compiler.go)
- [`configs/policies.cedar`](file:///e:/Projects/Project_TN/standalone-policy-engine/configs/policies.cedar)
- [`docs/technical-spec/POLICY_DSL_SPEC.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/POLICY_DSL_SPEC.md)
