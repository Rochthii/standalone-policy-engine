package engine

import (
	"errors"
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

// boolTrue và boolFalse là các sentinel ValueNode toàn cục dùng chung — ZERO ALLOCATION.
// Chúng là read-only: không caller nào được phép modify nội dung của chúng.
var (
	boolTrue  = &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: true}
	boolFalse = &parser.ValueNode{ValType: parser.ValueTypeBool, BoolVal: false}
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

	// scratchNodes lưu trữ sẵn các ValueNode trên RAM để tái sử dụng trong quá trình đánh giá (Zero Allocation)
	scratchNodes [64]parser.ValueNode
	scratchIPs   [64][16]byte
	scratchCount int
}

// allocValueNode mượn một ValueNode tạm thời từ scratch buffer của EvalContext.
func (ctx *EvalContext) allocValueNode() *parser.ValueNode {
	if ctx.scratchCount < len(ctx.scratchNodes) {
		node := &ctx.scratchNodes[ctx.scratchCount]
		ctx.scratchCount++
		*node = parser.ValueNode{}
		return node
	}
	return &parser.ValueNode{}
}

// sync.Pool giúp tái sử dụng các đối tượng EvalContext để triệt tiêu allocation.
var contextPool = sync.Pool{
	New: func() interface{} {
		return &EvalContext{}
	},
}

// GetEvalContext lấy một EvalContext từ pool và thiết lập các giá trị (Zero-Copy Context).
func GetEvalContext(subject, action, resource string, context map[string]string, dag *RoleDAG) *EvalContext {
	ctx := contextPool.Get().(*EvalContext)
	ctx.Subject = subject
	ctx.Action = action
	ctx.Resource = resource
	ctx.RoleDAG = dag
	ctx.subjectInherited = nil
	ctx.once = sync.Once{}
	ctx.scratchCount = 0
	ctx.Context = context // Gán trực tiếp con trỏ map, không sao chép lại từng phần tử

	return ctx
}

// Release trả EvalContext về pool sau khi dùng xong.
func (ctx *EvalContext) Release() {
	ctx.Context = nil
	ctx.RoleDAG = nil
	ctx.subjectInherited = nil
	ctx.scratchCount = 0
	contextPool.Put(ctx)
}

// resolveInheritedRoles trả về danh sách các vai trò kế thừa của Subject hiện tại một cách an toàn.
func (ctx *EvalContext) resolveInheritedRoles() []string {
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
			node := ctx.allocValueNode()
			node.ValType = parser.ValueTypeString
			node.StrVal = ctx.Subject
			return node, nil
		}
		// Thử tìm trong Context map với tiền tố principal.
		if ctx.Context != nil {
			val, exists := ctx.Context["principal."+field]
			if !exists {
				// Fallback tìm trực tiếp
				val, exists = ctx.Context[field]
			}
			if exists {
				return ctx.parseStringValue(val, expectedType)
			}
		}

	case parser.VarScopeResource:
		if field == "id" {
			node := ctx.allocValueNode()
			node.ValType = parser.ValueTypeString
			node.StrVal = ctx.Resource
			return node, nil
		}
		// Thử tìm trong Context map với tiền tố resource.
		if ctx.Context != nil {
			val, exists := ctx.Context["resource."+field]
			if !exists {
				val, exists = ctx.Context[field]
			}
			if exists {
				return ctx.parseStringValue(val, expectedType)
			}
		}

	case parser.VarScopeContext:
		if ctx.Context != nil {
			val, exists := ctx.Context[field]
			if !exists {
				val, exists = ctx.Context["context."+field]
			}
			if exists {
				return ctx.parseStringValue(val, expectedType)
			}
		}
	}

	return nil, ErrMissingAttribute
}

