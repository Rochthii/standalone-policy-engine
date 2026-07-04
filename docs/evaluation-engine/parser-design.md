# Parser & Lexer Design

Tài liệu này đặc tả thiết kế kỹ thuật của bộ phân tích từ vựng (Lexer) và bộ phân tích cú pháp (Parser) cho **Standalone Policy Engine**.

---

## 1. Phân tích từ vựng (Lexer Design)

Lexer chịu trách nhiệm chuyển đổi chuỗi text DSL chính sách thô thành một chuỗi các Token có ý nghĩa.

*   **Thuật toán State-Machine:** Lexer hoạt động như một máy trạng thái hữu hạn (FSM). Nó đọc chuỗi ký tự tuần tự (rune by rune) từ trái qua phải và sử dụng con trỏ `pos` để theo dõi vị trí hiện tại.
*   **Tránh backtracking:** Cú pháp của DSL được thiết kế tối giản để Lexer có thể phân tích thành công chỉ với **`lookahead = 1` (LL(1))** ký tự tiếp theo, giúp tốc độ phân tích đạt mức tối đa.

---

## 2. Phân tích cú pháp (Parser Design)

Parser nhận đầu vào là danh sách Token từ Lexer và dựng cây AST tương ứng.

*   **Thuật toán Recursive Descent (Phân tích đệ quy đi xuống):** 
    *   Parser được viết hoàn toàn bằng tay (Hand-written) thay vì sử dụng các công cụ sinh parser tự động (như Yacc/Antlr) để đạt hiệu năng tối đa và dễ dàng kiểm soát lỗi.
    *   Mỗi quy tắc ngữ pháp trong file EBNF sẽ tương ứng với một hàm trong Parser (ví dụ: `parsePolicy()`, `parseExpression()`, `parseLogicalOr()`).
*   **Thuật toán Operator Precedence (Độ ưu tiên toán tử):**
    *   Parser sử dụng thuật toán **Pratt Parsing** (Top-Down Operator Precedence) để xử lý mệnh đề điều kiện trong khối `when`. Thuật toán này giúp phân tích các biểu thức toán học, logic lồng nhau một cách cực kỳ thanh thoát và nhanh gọn, tránh việc tạo quá nhiều tầng hàm đệ quy sâu.

---

## 3. Quản lý Lỗi & Phòng ngừa Panic (Error Recovery & Panic Prevention)

*   **Không xảy ra Panic:** Bộ Parser tuyệt đối không sử dụng hàm `panic()` để xử lý lỗi cú pháp. Mọi lỗi phải được đóng gói vào đối tượng `error` của Go và trả về cho Control Plane một cách an toàn.
*   **Error Recovery (Phục hồi sau lỗi):** 
    *   Nếu một chính sách trong tệp cấu hình bị lỗi cú pháp, Parser sẽ thực hiện đồng bộ lại (Synchronization) bằng cách bỏ qua các token tiếp theo cho đến khi gặp dấu chấm phẩy `;` (kết thúc câu luật), sau đó tiếp tục parse chính sách kế tiếp.
    *   Cơ chế này giúp trả về danh sách toàn bộ các lỗi cú pháp trong tệp cấu hình cho Admin chỉ trong 1 lần gọi API, thay vì dừng lại ngay lập tức ở lỗi đầu tiên.
