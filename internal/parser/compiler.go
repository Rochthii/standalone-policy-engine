package parser

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// Compiler chịu trách nhiệm kiểm tra ngữ nghĩa, bảo mật và tối ưu hóa cây AST.
type Compiler struct {
	errors []string
}

// NewCompiler tạo mới một instance Compiler.
func NewCompiler() *Compiler {
	return &Compiler{
		errors: []string{},
	}
}

// Errors trả về danh sách các lỗi trong quá trình biên dịch.
func (c *Compiler) Errors() []string {
	return c.errors
}

func (c *Compiler) addError(pos Position, msg string) {
	c.errors = append(c.errors, fmt.Sprintf("Lỗi biên dịch tại dòng %d, cột %d: %s", pos.Line, pos.Column, msg))
}

// Compile thực hiện toàn bộ quy trình biên dịch và tối ưu hóa cho một chính sách.
func (c *Compiler) Compile(policy *PolicyNode) (*PolicyNode, error) {
	if policy == nil {
		return nil, fmt.Errorf("policy node là nil")
	}

	// 1. Kiểm tra độ sâu tối đa của AST (giới hạn 15 cấp)
	if policy.Condition != nil {
		if err := c.checkDepth(policy.Condition, 1); err != nil {
			return nil, err
		}

		// 2. Tiền xử lý (Pre-parse) các chuỗi IP, DateTime và thực hiện Constant Folding
		policy.Condition = c.optimizeNode(policy.Condition)
	}

	if len(c.errors) > 0 {
		return nil, fmt.Errorf("phát hiện lỗi trong quá trình biên dịch AST")
	}

	return policy, nil
}

// checkDepth kiểm tra đệ quy độ sâu của AST để phòng chống lỗi Stack Overflow.
func (c *Compiler) checkDepth(node Node, currentDepth int) error {
	if currentDepth > 15 {
		return fmt.Errorf("độ sâu của biểu thức logic vượt quá giới hạn an toàn là 15 cấp (ngăn chặn tấn công DoS)")
	}

	switch n := node.(type) {
	case *BinaryExprNode:
		if err := c.checkDepth(n.Left, currentDepth+1); err != nil {
			return err
		}
		if err := c.checkDepth(n.Right, currentDepth+1); err != nil {
			return err
		}
	case *UnaryExprNode:
		if err := c.checkDepth(n.Child, currentDepth+1); err != nil {
			return err
		}
	}

	return nil
}

// optimizeNode thực hiện tối ưu hóa, pre-parsing và constant folding trên một Node.
func (c *Compiler) optimizeNode(node Node) Node {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *BinaryExprNode:
		n.Left = c.optimizeNode(n.Left)
		n.Right = c.optimizeNode(n.Right)

		// Type checking tĩnh cho toán tử so sánh lớn hơn/nhỏ hơn
		if isRelationalOp(n.Op) {
			c.checkTypeCompatibility(n)
		}

		// Pre-parse IP/DateTime nếu có dạng StringLiteral so khớp với biến IP/DateTime
		c.preparseIdentifiersAndLiterals(n)

		// Constant Folding (Thu gọn hằng số)
		return c.foldBinaryExpr(n)

	case *UnaryExprNode:
		n.Child = c.optimizeNode(n.Child)
		return c.foldUnaryExpr(n)

	case *ValueNode:
		// Thử chủ động chuyển đổi StringLiteral thành IP hoặc DateTime nếu có cấu trúc phù hợp
		if n.ValType == ValueTypeString {
			if ipVal, ipnetVal, ok := tryParseIPOrCIDR(n.StrVal); ok {
				if ipnetVal != nil {
					n.ValType = ValueTypeIPNet
					n.IPNetVal = *ipnetVal
				} else {
					n.ValType = ValueTypeIP
					n.IPVal = ipVal
				}
			} else if dtVal, ok := tryParseDateTimeOrTime(n.StrVal); ok {
				n.ValType = ValueTypeDateTime
				n.IntVal = dtVal
			}
		}
		return n
	}

	return node
}

func isRelationalOp(op BinaryOp) bool {
	return op == BinOpGt || op == BinOpLt || op == BinOpGe || op == BinOpLe
}

