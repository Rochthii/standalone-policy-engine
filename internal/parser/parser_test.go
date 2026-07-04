package parser

import (
	"strings"
	"testing"
)

func TestLexer(t *testing.T) {
	input := `permit(principal == user:"alice", action == action:READ, resource == any)
when {
	context.ip_address in "192.168.1.0/24" &&
	context.age >= 18
};`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{TokPermit, "permit"},
		{TokLParen, "("},
		{TokPrincipal, "principal"},
		{TokEq, "=="},
		{TokIdent, "user"},
		{TokColon, ":"},
		{TokString, "alice"},
		{TokComma, ","},
		{TokAction, "action"},
		{TokEq, "=="},
		{TokIdent, "action"},
		{TokColon, ":"},
		{TokString, "READ"},
		{TokComma, ","},
		{TokResource, "resource"},
		{TokEq, "=="},
		{TokAny, "any"},
		{TokRParen, ")"},
		{TokWhen, "when"},
		{TokLBrace, "{"},
		{TokIdent, "context"},
		{TokDot, "."},
		{TokIdent, "ip_address"},
		{TokIn, "in"},
		{TokString, "192.168.1.0/24"},
		{TokAnd, "&&"},
		{TokIdent, "context"},
		{TokDot, "."},
		{TokIdent, "age"},
		{TokGe, ">="},
		{TokInt, "18"},
		{TokRBrace, "}"},
		{TokSemicolon, ";"},
		{TokEOF, ""},
	}

	lexer := NewLexer(input)
	for i, tt := range tests {
		tok := lexer.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - kiểu token sai. mong đợi=%q, thực tế=%q (literal: %q)", i, tt.expectedType, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal sai. mong đợi=%q, thực tế=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestParser(t *testing.T) {
	tests := []struct {
		input           string
		expectError     bool
		expectedEffect  Effect
		expectedScopeOp ScopeOp
	}{
		{
			input:           `permit(principal == user:"alice", action == action:READ, resource == any);`,
			expectError:     false,
			expectedEffect:  EffectPermit,
			expectedScopeOp: ScopeOpEq,
		},
		{
			input:           `forbid(principal in role:operator, action == action:DELETE, resource == file:"report.pdf") when { context.request_time < "17:00:00" };`,
			expectError:     false,
			expectedEffect:  EffectForbid,
			expectedScopeOp: ScopeOpIn,
		},
		{
			input:       `permit(principal == user:"alice", action == action:READ, resource == any)`, // thiếu dấu ;
			expectError: true,
		},
		{
			input:       `permit(principal = user:"alice", action == action:READ, resource == any);`, // sai toán tử scope
			expectError: true,
		},
	}

	for i, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		policies := p.Parse()

		if tt.expectError {
			if len(p.Errors()) == 0 {
				t.Fatalf("tests[%d] - mong đợi xảy ra lỗi cú pháp nhưng không có lỗi nào", i)
			}
		} else {
			if len(p.Errors()) > 0 {
				t.Fatalf("tests[%d] - xảy ra lỗi cú pháp không mong muốn: %v", i, p.Errors())
			}
			if len(policies) != 1 {
				t.Fatalf("tests[%d] - mong đợi parse được 1 policy, thực tế: %d", i, len(policies))
			}
			policy := policies[0]
			if policy.Effect != tt.expectedEffect {
				t.Errorf("tests[%d] - effect sai. mong đợi=%v, thực tế=%v", i, tt.expectedEffect, policy.Effect)
			}
			if policy.Principal.Operator != tt.expectedScopeOp {
				t.Errorf("tests[%d] - toán tử principal scope sai. mong đợi=%v, thực tế=%v", i, tt.expectedScopeOp, policy.Principal.Operator)
			}
		}
	}
}

