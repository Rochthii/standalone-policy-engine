package engine

import (
	"errors"
	"fmt"
	"net"
	"standalone-policy-engine/internal/parser"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Các lỗi nghiệp vụ có kiểm soát trong quá trình đánh giá
var (
	ErrMissingAttribute = errors.New("thuộc tính yêu cầu không tồn tại trong ngữ cảnh")
	ErrUnsupportedOp    = errors.New("toán tử không được hỗ trợ cho kiểu dữ liệu này")
	ErrTypeMismatch     = errors.New("kiểu dữ liệu không đồng nhất giữa hai vế")
)

// EvalContext chứa toàn bộ ngữ cảnh cần thiết cho một lượt đánh giá quyết định.
type EvalContext struct {
	Subject  string
	Action   string
	Resource string
	Context  map[string]string

	// RoleDAG dùng để kiểm tra phân cấp vai trò
	RoleDAG *RoleDAG

	// Cache lưu danh sách các vai trò đã kế thừa của Subject để tối ưu hóa tra cứu
	subjectInherited []string
	once             sync.Once
}

// sync.Pool giúp tái sử dụng các đối tượng EvalContext để triệt tiêu allocation.
var contextPool = sync.Pool{
	New: func() interface{} {
		return &EvalContext{
			Context: make(map[string]string),
		}
	},
}

// GetEvalContext lấy một EvalContext từ pool và thiết lập các giá trị.
func GetEvalContext(subject, action, resource string, context map[string]string, dag *RoleDAG) *EvalContext {
	ctx := contextPool.Get().(*EvalContext)
	ctx.Subject = subject
	ctx.Action = action
	ctx.Resource = resource
	ctx.RoleDAG = dag
	ctx.subjectInherited = nil
	ctx.once = sync.Once{}

	// Sao chép context map
	for k, v := range context {
		ctx.Context[k] = v
	}

	return ctx
}

// Release trả EvalContext về pool sau khi dùng xong.
func (ctx *EvalContext) Release() {
	// Xóa sạch map để tránh rò rỉ bộ nhớ
	for k := range ctx.Context {
		delete(ctx.Context, k)
	}
	ctx.RoleDAG = nil
	ctx.subjectInherited = nil
	contextPool.Put(ctx)
}

// getInheritedRoles thu thập các vai trò kế thừa của Subject (kèm cache).
func (ctx *EvalContext) getInheritedRoles() []string {
	ctx.once.Do(func() {
		if ctx.RoleDAG != nil {
			ctx.subjectInherited = ctx.RoleDAG.GetInheritedRoles(ctx.Subject)
		} else {
			ctx.subjectInherited = []string{ctx.Subject}
		}
	})
	return ctx.subjectInherited
}

// GetAttribute truy xuất thuộc tính động từ ngữ cảnh và parse sang kiểu mong đợi.
func (ctx *EvalContext) GetAttribute(scope parser.VarScope, field string, expectedType parser.ValueType) (*parser.ValueNode, error) {
	switch scope {
	case parser.VarScopePrincipal:
		if field == "id" {
			return &parser.ValueNode{ValType: parser.ValueTypeString, StrVal: ctx.Subject}, nil
		}
		// Thử tìm trong Context map với tiền tố principal.
		val, exists := ctx.Context["principal."+field]
		if !exists {
			// Fallback tìm trực tiếp
			val, exists = ctx.Context[field]
		}
		if exists {
			return parseStringValue(val, expectedType)
		}

	case parser.VarScopeResource:
		if field == "id" {
			return &parser.ValueNode{ValType: parser.ValueTypeString, StrVal: ctx.Resource}, nil
		}
		// Thử tìm trong Context map với tiền tố resource.
		val, exists := ctx.Context["resource."+field]
		if !exists {
			val, exists = ctx.Context[field]
		}
		if exists {
			return parseStringValue(val, expectedType)
		}

	case parser.VarScopeContext:
		val, exists := ctx.Context[field]
		if exists {
			return parseStringValue(val, expectedType)
		}
	}

	return nil, ErrMissingAttribute
}

func parseStringValue(val string, expectedType parser.ValueType) (*parser.ValueNode, error) {
	node := &parser.ValueNode{ValType: expectedType}
	switch expectedType {
	case parser.ValueTypeString:
		node.StrVal = val
		return node, nil
	case parser.ValueTypeInt:
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}
		node.IntVal = intVal
		return node, nil
	case parser.ValueTypeBool:
		boolVal, err := strconv.ParseBool(val)
		if err != nil {
			return nil, err
		}
		node.BoolVal = boolVal
		return node, nil
	case parser.ValueTypeIP:
		ip := net.ParseIP(val)
		if ip == nil {
			return nil, errors.New("định dạng IP không hợp lệ")
		}
		node.IPVal = ip
		return node, nil
	case parser.ValueTypeIPNet:
		_, ipnet, err := net.ParseCIDR(val)
		if err != nil {
			return nil, err
		}
		node.IPNetVal = *ipnet
		return node, nil
	case parser.ValueTypeDateTime:
		// Hỗ trợ DateTime hoặc Time
		dtVal, ok := tryParseDateTimeOrTime(val)
		if !ok {
			return nil, errors.New("định dạng DateTime không hợp lệ")
		}
		node.IntVal = dtVal
		return node, nil
	}
	return nil, errors.New("kiểu dữ liệu mong đợi không xác định")
}