func (ctx *EvalContext) parseStringValue(val string, expectedType parser.ValueType) (*parser.ValueNode, error) {
	node := ctx.allocValueNode()
	node.ValType = expectedType
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
		node.IPVal = parseIPv4FastInto(val, &ctx.scratchIPs[ctx.scratchCount-1])
		if node.IPVal == nil {
			return nil, errors.New("định dạng IP không hợp lệ")
		}
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
		return boolTrue, nil
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
			if childVal.BoolVal {
				return boolFalse, nil
			}
			return boolTrue, nil
		}

		if n.Op == parser.UnaryOpNeg {
			if childVal.ValType != parser.ValueTypeInt {
				return nil, ErrUnsupportedOp
			}
			node := ctx.allocValueNode()
			node.ValType = parser.ValueTypeInt
			node.IntVal = -childVal.IntVal
			return node, nil
		}

	case *parser.BinaryExprNode:
		// 1. Xử lý đoản mạch logic (Short-Circuit Evaluation)
		if n.Op == parser.BinOpAnd {
			leftVal, err := Evaluate(n.Left, ctx)
			if err != nil {
				// Lỗi (ví dụ thiếu thuộc tính) trả về false
				return boolFalse, nil
			}
			if leftVal.ValType != parser.ValueTypeBool {
				return nil, ErrUnsupportedOp
			}
			if !leftVal.BoolVal {
				return boolFalse, nil
			}

			rightVal, err := Evaluate(n.Right, ctx)
			if err != nil {
				return boolFalse, nil
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
				return boolTrue, nil
			}

			rightVal, err := Evaluate(n.Right, ctx)
			if err != nil {
				// Nếu cả hai vế đều lỗi hoặc vế phải lỗi và vế trái không phải true -> trả về false
				return boolFalse, nil
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
		return boolFalse, nil
	}

	// Đọc vế phải
	rightVal, err := Evaluate(n.Right, ctx)
	if err != nil {
		return boolFalse, nil
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

	if result {
		return boolTrue, nil
	}
	return boolFalse, nil
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
	// 1. Thử parse nhanh định dạng Time (HH:MM:SS, HH:MM:SSZ, HH:MM) - Zero Allocation
	if val, ok := parseFastTime(s); ok {
		return val, true
	}

	// 2. Thử parse định dạng RFC3339 DateTime
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixNano(), true
	}

	return 0, false
}

func parseFastTime(s string) (int64, bool) {
	s = strings.TrimSuffix(s, "Z")
	// Định dạng "HH:MM:SS" (8 ký tự)
	if len(s) == 8 && s[2] == ':' && s[5] == ':' {
		h0, h1 := s[0]-'0', s[1]-'0'
		m0, m1 := s[3]-'0', s[4]-'0'
		s0, s1 := s[6]-'0', s[7]-'0'
		if h0 <= 9 && h1 <= 9 && m0 <= 9 && m1 <= 9 && s0 <= 9 && s1 <= 9 {
			h := int64(h0)*10 + int64(h1)
			m := int64(m0)*10 + int64(m1)
			sec := int64(s0)*10 + int64(s1)
			if h < 24 && m < 60 && sec < 60 {
				return h*3600 + m*60 + sec, true
			}
		}
	} else if len(s) == 5 && s[2] == ':' {
		// Định dạng "HH:MM" (5 ký tự)
		h0, h1 := s[0]-'0', s[1]-'0'
		m0, m1 := s[3]-'0', s[4]-'0'
		if h0 <= 9 && h1 <= 9 && m0 <= 9 && m1 <= 9 {
			h := int64(h0)*10 + int64(h1)
			m := int64(m0)*10 + int64(m1)
			if h < 24 && m < 60 {
				return h*3600 + m*60, true
			}
		}
	}
	return 0, false
}

// parseIPv4FastInto phân giải địa chỉ IPv4 trực tiếp vào bộ đệm cố định trên RAM mà không cấp phát heap.
func parseIPv4FastInto(s string, dst *[16]byte) net.IP {
	var p [4]byte
	n := 0
	val := 0
	digits := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
			digits++
			if val > 255 || digits > 3 {
				return nil
			}
		} else if c == '.' {
			if digits == 0 || n >= 3 {
				return nil
			}
			p[n] = byte(val)
			n++
			val = 0
			digits = 0
		} else {
			return nil
		}
	}
	if n != 3 || digits == 0 {
		// Thử fallback sang net.ParseIP nếu là định dạng IPv6 hoặc đặc biệt
		if ip := net.ParseIP(s); ip != nil {
			copy(dst[:], ip)
			return net.IP((*dst)[:len(ip)])
		}
		return nil
	}
	p[3] = byte(val)

	// Biểu diễn IPv4 dưới dạng chuẩn 16-byte IPv4-mapped IPv6 (tương thích net.IP)
	dst[0], dst[1], dst[2], dst[3] = 0, 0, 0, 0
	dst[4], dst[5], dst[6], dst[7] = 0, 0, 0, 0
	dst[8], dst[9], dst[10], dst[11] = 0, 0, 0xff, 0xff
	dst[12], dst[13], dst[14], dst[15] = p[0], p[1], p[2], p[3]
	return net.IP((*dst)[:])
}
