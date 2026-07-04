package parser

import (
	"fmt"
	"net"
	"strconv"
)

// Các độ ưu tiên toán tử trong mệnh đề điều kiện (Precedence)
const (
	_ int = iota
	LOWEST
	OR          // ||
	AND         // &&
	EQUALS      // ==, !=
	LESSGREATER // >, <, >=, <=, in, contains
	PREFIX      // !
)

// Bảng ánh xạ độ ưu tiên cho các Token
var precedences = map[TokenType]int{
	TokOr:       OR,
	TokAnd:      AND,
	TokEq:       EQUALS,
	TokNe:       EQUALS,
	TokGt:       LESSGREATER,
	TokLt:       LESSGREATER,
	TokGe:       LESSGREATER,
	TokLe:       LESSGREATER,
	TokIn:       LESSGREATER,
	TokContains: LESSGREATER,
}

type (
	prefixParseFn func() Node
	infixParseFn  func(Node) Node
)

// Parser thực hiện phân tích cú pháp dựa trên luồng Token từ Lexer.
type Parser struct {
	l      *Lexer
	errors []string

	curToken  Token
	peekToken Token

	prefixParseFns map[TokenType]prefixParseFn
	infixParseFns  map[TokenType]infixParseFn
}

// NewParser tạo mới một instance Parser.
func NewParser(l *Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[TokenType]prefixParseFn)
	p.registerPrefix(TokIdent, p.parseIdentifier)
	p.registerPrefix(TokInt, p.parseNumber)
	p.registerPrefix(TokString, p.parseString)
	p.registerPrefix(TokIP, p.parseIP)
	p.registerPrefix(TokTrue, p.parseBoolean)
	p.registerPrefix(TokFalse, p.parseBoolean)
	p.registerPrefix(TokNot, p.parsePrefixExpression)
	p.registerPrefix(TokLParen, p.parseGroupedExpression)
	p.registerPrefix(TokPrincipal, p.parseVariable)
	p.registerPrefix(TokResource, p.parseVariable)
	p.registerPrefix(TokIdent, p.parseVariable) // Để xử lý context (bắt đầu bằng IDENT)

	p.infixParseFns = make(map[TokenType]infixParseFn)
	p.registerInfix(TokOr, p.parseInfixExpression)
	p.registerInfix(TokAnd, p.parseInfixExpression)
	p.registerInfix(TokEq, p.parseInfixExpression)
	p.registerInfix(TokNe, p.parseInfixExpression)
	p.registerInfix(TokGt, p.parseInfixExpression)
	p.registerInfix(TokLt, p.parseInfixExpression)
	p.registerInfix(TokGe, p.parseInfixExpression)
	p.registerInfix(TokLe, p.parseInfixExpression)
	p.registerInfix(TokIn, p.parseInfixExpression)
	p.registerInfix(TokContains, p.parseInfixExpression)

	// Đọc 2 token để khởi tạo curToken và peekToken
	p.nextToken()
	p.nextToken()

	return p
}

// Errors trả về danh sách các lỗi cú pháp.
func (p *Parser) Errors() []string {
	return p.errors
}

// addError ghi nhận lỗi cú pháp kèm thông tin dòng và cột.
func (p *Parser) addError(pos Position, msg string) {
	p.errors = append(p.errors, fmt.Sprintf("Lỗi cú pháp tại dòng %d, cột %d: %s", pos.Line, pos.Column, msg))
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t TokenType) bool {
	return p.peekToken.Type == t
}

// expectPeek kiểm tra token tiếp theo có đúng kiểu mong đợi không.
// Nếu đúng, parser sẽ di chuyển lên 1 bước và trả về true.
// Nếu sai, parser ghi nhận lỗi và trả về false.
func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.addError(p.peekToken.Pos, fmt.Sprintf("Mong đợi token kiểu '%s', nhưng nhận được '%s' (literal: '%s')", t, p.peekToken.Type, p.peekToken.Literal))
	return false
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

// Parse thực hiện phân tích toàn bộ chuỗi nguồn và trả về danh sách các chính sách (PolicyNode).
func (p *Parser) Parse() []*PolicyNode {
	policies := []*PolicyNode{}

	for !p.curTokenIs(TokEOF) {
		policy := p.parsePolicy()
		if policy != nil {
			policies = append(policies, policy)
		}
		p.nextToken()
	}

	return policies
}

