# Decision Engine Model

Tài liệu này đặc tả mô hình ra quyết định chi tiết (Decision Logic) và các quy tắc gom nhóm kết quả của **Standalone Policy Engine**.

---

## 1. Thuật toán Ra Quyết định Tổng hợp (Consolidated Decision Algorithm)

Khi PEP gửi yêu cầu phân quyền, PDP thực hiện quy trình tổng hợp quyết định dựa trên thuật toán sau:

```go
func EvaluateDecision(matchedPolicies []*ast.PolicyNode, ctx *EvalContext) (Decision, string, error) {
    if len(matchedPolicies) == 0 {
        return Deny, "no_policies_matched", nil
    }

    hasAllow := false
    var matchedAllowID string
    
    for _, policy := range matchedPolicies {
        // 1. Chạy Evaluator đánh giá điều kiện
        isMatch, err := EvaluateCondition(policy.Condition, ctx)
        if err != nil {
            // Lỗi đánh giá (Ví dụ: Thiếu thuộc tính) -> Mặc định coi là không khớp điều kiện
            continue
        }
        
        if isMatch {
            if policy.Effect == "forbid" {
                // Gặp luật FORBID khớp -> Cấm ngay lập tức (Forbid Overrides)
                return Deny, fmt.Sprintf("explicitly_forbidden_by_policy_%s", policy.ID), nil
            }
            if policy.Effect == "permit" {
                hasAllow = true
                matchedAllowID = policy.ID
            }
        }
    }

    if hasAllow {
        return Allow, fmt.Sprintf("allowed_by_policy_%s", matchedAllowID), nil
    }

    return Deny, "no_matching_allow_policy", nil
}
```

---

## 2. Giải thích Quyết định (Explain Mechanism)

Để đáp ứng yêu cầu **FR-005 (Policy Explain)**, response trả về cho API Gateway không chỉ có kết quả ALLOW/DENY thô mà còn đi kèm lý do và danh sách các ID chính sách đã kích hoạt.

*   **Trường hợp ALLOW:** Trả về ID của chính sách `permit` đã cho phép truy cập.
*   **Trường hợp DENY (do Forbid cản trở):** Trả về ID của chính sách `forbid` cụ thể đã chặn truy cập.
*   **Trường hợp DENY (do mặc định):** Trả về mã lý do `"no_matching_allow_policy"` để chỉ ra rằng không có bất kỳ luật cho phép nào được tìm thấy cho ngữ cảnh này.
*   **Decision Audit Metadata:** Thông tin này được đẩy sang hệ thống log để phục vụ việc hiển thị trên Dashboard giám sát, giúp quản trị viên SOC nhanh chóng debug xem tại sao người dùng bị chặn quyền.