func TestCompiler_IPPreparsing(t *testing.T) {
	input := `permit(principal == user:"alice", action == action:READ, resource == any)
when { context.ip_address in "192.168.1.0/24" };`

	l := NewLexer(input)
	p := NewParser(l)
	policies := p.Parse()

	if len(p.Errors()) > 0 {
		t.Fatalf("Lỗi cú pháp: %v", p.Errors())
	}

	c := NewCompiler()
	compiledPolicy, err := c.Compile(policies[0])
	if err != nil {
		t.Fatalf("Lỗi biên dịch: %v", err)
	}

	// Đảm bảo vế phải của phép so sánh IP đã được pre-parse sang ValueTypeIPNet
	binExpr, ok := compiledPolicy.Condition.(*BinaryExprNode)
	if !ok {
		t.Fatalf("Mong đợi nốt gốc là BinaryExprNode")
	}

	rightVal, ok := binExpr.Right.(*ValueNode)
	if !ok {
		t.Fatalf("Mong đợi vế phải là ValueNode")
	}

	if rightVal.ValType != ValueTypeIPNet {
		t.Errorf("Mong đợi kiểu IPNet, thực tế: %v", rightVal.ValType)
	}

	if rightVal.IPNetVal.String() != "192.168.1.0/24" {
		t.Errorf("Địa chỉ IPNet không khớp. thực tế: %v", rightVal.IPNetVal.String())
	}
}

func TestCompiler_ConstantFolding(t *testing.T) {
	tests := []struct {
		condition      string
		expectedResult bool
	}{
		{"{ 100 > 50 }", true},
		{"{ 50 >= 100 }", false},
		{"{ !true }", false},
		{"{ false && context.some_var }", false}, // Đoản mạch AND
		{"{ true || context.some_var }", true},   // Đoản mạch OR
	}

	for i, tt := range tests {
		input := `permit(principal == any, action == any, resource == any) when ` + tt.condition + `;`
		l := NewLexer(input)
		p := NewParser(l)
		policies := p.Parse()

		if len(p.Errors()) > 0 {
			t.Fatalf("tests[%d] - Lỗi cú pháp: %v", i, p.Errors())
		}

		c := NewCompiler()
		compiled, err := c.Compile(policies[0])
		if err != nil {
			t.Fatalf("tests[%d] - Lỗi biên dịch: %v", i, err)
		}

		valNode, ok := compiled.Condition.(*ValueNode)
		if !ok {
			t.Fatalf("tests[%d] - Constant folding thất bại, không thể thu gọn về ValueNode", i)
		}

		if valNode.ValType != ValueTypeBool {
			t.Fatalf("tests[%d] - Kiểu kết quả sai. mong đợi=Bool, thực tế=%v", i, valNode.ValType)
		}

		if valNode.BoolVal != tt.expectedResult {
			t.Errorf("tests[%d] - Kết quả sai. mong đợi=%v, thực tế=%v", i, tt.expectedResult, valNode.BoolVal)
		}
	}
}

func TestCompiler_ASTDepthLimit(t *testing.T) {
	// Tạo biểu thức lồng nhau sâu hơn 15 cấp
	// Ví dụ: true && true && true ... (16 lần)
	sb := strings.Builder{}
	sb.WriteString("permit(principal == any, action == any, resource == any) when { ")
	for i := 0; i < 16; i++ {
		sb.WriteString("true")
		if i < 15 {
			sb.WriteString(" && ")
		}
	}
	sb.WriteString(" };")

	l := NewLexer(sb.String())
	p := NewParser(l)
	policies := p.Parse()

	if len(p.Errors()) > 0 {
		t.Fatalf("Lỗi cú pháp: %v", p.Errors())
	}

	c := NewCompiler()
	_, err := c.Compile(policies[0])
	if err == nil {
		t.Fatalf("Mong đợi lỗi biên dịch do độ sâu AST vượt quá 15 cấp, nhưng không có lỗi")
	}

	if !strings.Contains(err.Error(), "độ sâu của biểu thức logic vượt quá giới hạn an toàn") {
		t.Errorf("Thông điệp lỗi sai: %v", err)
	}
}