// parsePolicy phân tích một câu chính sách hoàn chỉnh.
// Định dạng: Effect "(" Scope ")" [ ConditionClause ] ";"
func (p *Parser) parsePolicy() *PolicyNode {
	policy := &PolicyNode{
		pos: p.curToken.Pos,
	}

	// 1. Phân tích Effect (permit hoặc forbid)
	if p.curTokenIs(TokPermit) {
		policy.Effect = EffectPermit
	} else if p.curTokenIs(TokForbid) {
		policy.Effect = EffectForbid
	} else {
		p.addError(p.curToken.Pos, fmt.Sprintf("Mỗi câu chính sách phải bắt đầu bằng 'permit' hoặc 'forbid', nhận được: '%s'", p.curToken.Literal))
		return nil
	}

	// 2. Phân tích dấu ngoặc đơn mở "("
	if !p.expectPeek(TokLParen) {
		return nil
	}

	// 3. Phân tích Scope tĩnh (principal, action, resource)
	p.nextToken()
	policy.Principal = p.parseScope(TokPrincipal)
	if policy.Principal == nil {
		return nil
	}

	if !p.expectPeek(TokComma) {
		return nil
	}

	p.nextToken()
	policy.Action = p.parseScope(TokAction)
	if policy.Action == nil {
		return nil
	}

	if !p.expectPeek(TokComma) {
		return nil
	}

	p.nextToken()
	policy.Resource = p.parseScope(TokResource)
	if policy.Resource == nil {
		return nil
	}

	// 4. Phân tích dấu ngoặc đơn đóng ")"
	if !p.expectPeek(TokRParen) {
		return nil
	}

	// 5. Phân tích mệnh đề điều kiện tùy chọn (when hoặc unless)
	if p.peekTokenIs(TokWhen) || p.peekTokenIs(TokUnless) {
		p.nextToken()
		policy.IsUnless = p.curTokenIs(TokUnless)

		if !p.expectPeek(TokLBrace) {
			return nil
		}

		p.nextToken() // Bỏ qua '{' và đi vào trong biểu thức logic

		policy.Condition = p.parseExpression(LOWEST)
		if policy.Condition == nil {
			return nil
		}

		if !p.expectPeek(TokRBrace) {
			return nil
		}
	}

	// 6. Phân tích dấu kết thúc câu ";"
	if !p.expectPeek(TokSemicolon) {
		return nil
	}

	return policy
}

// parseScope phân tích một điều khoản Scope tĩnh.
// Ví dụ: principal == user:"alice"
// Ví dụ: resource == any
func (p *Parser) parseScope(scopeKeyword TokenType) *ScopeNode {
	scope := &ScopeNode{
		pos: p.curToken.Pos,
	}

	// Xác nhận từ khóa Scope đúng (principal, action, hoặc resource)
	if !p.curTokenIs(scopeKeyword) {
		p.addError(p.curToken.Pos, fmt.Sprintf("Mong đợi từ khóa Scope là '%s', nhưng nhận được '%s'", scopeKeyword, p.curToken.Literal))
		return nil
	}

	// Phân tích toán tử (== hoặc in)
	if p.peekTokenIs(TokEq) {
		scope.Operator = ScopeOpEq
	} else if p.peekTokenIs(TokIn) {
		scope.Operator = ScopeOpIn
	} else {
		p.addError(p.peekToken.Pos, fmt.Sprintf("Toán tử Scope không hợp lệ: '%s'. Chỉ hỗ trợ '==' hoặc 'in'", p.peekToken.Literal))
		return nil
	}
	p.nextToken() // chuyển sang toán tử

	// Phân tích giá trị so khớp Scope
	p.nextToken() // chuyển sang giá trị
	if p.curTokenIs(TokAny) {
		scope.Operator = ScopeOpAny
	} else if p.curTokenIs(TokIdent) || p.curTokenIs(TokPrincipal) || p.curTokenIs(TokAction) || p.curTokenIs(TokResource) {
		// Dạng Type:ID, ví dụ: user:"alice"
		scope.EntityType = p.curToken.Literal

		if !p.expectPeek(TokColon) {
			return nil
		}

		// ID có thể là String hoặc Ident (ví dụ: role:admin hoặc role:"admin")
		if !p.peekTokenIs(TokString) && !p.peekTokenIs(TokIdent) {
			p.addError(p.peekToken.Pos, fmt.Sprintf("Mã định danh thực thể Scope không hợp lệ: '%s'. Phải là String hoặc Identifier", p.peekToken.Literal))
			return nil
		}
		p.nextToken()
		scope.EntityID = p.curToken.Literal
	} else {
		p.addError(p.curToken.Pos, fmt.Sprintf("Giá trị Scope không hợp lệ: '%s'. Phải là 'any' hoặc định dạng 'Type:ID'", p.curToken.Literal))
		return nil
	}

	return scope
}