// Evaluate thực hiện duyệt đệ quy cây AST và đánh giá kết quả của biểu thức.
func Evaluate(node parser.Node, ctx *EvalContext) (*parser.ValueNode, error) {
	if node == nil {
		return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: true}, nil
	}

	switch n := node.(type) {
	case *parser.ValueNode:
		return n, nil

	case *parser.VariableNode:
		// Trình biên dịch (Compiler) đã pre-parse kiểu dữ liệu tĩnh trong AST ở mức tối đa.
		// Tuy nhiên, ở đây ta chưa biết kiểu mong đợi trực tiếp của VariableNode đơn lẻ.
		// Ta mặc định trả về String, và việc parse sang kiểu tương ứng sẽ do hàm BinaryExpr điều phối.
		val, err := ctx.GetAttribute(n.Scope, n.Field, parser.ValueTypeString)
		if err != nil {
			return nil, err
		}
		return val, nil

	case *parser.UnaryExprNode:
		childVal, err := Evaluate(n.Child, ctx)
		if err != nil {
			return nil, err
		}

		if n.Op == parser.UnaryOpNot {
			if childVal.ValType != parser.ValueTypeBool {
				return nil, ErrUnsupportedOp
			}
			return &parser.ValueNode{
				ValType: parser.ValueTypeBool,
				BoolVal: !childVal.BoolVal,
			}, nil
		}

		if n.Op == parser.UnaryOpNeg {
			if childVal.ValType != parser.ValueTypeInt {
				return nil, ErrUnsupportedOp
			}
			return &parser.ValueNode{
				ValType: parser.ValueTypeInt,
				IntVal:  -childVal.IntVal,
			}, nil
		}

	case *parser.BinaryExprNode:
		// 1. Xử lý đoản mạch logic (Short-Circuit Evaluation)
		if n.Op == parser.BinOpAnd {
			leftVal, err := Evaluate(n.Left, ctx)
			if err != nil {
				// Lỗi (ví dụ thiếu thuộc tính) trả về false
				return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
			}
			if leftVal.ValType != parser.ValueTypeBool {
				return nil, ErrUnsupportedOp
			}
			if !leftVal.BoolVal {
				return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
			}

			rightVal, err := Evaluate(n.Right, ctx)
			if err != nil {
				return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
			}
			if rightVal.ValType != parser.ValueTypeBool {
				return nil, ErrUnsupportedOp
			}
			return rightVal, nil
		}

		if n.Op == parser.BinOpOr {
			leftVal, err := Evaluate(n.Left, ctx)
			var leftBool bool
			var leftErr error
			if err == nil && leftVal.ValType == parser.ValueTypeBool {
				leftBool = leftVal.BoolVal
			} else {
				leftErr = err
			}

			// Nếu vế trái thành công và là true, đoản mạch ngay
			if leftErr == nil && leftBool {
				return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: true}, nil
			}

			rightVal, err := Evaluate(n.Right, ctx)
			if err != nil {
				// Nếu cả hai vế đều lỗi hoặc vế phải lỗi và vế trái không phải true -> trả về false
				return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
			}
			if rightVal.ValType != parser.ValueTypeBool {
				return nil, ErrUnsupportedOp
			}
			return rightVal, nil
		}

		// 2. Đánh giá các toán tử so sánh thông thường
		return evaluateComparison(n, ctx)
	}

	return nil, errors.New("kiểu node AST không được hỗ trợ")
}