// checkTypeCompatibility thực hiện kiểm tra kiểu tĩnh cơ bản.
func (c *Compiler) checkTypeCompatibility(n *BinaryExprNode) {
	leftVal, leftOk := n.Left.(*ValueNode)
	rightVal, rightOk := n.Right.(*ValueNode)

	// Nếu cả 2 vế đều là hằng số literal
	if leftOk && rightOk {
		if leftVal.ValType != rightVal.ValType {
			c.addError(n.pos, fmt.Sprintf("Không thể so sánh hai kiểu dữ liệu khác nhau (kiểu %d và %d)", leftVal.ValType, rightVal.ValType))
		}
	}
}

// preparseIdentifiersAndLiterals thực hiện phân tích trước các kiểu IP/DateTime
// dựa trên ngữ cảnh toán tử so khớp.
func (c *Compiler) preparseIdentifiersAndLiterals(n *BinaryExprNode) {
	leftVar, leftIsVar := n.Left.(*VariableNode)
	rightVal, rightIsVal := n.Right.(*ValueNode)

	// Hỗ trợ trường hợp: context.ip_address in "192.168.1.0/24"
	if leftIsVar && rightIsVal && rightVal.ValType == ValueTypeString {
		// Dựa vào tên thuộc tính để suy đoán kiểu (hoặc thử parse trực tiếp)
		if n.Op == BinOpIn || n.Op == BinOpEq {
			if strings.Contains(leftVar.Field, "ip") || strings.Contains(leftVar.Field, "addr") {
				if ip, ipnet, ok := tryParseIPOrCIDR(rightVal.StrVal); ok {
					if ipnet != nil {
						rightVal.ValType = ValueTypeIPNet
						rightVal.IPNetVal = *ipnet
					} else {
						rightVal.ValType = ValueTypeIP
						rightVal.IPVal = ip
					}
				}
			}
		}

		// Dựa vào tên thuộc tính thời gian để suy đoán kiểu
		if strings.Contains(leftVar.Field, "time") || strings.Contains(leftVar.Field, "date") {
			if dt, ok := tryParseDateTimeOrTime(rightVal.StrVal); ok {
				rightVal.ValType = ValueTypeDateTime
				rightVal.IntVal = dt
			}
		}
	}
}

// foldBinaryExpr thực hiện thu gọn hằng số cho toán tử hai ngôi.
func (c *Compiler) foldBinaryExpr(n *BinaryExprNode) Node {
	leftVal, leftOk := n.Left.(*ValueNode)
	rightVal, rightOk := n.Right.(*ValueNode)

	// Đoản mạch logic tĩnh (Short-circuit constant folding)
	if n.Op == BinOpAnd {
		if leftOk && leftVal.ValType == ValueTypeBool && !leftVal.BoolVal {
			return &ValueNode{ValType: ValueTypeBool, BoolVal: false, pos: n.pos}
		}
		if rightOk && rightVal.ValType == ValueTypeBool && !rightVal.BoolVal {
			return &ValueNode{ValType: ValueTypeBool, BoolVal: false, pos: n.pos}
		}
	}
	if n.Op == BinOpOr {
		if leftOk && leftVal.ValType == ValueTypeBool && leftVal.BoolVal {
			return &ValueNode{ValType: ValueTypeBool, BoolVal: true, pos: n.pos}
		}
		if rightOk && rightVal.ValType == ValueTypeBool && rightVal.BoolVal {
			return &ValueNode{ValType: ValueTypeBool, BoolVal: true, pos: n.pos}
		}
	}

	// Nếu cả hai vế đều là hằng số literal tĩnh, ta tính toán kết quả luôn
	if leftOk && rightOk {
		switch n.Op {
		case BinOpAnd:
			if leftVal.ValType == ValueTypeBool && rightVal.ValType == ValueTypeBool {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: leftVal.BoolVal && rightVal.BoolVal, pos: n.pos}
			}
		case BinOpOr:
			if leftVal.ValType == ValueTypeBool && rightVal.ValType == ValueTypeBool {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: leftVal.BoolVal || rightVal.BoolVal, pos: n.pos}
			}
		case BinOpEq:
			return &ValueNode{ValType: ValueTypeBool, BoolVal: evaluateEq(leftVal, rightVal), pos: n.pos}
		case BinOpNe:
			return &ValueNode{ValType: ValueTypeBool, BoolVal: !evaluateEq(leftVal, rightVal), pos: n.pos}
		case BinOpGt:
			val, ok := evaluateCompare(leftVal, rightVal)
			if ok {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: val > 0, pos: n.pos}
			}
		case BinOpLt:
			val, ok := evaluateCompare(leftVal, rightVal)
			if ok {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: val < 0, pos: n.pos}
			}
		case BinOpGe:
			val, ok := evaluateCompare(leftVal, rightVal)
			if ok {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: val >= 0, pos: n.pos}
			}
		case BinOpLe:
			val, ok := evaluateCompare(leftVal, rightVal)
			if ok {
				return &ValueNode{ValType: ValueTypeBool, BoolVal: val <= 0, pos: n.pos}
			}
		}
	}

	return n
}

