---
name: dsl-compiler
description: Expert rules and constraints for Cedar-like DSL Lexer, Pratt Parser, AST optimization, and Compiler.
---

# Policy DSL Parser & Compiler Skill

## 🎯 Mission
Parse declarative policy syntax into immutable, type-checked, optimized AST nodes.

## 🔑 Critical Implementation Rules
1. **Grammar & Semantics**:
   - Syntax: `permit(...) [when/unless {...}];` and `forbid(...) [when/unless {...}];`.
   - Scope: `principal`, `action`, `resource` using operators `==` or `in` with `Type:ID` or `any`.
   - Condition operators: `&&`, `||`, `!`, `==`, `!=`, `>`, `<`, `>=`, `<=`, `in`, `contains`.
2. **Pratt Parser**:
   - Operator precedence: `OR < AND < EQUALS < LESSGREATER < PREFIX`.
   - Error collection: Must record exact line, column, and offset for syntax errors.
3. **Compiler Optimization & Security**:
   - **AST Depth Limit**: Strictly reject trees with depth $> 15$ to prevent Stack Overflow DoS.
   - **Pre-parsing**: Convert IP strings to `ValueTypeIP` / `ValueTypeIPNet` and DateTime/Time strings to `ValueTypeDateTime` (Unix nanoseconds or seconds since midnight).
   - **Constant Folding**: Evaluate constant subtrees (e.g. `true && false`, `10 > 5`) at compile time.
   - **Double-Wildcard Rule**: Reject tenant activation if wildcard policies (`principal == any && resource == any`) exceed $5\%$ of total policies.

## 📂 Source Files
- [`internal/parser/lexer.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/lexer.go)
- [`internal/parser/parser.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/parser.go)
- [`internal/parser/ast.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/ast.go)
- [`internal/parser/compiler.go`](file:///e:/Projects/Project_TN/standalone-policy-engine/internal/parser/compiler.go)