func evaluateComparison(n *parser.BinaryExprNode, ctx *EvalContext) (*parser.ValueNode, error) {
	// Xác định kiểu dữ liệu của vế phải (thường là hằng số literal đã được compiler định kiểu)
	expectedType := parser.ValueTypeString
	if rightVal, ok := n.Right.(*parser.ValueNode); ok {
		expectedType = rightVal.ValType
	}

	if n.Op == parser.BinOpIn && expectedType == parser.ValueTypeIPNet {
		expectedType = parser.ValueTypeIP
	}

	// Đọc vế trái (Variable) với kiểu mong đợi lấy từ vế phải
	var leftVal *parser.ValueNode
	var err error
	if leftVar, ok := n.Left.(*parser.VariableNode); ok {
		leftVal, err = ctx.GetAttribute(leftVar.Scope, leftVar.Field, expectedType)
	} else {
		leftVal, err = Evaluate(n.Left, ctx)
	}

	if err != nil {
		// Thiếu thuộc tính hoặc lỗi parse -> Fail-closed: Trả về false
		return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
	}

	// Đọc vế phải
	rightVal, err := Evaluate(n.Right, ctx)
	if err != nil {
		return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}, nil
	}

	// Thực hiện phép so sánh tương ứng
	result := false
	switch n.Op {
	case parser.BinOpEq:
		result = compareEq(leftVal, rightVal)

	case parser.BinOpNe:
		result = !compareEq(leftVal, rightVal)

	case parser.BinOpGt, parser.BinOpLt, parser.BinOpGe, parser.BinOpLe:
		cmp, ok := compareOrder(leftVal, rightVal)
		if !ok {
			return nil, ErrUnsupportedOp
		}
		switch n.Op {
		case parser.BinOpGt:
			result = cmp > 0
		case parser.BinOpLt:
			result = cmp < 0
		case parser.BinOpGe:
			result = cmp >= 0
		case parser.BinOpLe:
			result = cmp <= 0
		}

	case parser.BinOpIn:
		// Hỗ trợ 2 trường hợp:
		// 1. IP address in CIDR network (ví dụ: context.ip_address in "192.168.1.0/24")
		// 2. Role hierarchy check (ví dụ: principal in role:admin)
		if leftVal.ValType == parser.ValueTypeIP && rightVal.ValType == parser.ValueTypeIPNet {
			result = rightVal.IPNetVal.Contains(leftVal.IPVal)
		} else if leftVal.ValType == parser.ValueTypeString && rightVal.ValType == parser.ValueTypeString {
			// Kiểm tra vai trò thừa kế trong DAG
			// Vế trái: Vai trò của Subject (ví dụ "super_admin")
			// Vế phải: Vai trò yêu cầu (ví dụ "admin" hoặc "role:admin")
			parent := leftVal.StrVal
			child := rightVal.StrVal
			// Chuẩn hóa chuỗi loại bỏ tiền tố role: nếu có
			child = strings.TrimPrefix(child, "role:")
			parent = strings.TrimPrefix(parent, "role:")

			if ctx.RoleDAG != nil {
				result = ctx.RoleDAG.IsDescendant(parent, child)
			} else {
				result = parent == child
			}
		} else {
			return nil, ErrUnsupportedOp
		}

	case parser.BinOpContains:
		// Hỗ trợ kiểm tra phần tử trong mảng (ví dụ: principal.groups contains "finance")
		// Ở đây, vế trái principal.groups được truyền từ context dưới dạng chuỗi phân tách bởi dấu phẩy ","
		if leftVal.ValType == parser.ValueTypeString && rightVal.ValType == parser.ValueTypeString {
			parts := strings.Split(leftVal.StrVal, ",")
			for _, part := range parts {
				if strings.TrimSpace(part) == rightVal.StrVal {
					result = true
					break
				}
			}
		} else {
			return nil, ErrUnsupportedOp
		}
	}

	return &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: result}, nil
}

func compareEq(l, r *parser.ValueNode) bool {
	if l.ValType != r.ValType {
		return false
	}
	switch l.ValType {
	case parser.ValueTypeString:
		return l.StrVal == r.StrVal
	case parser.ValueTypeInt, parser.ValueTypeDateTime:
		return l.IntVal == r.IntVal
	case parser.ValueTypeBool:
		return l.BoolVal == r.BoolVal
	case parser.ValueTypeIP:
		return l.IPVal.Equal(r.IPVal)
	}
	return false
}

func compareOrder(l, r *parser.ValueNode) (int, bool) {
	if l.ValType != r.ValType {
		return 0, false
	}
	switch l.ValType {
	case parser.ValueTypeInt, parser.ValueTypeDateTime:
		if l.IntVal > r.IntVal {
			return 1, true
		} else if l.IntVal < r.IntVal {
			return -1, true
		}
		return 0, true
	case parser.ValueTypeString:
		return strings.Compare(l.StrVal, r.StrVal), true
	}
	return 0, false
}

// copy helper functions from compiler to avoid cross import cycle

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
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixNano(), true
	}

	var hour, min, sec int
	var err error
	if strings.HasSuffix(s, "Z") {
		_, err = fmt.Sscanf(s, "%d:%d:%dZ", &hour, &min, &sec)
	} else {
		_, err = fmt.Sscanf(s, "%d:%d:%d", &hour, &min, &sec)
	}

	if err == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 && sec >= 0 && sec < 60 {
		return int64(hour*3600 + min*60 + sec), true
	}

	if _, err = fmt.Sscanf(s, "%d:%d", &hour, &min); err == nil && hour >= 0 && hour < 24 && min >= 0 && min < 60 {
		return int64(hour*3600 + min*60), true
	}

	return 0, false
}
