package parser

import (
	"unicode"
)

// TokenType đại diện cho kiểu của một token.
type TokenType string

const (
	TokEOF     TokenType = "EOF"
	TokIllegal TokenType = "ILLEGAL"

	// Identifiers & Literals
	TokIdent  TokenType = "IDENT"
	TokInt    TokenType = "INT"
	TokString TokenType = "STRING"
	TokIP     TokenType = "IP" // IPAddressLiteral trần (ví dụ: 192.168.1.1 hoặc 10.0.0.0/8)

	// Keywords
	TokPermit    TokenType = "permit"
	TokForbid    TokenType = "forbid"
	TokPrincipal TokenType = "principal"
	TokAction    TokenType = "action"
	TokResource  TokenType = "resource"
	TokAny       TokenType = "any"
	TokWhen      TokenType = "when"
	TokUnless    TokenType = "unless"
	TokIn        TokenType = "in"
	TokContains  TokenType = "contains"
	TokTrue      TokenType = "true"
	TokFalse     TokenType = "false"

	// Operators
	TokEq  TokenType = "=="
	TokNe  TokenType = "!="
	TokGt  TokenType = ">"
	TokLt  TokenType = "<"
	TokGe  TokenType = ">="
	TokLe  TokenType = "<="
	TokAnd TokenType = "&&"
	TokOr  TokenType = "||"
	TokNot TokenType = "!"
	TokDot TokenType = "."

	// Delimiters
	TokComma     TokenType = ","
	TokColon     TokenType = ":"
	TokSemicolon TokenType = ";"
	TokLParen    TokenType = "("
	TokRParen    TokenType = ")"
	TokLBrace    TokenType = "{"
	TokRBrace    TokenType = "}"
)

// Token đại diện cho một đơn vị từ vựng được quét bởi Lexer.
type Token struct {
	Type    TokenType
	Literal string
	Pos     Position
}

// Lexer thực hiện quét chuỗi nguồn đầu vào và trả về luồng các token.
type Lexer struct {
	input        []rune
	position     int      // Vị trí hiện tại trong input (chỉ tới ký tự ch)
	readPosition int      // Vị trí đọc tiếp theo (sau ch)
	ch           rune     // Ký tự hiện tại đang xét
	line         int      // Dòng hiện tại (1-indexed)
	col          int      // Cột hiện tại (1-indexed)
	inCondition  bool     // Trạng thái Stateful Lexer: true nếu nằm trong mệnh đề { ... }
	braceCount   int      // Đếm số lượng dấu ngoặc nhọn để biết khi nào thoát khỏi condition
}

// NewLexer tạo mới một instance Lexer.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input: []rune(input),
		line:  1,
		col:   0,
	}
	l.readChar()
	return l
}

// readChar đọc ký tự tiếp theo từ input và cập nhật con trỏ vị trí.
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}

	l.position = l.readPosition
	l.readPosition++

	// Cập nhật dòng và cột
	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