// parseExpression phân tích biểu thức logic trong condition sử dụng Pratt Parser.
func (p *Parser) parseExpression(precedence int) Node {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.addError(p.curToken.Pos, fmt.Sprintf("Không tìm thấy hàm phân tích tiền tố cho token: '%s' (literal: '%s')", p.curToken.Type, p.curToken.Literal))
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(TokRBrace) && !p.peekTokenIs(TokEOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

// prefix parse functions

func (p *Parser) parseIdentifier() Node {
	// Mặc định, một Identifier đơn độc trong condition được coi là lỗi, trừ khi nó là một phần của context
	// Ví dụ: context.ip_address. Ở đây, ta đã đăng ký parseVariable cho TokIdent và TokContext.
	// Nếu nó lọt vào đây, nó sẽ được hiểu là lỗi cú pháp hoặc một định danh chưa được xử lý.
	p.addError(p.curToken.Pos, fmt.Sprintf("Định danh đơn độc không hợp lệ trong mệnh đề điều kiện: '%s'", p.curToken.Literal))
	return nil
}

func (p *Parser) parseNumber() Node {
	val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.addError(p.curToken.Pos, fmt.Sprintf("Không thể chuyển đổi '%s' thành số nguyên 64-bit", p.curToken.Literal))
		return nil
	}
	return &ValueNode{
		ValType: ValueTypeInt,
		IntVal:  val,
		pos:     p.curToken.Pos,
	}
}

func (p *Parser) parseString() Node {
	return &ValueNode{
		ValType: ValueTypeString,
		StrVal:  p.curToken.Literal,
		pos:     p.curToken.Pos,
	}
}

func (p *Parser) parseIP() Node {
	// IPAddressLiteral có thể chứa hoặc không chứa CIDR mask
	ipStr := p.curToken.Literal
	if ip := net.ParseIP(ipStr); ip != nil {
		return &ValueNode{
			ValType: ValueTypeIP,
			IPVal:   ip,
			pos:     p.curToken.Pos,
		}
	}

	// Thử parse dạng CIDR
	if _, ipnet, err := net.ParseCIDR(ipStr); err == nil && ipnet != nil {
		return &ValueNode{
			ValType:  ValueTypeIPNet,
			IPNetVal: *ipnet,
			pos:      p.curToken.Pos,
		}
	}

	p.addError(p.curToken.Pos, fmt.Sprintf("Định dạng IP Address không hợp lệ: '%s'", p.curToken.Literal))
	return nil
}

func (p *Parser) parseBoolean() Node {
	return &ValueNode{
		ValType: ValueTypeBool,
		BoolVal: p.curTokenIs(TokTrue),
		pos:     p.curToken.Pos,
	}
}

func (p *Parser) parsePrefixExpression() Node {
	expr := &UnaryExprNode{
		pos: p.curToken.Pos,
	}

	if p.curTokenIs(TokNot) {
		expr.Op = UnaryOpNot
	} else {
		p.addError(p.curToken.Pos, fmt.Sprintf("Toán tử một ngôi không được hỗ trợ: '%s'", p.curToken.Literal))
		return nil
	}

	p.nextToken()
	expr.Child = p.parseExpression(PREFIX)

	return expr
}

func (p *Parser) parseGroupedExpression() Node {
	p.nextToken() // Bỏ qua '('

	expr := p.parseExpression(LOWEST)

	if !p.expectPeek(TokRParen) {
		return nil
	}

	return expr
}

// parseVariable phân tích các biến: principal.attr, resource.attr, context.attr
func (p *Parser) parseVariable() Node {
	expr := &VariableNode{
		pos: p.curToken.Pos,
	}

	// Phân loại Scope của biến
	switch p.curToken.Literal {
	case "principal":
		expr.Scope = VarScopePrincipal
	case "resource":
		expr.Scope = VarScopeResource
	case "context":
		expr.Scope = VarScopeContext
	default:
		p.addError(p.curToken.Pos, fmt.Sprintf("Không gian tên biến không hợp lệ: '%s'. Chỉ hỗ trợ 'principal', 'resource', hoặc 'context'", p.curToken.Literal))
		return nil
	}

	// Đảm bảo theo sau bởi dấu chấm "."
	if !p.expectPeek(TokDot) {
		return nil
	}

	// Tiếp theo bắt buộc phải là tên thuộc tính (Identifier)
	if !p.expectPeek(TokIdent) {
		return nil
	}

	expr.Field = p.curToken.Literal
	return expr
}

// infix parse functions

func (p *Parser) parseInfixExpression(left Node) Node {
	expr := &BinaryExprNode{
		pos:  p.curToken.Pos,
		Left: left,
	}

	switch p.curToken.Type {
	case TokOr:
		expr.Op = BinOpOr
	case TokAnd:
		expr.Op = BinOpAnd
	case TokEq:
		expr.Op = BinOpEq
	case TokNe:
		expr.Op = BinOpNe
	case TokGt:
		expr.Op = BinOpGt
	case TokLt:
		expr.Op = BinOpLt
	case TokGe:
		expr.Op = BinOpGe
	case TokLe:
		expr.Op = BinOpLe
	case TokIn:
		expr.Op = BinOpIn
	case TokContains:
		expr.Op = BinOpContains
	default:
		p.addError(p.curToken.Pos, fmt.Sprintf("Toán tử hai ngôi không được hỗ trợ: '%s'", p.curToken.Literal))
		return nil
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(precedence)

	return expr
}

func (p *Parser) registerPrefix(tokenType TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
