package engine

import (
	"errors"
	"fmt"
	"sync"
)

// RoleDAG quản lý cấu trúc đồ thị phân cấp vai trò dạng có hướng không chu trình (DAG).
// Hỗ trợ cập nhật động và tra cứu quan hệ kế thừa với độ phức tạp O(1) nhờ cơ chế
// tính toán trước bao đóng bắc cầu (Transitive Closure).
type RoleDAG struct {
	mu sync.Mutex

	// adj lưu danh sách các vai trò kế thừa trực tiếp.
	// Ví dụ: adj["super_admin"] = ["admin"] có nghĩa là super_admin kế thừa admin.
	adj map[string][]string

	// transitiveClosure lưu trữ tập hợp tất cả các vai trò kế thừa trực tiếp và gián tiếp.
	// Ví dụ: transitiveClosure["super_admin"] = {"admin": true, "operator": true}
	// Giúp kiểm tra thừa kế vai trò chỉ tốn O(1) ở runtime.
	transitiveClosure map[string]map[string]bool

	// inheritedRoles lưu sẵn slice bất biến chứa chính nó và các vai trò kế thừa.
	// Ví dụ: inheritedRoles["super_admin"] = ["super_admin", "admin", "operator"]
	// Giúp GetInheritedRoles đạt O(1) và 0 heap allocations ở runtime.
	inheritedRoles map[string][]string
}

// NewRoleDAG tạo mới một instance RoleDAG.
func NewRoleDAG() *RoleDAG {
	return &RoleDAG{
		adj:               make(map[string][]string),
		transitiveClosure: make(map[string]map[string]bool),
		inheritedRoles:    make(map[string][]string),
	}
}

// AddInheritance thiết lập quan hệ kế thừa: parent kế thừa các quyền của child.
// Ví dụ: AddInheritance("super_admin", "admin")
// Hàm tự động phát hiện và chặn các mối quan hệ thừa kế tạo vòng lặp (chu trình).
func (g *RoleDAG) AddInheritance(parent, child string) error {
	if parent == child {
		return errors.New("một vai trò không thể kế thừa chính nó")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. Bản sao tạm thời của adj để kiểm tra chu trình
	tempAdj := make(map[string][]string)
	for k, v := range g.adj {
		tempAdj[k] = append([]string(nil), v...)
	}
	tempAdj[parent] = append(tempAdj[parent], child)

	// 2. Chạy thuật toán phát hiện chu trình
	if hasCycle(tempAdj) {
		return fmtErrorf("phát hiện quan hệ thừa kế vòng lặp (chu trình) giữa '%s' và '%s'", parent, child)
	}

	// 3. Nếu an toàn, cập nhật adj chính thức
	g.adj[parent] = append(g.adj[parent], child)

	// 4. Tính toán lại bao đóng bắc cầu
	g.rebuildTransitiveClosure()

	return nil
}

// IsDescendant kiểm tra xem vai trò parent có kế thừa (trực tiếp hoặc gián tiếp) vai trò child không.
// Ví dụ: IsDescendant("super_admin", "operator") -> true
// Độ phức tạp: O(1) nhờ map transitiveClosure bất biến (100% Lock-Free).
func (g *RoleDAG) IsDescendant(parent, child string) bool {
	if parent == child {
		return true
	}
	if g == nil {
		return false
	}

	ancestors, exists := g.transitiveClosure[parent]
	if !exists {
		return false
	}

	return ancestors[child]
}

// GetInheritedRoles trả về tất cả các vai trò mà vai trò cho trước kế thừa (bao gồm cả chính nó).
// Ví dụ: GetInheritedRoles("super_admin") -> ["super_admin", "admin", "operator"]
// Trả về trực tiếp slice đã tính trước (0 allocation nếu role đã được định nghĩa trong DAG).
func (g *RoleDAG) GetInheritedRoles(role string) []string {
	if g != nil {
		if roles, exists := g.inheritedRoles[role]; exists {
			return roles
		}
	}
	return []string{role}
}

// GetInheritedRolesInto là phiên bản zero-allocation tuyệt đối.
// Nếu role chưa có trong DAG, ghi trực tiếp vào stack buffer scratch thay vì cấp phát slice mới trên heap.
func (g *RoleDAG) GetInheritedRolesInto(role string, scratch *[16]string) []string {
	if g != nil {
		if roles, exists := g.inheritedRoles[role]; exists {
			return roles
		}
	}
	(*scratch)[0] = role
	return (*scratch)[:1:1] // Khóa cứng capacity = 1 để bảo vệ stack scratch không bị append làm tràn bộ đệm
}

// hasCycle phát hiện chu trình trên đồ thị danh sách kề sử dụng DFS và ba màu trạng thái.
func hasCycle(adj map[string][]string) bool {
	visited := make(map[string]int) // 0: chưa duyệt, 1: đang duyệt (gray), 2: đã duyệt xong (black)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = 1 // Đang duyệt

		for _, neighbor := range adj[node] {
			state := visited[neighbor]
			if state == 1 {
				return true // Phát hiện cạnh ngược (back edge) -> có chu trình
			}
			if state == 0 {
				if dfs(neighbor) {
					return true
				}
			}
		}

		visited[node] = 2 // Duyệt xong
		return false
	}

	// Kiểm tra chu trình từ tất cả các nút
	for node := range adj {
		if visited[node] == 0 {
			if dfs(node) {
				return true
			}
		}
	}

	return false
}

// rebuildTransitiveClosure tính toán lại bao đóng bắc cầu cho tất cả các vai trò.
// Sử dụng DFS tìm đường đi đến tất cả các nút con kế thừa trực tiếp/gián tiếp.
func (g *RoleDAG) rebuildTransitiveClosure() {
	g.transitiveClosure = make(map[string]map[string]bool)
	g.inheritedRoles = make(map[string][]string)

	for node := range g.adj {
		g.transitiveClosure[node] = make(map[string]bool)
		visited := make(map[string]bool)
		g.dfsCollect(node, node, visited)

		// Pre-compute slice bất biến cho node và khóa cứng capacity = length (3-index slice)
		// Ngăn chặn 100% rủi ro append() làm ô nhiễm backing array trong DAG.
		roles := make([]string, 1, len(g.transitiveClosure[node])+1)
		roles[0] = node
		for anc := range g.transitiveClosure[node] {
			roles = append(roles, anc)
		}
		g.inheritedRoles[node] = roles[:len(roles):len(roles)]
	}
}

func (g *RoleDAG) dfsCollect(startNode, currentNode string, visited map[string]bool) {
	visited[currentNode] = true
	if startNode != currentNode {
		g.transitiveClosure[startNode][currentNode] = true
	}

	for _, neighbor := range g.adj[currentNode] {
		if !visited[neighbor] {
			g.dfsCollect(startNode, neighbor, visited)
		}
	}
}

// fmtErrorf là hàm helper tạo lỗi định dạng.
func fmtErrorf(format string, a ...interface{}) error {
	return errors.New(formatString(format, a...))
}

func formatString(format string, a ...interface{}) string {
	// Dummy formatter đơn giản để tránh import fmt cồng kềnh nếu không cần thiết.
	// Nhưng ở đây dùng fmt.Sprintf là chuẩn nhất.
	return fmtSprintf(format, a...)
}

func fmtSprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}
