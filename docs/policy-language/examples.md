# Policy DSL Examples

Tài liệu này tổng hợp các ví dụ thực tế về cách viết chính sách phân quyền (DSL) cho các mô hình RBAC, ABAC và PBAC của **Standalone Policy Engine**.

---

## 1. Ví dụ mô hình RBAC (Role-Based Access Control)

### Kịch bản 1: Chỉ cho phép người dùng có vai trò `admin` hoặc kế thừa từ `admin` quản trị chính sách
```cedar
permit(
    principal in role:admin,
    action == action:CreatePolicy,
    resource == any
);
```

### Kịch bản 2: Cho phép vai trò `operator` xem logs hệ thống
```cedar
permit(
    principal == role:operator,
    action == action:ViewLogs,
    resource == file:system_audit_log
);
```

---

## 2. Ví dụ mô hình ABAC (Attribute-Based Access Control)

### Kịch bản 1: Phân quyền theo giờ làm việc hành chính và địa chỉ IP
*   *Mô tả:* Cho phép nhân viên sửa đổi hồ sơ khách hàng chỉ khi họ kết nối từ IP dải văn phòng và trong thời gian từ 08:00 đến 17:00.
```cedar
permit(
    principal in role:staff,
    action == action:UpdateProfile,
    resource == any
)
when {
    context.ip_address in "192.168.1.0/24" &&
    context.request_time >= "08:00:00Z" &&
    context.request_time <= "17:00:00Z"
};
```

### Kịch bản 2: So khớp thuộc tính động giữa Người dùng và Tài nguyên (Data Ownership)
*   *Mô tả:* Người dùng chỉ được sửa đổi tài liệu nếu họ là chủ sở hữu (Owner) của tài liệu đó.
```cedar
permit(
    principal in role:user,
    action == action:EditDocument,
    resource == any
)
when {
    principal.id == resource.owner_id
};
```

---

## 3. Ví dụ mô hình PBAC kết hợp (Cấm tường minh - Explicit Forbid)

### Kịch bản: Cấm nhân viên truy cập dữ liệu nhạy cảm nếu trạng thái thiết bị không an toàn (Compromised)
*   *Mô tả:* Dù có luật cho phép (permit) từ trước, chỉ cần thiết bị của người dùng bị báo cáo không an toàn, họ sẽ bị chặn truy cập ngay lập tức.
```cedar
forbid(
    principal in role:employee,
    action == any,
    resource == folder:classified_finance
)
when {
    context.device_status == "compromised" ||
    context.network_type == "public_wifi"
};
```
*Ghi chú: Nhờ cơ chế Forbid Overrides, câu luật cấm này sẽ ghi đè và chặn đứng mọi luật cho phép khác đối với thư mục tài chính.*