// foldUnaryExpr thực hiện thu gọn hằng số cho toán tử một ngôi.
func (c *Compiler) foldUnaryExpr(n *UnaryExprNode) Node {
	childVal, ok := n.Child.(*ValueNode)
	if !ok {
		return n
	}

	if n.Op == UnaryOpNot && childVal.ValType == ValueTypeBool {
		return &ValueNode{
			ValType: ValueTypeBool,
			BoolVal: !childVal.BoolVal,
			pos:     n.pos,
		}
	}

	if n.Op == UnaryOpNeg && childVal.ValType == ValueTypeInt {
		return &ValueNode{
			ValType: ValueTypeInt,
			IntVal:  -childVal.IntVal,
			pos:     n.pos,
		}
	}

	return n
}

func evaluateEq(l, r *ValueNode) bool {
	if l.ValType != r.ValType {
		return false
	}
	switch l.ValType {
	case ValueTypeBool:
		return l.BoolVal == r.BoolVal
	case ValueTypeInt:
		return l.IntVal == r.IntVal
	case ValueTypeString:
		return l.StrVal == r.StrVal
	case ValueTypeIP:
		return l.IPVal.Equal(r.IPVal)
	}
	return false
}

func evaluateCompare(l, r *ValueNode) (int, bool) {
	if l.ValType != r.ValType {
		return 0, false
	}
	switch l.ValType {
	case ValueTypeInt, ValueTypeDateTime:
		if l.IntVal > r.IntVal {
			return 1, true
		} else if l.IntVal < r.IntVal {
			return -1, true
		}
		return 0, true
	case ValueTypeString:
		if l.StrVal > r.StrVal {
			return 1, true
		} else if l.StrVal < r.StrVal {
			return -1, true
		}
		return 0, true
	}
	return 0, false
}

// helper functions for parsing

func tryParseIPOrCIDR(s string) (net.IP, *net.IPNet, bool) {
	if ip := net.ParseIP(s); ip != nil {
		return ip, nil, true
	}
	if ip, ipnet, err := net.ParseCIDR(s); err == nil && ipnet != nil {
		return ip, ipnet, true
	}
	return nil, nil, false
}

func tryParseDateTimeOrTime(s string) (int64, bool) {
	// 1. Thử parse định dạng RFC3339 hoàn chỉnh
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixNano(), true
	}

	// 2. Thử parse định dạng thời gian đơn thuần: 15:04:05 hoặc 15:04:05Z
	// Ta sẽ quy đổi thành số giây tính từ đầu ngày để so sánh tương đối
	var hour, min, sec int
	var err error
	if strings.HasSuffix(s, "Z") {
		_, err = fmt.Sscanf(s, "%d:%d:%dZ", &hour, &min, &sec)
	} else {
		_, err = fmt.Sscanf(s, "%d:%d:%d", &hour, &min, &sec)
	}

	if err == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 && sec >= 0 && sec < 60 {
		totalSeconds := int64(hour*3600 + min*60 + sec)
		return totalSeconds, true
	}

	// 3. Thử parse định dạng rút gọn 15:04
	if _, err = fmt.Sscanf(s, "%d:%d", &hour, &min); err == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
		totalSeconds := int64(hour*3600 + min*60)
		return totalSeconds, true
	}

	return 0, false
}