// peekChar xem trước ký tự tiếp theo mà không di chuyển con trỏ.
func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken trả về token tiếp theo được nhận diện từ luồng đầu vào.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	var tok Token
	tok.Pos = Position{
		Line:   l.line,
		Column: l.col,
		Offset: l.position,
	}

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = TokEq
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokIllegal
			tok.Literal = string(l.ch)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = TokNe
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokNot
			tok.Literal = string(l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = TokGe
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokGt
			tok.Literal = string(l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok.Type = TokLe
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokLt
			tok.Literal = string(l.ch)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok.Type = TokAnd
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokIllegal
			tok.Literal = string(l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok.Type = TokOr
			tok.Literal = string(ch) + string(l.ch)
		} else {
			tok.Type = TokIllegal
			tok.Literal = string(l.ch)
		}
	case '.':
		tok.Type = TokDot
		tok.Literal = string(l.ch)
	case ',':
		tok.Type = TokComma
		tok.Literal = string(l.ch)
	case ':':
		tok.Type = TokColon
		tok.Literal = string(l.ch)
	case ';':
		tok.Type = TokSemicolon
		tok.Literal = string(l.ch)
	case '(':
		tok.Type = TokLParen
		tok.Literal = string(l.ch)
	case ')':
		tok.Type = TokRParen
		tok.Literal = string(l.ch)
	case '{':
		tok.Type = TokLBrace
		tok.Literal = string(l.ch)
		l.braceCount++
		l.inCondition = true // Khi gặp {, bắt đầu chế độ Stateful "In condition"
	case '}':
		tok.Type = TokRBrace
		tok.Literal = string(l.ch)
		l.braceCount--
		if l.braceCount <= 0 {
			l.braceCount = 0
			l.inCondition = false // Thoát khỏi condition
		}
	case '"':
		tok.Type = TokString
		tok.Literal = l.readString()
		return tok
	case 0:
		tok.Type = TokEOF
		tok.Literal = ""
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			// Nhận diện số nguyên hoặc IP Address trần
			tok.Literal, tok.Type = l.readNumberOrIP()
			return tok
		} else {
			tok.Type = TokIllegal
			tok.Literal = string(l.ch)
		}
	}

	l.readChar()
	return tok
}

// skipWhitespace bỏ qua các khoảng trắng, dấu tab, dấu xuống dòng.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// isLetter kiểm tra ký tự có thể là ký tự cấu thành một định danh.
func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

// isDigit kiểm tra ký tự là chữ số.
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// readIdentifier đọc một từ khóa hoặc tên biến.
func (l *Lexer) readIdentifier() string {
	start := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return string(l.input[start:l.position])
}

// lookupIdent ánh xạ một định danh với từ khóa tương ứng.
func lookupIdent(ident string) TokenType {
	switch ident {
	case "permit":
		return TokPermit
	case "forbid":
		return TokForbid
	case "principal":
		return TokPrincipal
	case "action":
		return TokAction
	case "resource":
		return TokResource
	case "any":
		return TokAny
	case "when":
		return TokWhen
	case "unless":
		return TokUnless
	case "in":
		return TokIn
	case "contains":
		return TokContains
	case "true":
		return TokTrue
	case "false":
		return TokFalse
	default:
		return TokIdent
	}
}

// readString đọc nội dung chuỗi nằm trong dấu nháy kép "".
func (l *Lexer) readString() string {
	l.readChar() // bỏ qua dấu nháy kép mở đầu
	start := l.position
	for l.ch != '"' && l.ch != 0 {
		l.readChar()
	}
	str := string(l.input[start:l.position])
	l.readChar() // bỏ qua dấu nháy kép đóng
	return str
}

// readNumberOrIP quét một chuỗi các ký số.
func (l *Lexer) readNumberOrIP() (string, TokenType) {
	start := l.position
	
	// Đọc chuỗi chữ số đầu tiên
	for isDigit(l.ch) {
		l.readChar()
	}

	// Nếu ký tự tiếp theo là '.', đây có thể là địa chỉ IP Address
	if l.ch == '.' {
		// Đọc tiếp các phần còn lại của IP: .Digit.Digit.Digit
		for i := 0; i < 3; i++ {
			if l.ch != '.' {
				return string(l.input[start:l.position]), TokIllegal
			}
			l.readChar() // bỏ qua dấu chấm
			if !isDigit(l.ch) {
				return string(l.input[start:l.position]), TokIllegal
			}
			for isDigit(l.ch) {
				l.readChar()
			}
		}
		
		// Hỗ trợ dải mạng CIDR (ví dụ: /24)
		if l.ch == '/' {
			l.readChar() // bỏ qua dấu /
			if !isDigit(l.ch) {
				return string(l.input[start:l.position]), TokIllegal
			}
			for isDigit(l.ch) {
				l.readChar()
			}
		}
		
		return string(l.input[start:l.position]), TokIP
	}

	// Nếu không có '.', đó chỉ là số nguyên thông thường
	return string(l.input[start:l.position]), TokInt
}
