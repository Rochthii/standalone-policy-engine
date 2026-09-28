# Bản thảo luận văn — Chương 3–5

**Đề tài:** Cơ chế ủy quyền ràng buộc giao dịch cho hành động của tác tử AI trong hệ thống ERP: Thiết kế và đánh giá trên Odoo 17.

**Trạng thái:** bản thảo nội dung ngày 2026-09-28; chưa phải bản xuất DOCX/PDF hay bản nộp. Các mục C1–C9 dưới đây tuân theo [bảng đối chiếu claim–evidence](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md). Khi có khác biệt giữa tài liệu thiết kế cũ và bằng chứng mới, [current-state audit](../technical-spec/CURRENT_STATE_AUDIT.md) cùng mã nguồn/kiểm thử hiện tại có thẩm quyền cao hơn. Số liệu EVAL-01/02/03 là kết quả của worktree có thay đổi chưa commit tại `304c1f5`, không phải bản phát hành có provenance hoàn chỉnh.

## Chương 3. Mô hình ủy quyền ràng buộc giao dịch

### 3.1. Vấn đề và ranh giới nghiên cứu

Tác tử AI trong nghiên cứu này là một **chủ thể phần mềm không phải con người** có thể đề xuất lời gọi công cụ nghiệp vụ thay mặt một người dùng. Nghiên cứu không đo khả năng suy luận của mô hình. Câu hỏi là hệ thống ERP có thể quyết định và thực thi đúng quyền cho một thay đổi nghiệp vụ cụ thể hay không, ngay cả khi yêu cầu bị sửa, trạng thái đơn thay đổi, quyền bị thu hồi, phê duyệt cũ được dùng lại hoặc client gửi lại lệnh sau khi mất phản hồi.

Trường hợp khảo sát là **một đường xác nhận `purchase.order` trên Odoo 17**. Đơn có người tạo, nhà cung cấp, tiền tệ, tổng tiền, dòng hàng, trạng thái và người phê duyệt. Các thuộc tính này tạo một giao dịch đủ cụ thể để kiểm tra liên kết giữa quyền và thay đổi nghiệp vụ; chúng không đại diện cho mọi nghiệp vụ ERP. Ba lớp đóng góp là: ủy quyền theo danh tính người giao quyền, ràng buộc quyền với ý định và trạng thái giao dịch, và thực thi lại điều kiện đó trong transaction cuối. PDP Go, policy evaluator, gRPC và mTLS là phương tiện hiện thực. [Đề cương hiện hành](DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md) khóa RQ1–RQ4 và phạm vi này.

Agent chỉ đưa ra đề xuất hành động. Odoo PEP nhận diện bản ghi, đọc dữ liệu kinh doanh có thẩm quyền từ ORM/PostgreSQL và tạo yêu cầu gửi PDP. Nội dung prompt hoặc tham số agent đưa vào không trở thành nguồn sự thật cho amount, vendor, currency, lines hay trạng thái PO. Nếu PEP bị xâm phạm, giả định tin cậy của thiết kế không còn hiệu lực; đó là giới hạn của mô hình đe dọa, không phải tình huống được các ca kiểm thử chứng minh đã chống được. [Threat model](../technical-spec/THREAT_MODEL.md) và [hợp đồng CBI](../technical-spec/CANONICAL_BUSINESS_INTENT.md) mô tả các nguồn tin cậy này.

### 3.2. Chủ thể, trạng thái và điểm tin cậy

| Thành phần | Trách nhiệm trong đường bảo vệ | Giới hạn tin cậy |
|---|---|---|
| Human delegator | Cấp một grant cho agent với tenant, phạm vi và thời hạn. | Không tự xác nhận trạng thái hiện tại của PO hoặc hoàn tất phê duyệt. |
| AI agent | Đề xuất gọi tool xác nhận PO dưới grant. | Không tự khai báo quyền, CBI hay danh tính approver. |
| Odoo PEP | Ràng buộc route với PO, khóa/đọc lại bản ghi, dựng CBI, áp dụng thay đổi và ghi trạng thái command/approval. | Phải là thành phần ứng dụng đáng tin; không lấy trường bảo vệ từ input agent. |
| Go PDP | Kiểm tra JWT/tenant, grant/proof, policy và revocation; phát hành/xác minh AC theo quyền human. | Không trực tiếp quan sát trạng thái Odoo đang biến đổi; chỉ nhận CBI do PEP tin cậy dựng. |
| PostgreSQL | Cung cấp transaction, row lock, uniqueness, trigger và deferred validation cho phạm vi ERP. | Không bảo đảm hiệu ứng bên ngoài DB xảy ra đúng một lần. |
| Human approver | Quyết định một pending intent sau khi danh tính, quyền hiện hành và SoD được kiểm tra. | Không thể mở rộng grant hoặc phê duyệt một intent khác bằng AC cũ. |

Các danh tính `user:` và `agent:` được phân biệt. Quyền đại diện chỉ có **một hop human → agent**, trong cùng tenant và company theo đường được cấu hình. `REQUIRE_HUMAN_APPROVAL` không phải một quyết định thứ ba: nó là obligation đi kèm `ALLOW`. `DENY` vẫn là từ chối cứng kể cả khi response có obligation này. Odoo Activity dùng để thông báo và điều hướng người duyệt; hoàn thành Activity không phải bằng chứng đã phê duyệt. [Approval contract](../technical-spec/APPROVAL_CAPABILITY.md) quy định ranh giới này.

### 3.3. Bất biến an ninh và ngữ nghĩa thời điểm

Đối với một lệnh xác nhận PO trong phạm vi nghiên cứu, điều kiện để thay đổi được commit gồm: danh tính caller đáng tin, grant một hop còn hợp lệ, CBI khớp giao dịch, trạng thái material hiện tại khớp, policy/revocation hiện hành cho phép, AC hợp lệ khi cần human, và command/AC chỉ được tiêu thụ một lần cùng thay đổi PO. Có thể diễn đạt điều kiện cần như sau:

```text
CommitProtectedPO(c) ⇒
  TrustedAgent(c) ∧ ActiveDelegation(c) ∧ ExactCBI(c)
  ∧ CurrentMaterialState(c) ∧ CurrentConfiguredAuthority(c)
  ∧ (ApprovalRequired(c) ⇒ ValidExactAC(c))
  ∧ AtomicScopedConsumption(c)
```

Đây là bất biến của đường Odoo/PostgreSQL đã thử nghiệm **dưới giả định** mọi writer policy/revocation thuộc phạm vi dùng cùng ERP fence, trigger local-authority không bị tắt và clock UTC của PostgreSQL đáng tin. Các kiểm tra deadline ở bước deferred validation khi COMMIT, không phải khẳng định quyền vẫn hợp lệ ở thời điểm WAL bền vững hoặc response tới client. Thay đổi PO và tiêu thụ command/AC là nguyên tử **trong một DB transaction**; policy storage/PDP bên ngoài DB đó không được mô tả như một distributed transaction. [Security invariants](../technical-spec/SECURITY_INVARIANTS.md) nêu thiết kế; bằng chứng và ngôn ngữ claim cuối cùng ở [EVAL-01](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) và [EVAL-04](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md).

### 3.4. Delegation và proof của agent

Grant liên kết delegator, agent, tenant/company, phạm vi hành động và hiệu lực thời gian. PEP kiểm tra mối liên hệ giữa PO và grant, rồi tạo CBI cho lệnh đang xét. Proof V2 ràng buộc CBI hash cùng grant và danh tính agent; PDP xác minh cấu trúc, thời hạn, key ID và HMAC trước khi đánh giá policy cho đường PO được bảo vệ. Proof V1 chỉ còn ở ranh giới legacy đã nêu trong [CBI contract](../technical-spec/CANONICAL_BUSINESS_INTENT.md), không phải phương án fallback cho PO protected route. Một proof hợp lệ không tự cho phép xác nhận đơn: quyền hiện hành và điều kiện giao dịch vẫn phải được xét.

Sự phân tách này trả lời RQ1. Người ủy quyền xác lập **ai được đại diện cho ai**; proof chứng minh yêu cầu hiện tại mang đúng grant và intent; PDP còn kiểm tra quyền theo action/resource/tenant. Việc giả định agent tự điền trường ngữ cảnh đúng sẽ làm mất tính ràng buộc. Vì vậy PEP phải dựng các trường material từ bản ghi đã khóa. Source của kiểm tra proof ở [Go verifier](../../internal/security/delegation_proof_v2.go) và [gRPC request binding](../../internal/server/delegation_auth_v2.go); các ca âm tương ứng thuộc `AUTH-P01`, `AUTH-N01`, `AUTH-N02` trong [evaluation matrix](../technical-spec/EVALUATION_MATRIX.md).

### 3.5. Canonical Business Intent và trạng thái material

CBI v1 gồm phiên bản; tenant/company; resource/action; vendor; currency/scale; tổng tiền ở **integer minor units**; line digest; record state/write version; state witness; creator/delegator/agent; grant, command ID và proof version. Thứ tự trường và cách mã hóa byte được cố định; `intent_hash = SHA-256(canonical_intent_bytes)`. Hai ngôn ngữ Go/Python dùng chung vector để kiểm tra cùng một intent cho ra cùng bytes/hash. Không dùng thứ tự khóa JSON hay biểu diễn float để ký. [CBI contract](../technical-spec/CANONICAL_BUSINESS_INTENT.md) là định nghĩa normative của từng trường.

Tiền được đọc từ PostgreSQL `NUMERIC` và đổi chính xác: với giá trị `A` và scale tiền tệ `S`, `amount_minor = A × 10^S` chỉ khi kết quả là số nguyên nằm trong miền cho phép. Ví dụ `1234.56` USD ở scale 2 thành `123456`; đầu vào phải bị từ chối nếu phép nhân còn phần lẻ, thay vì âm thầm làm tròn. Tổng tiền, đơn giá dòng và số lượng là những khái niệm khác nhau: unit price/quantity dùng decimal canonical để bảo toàn độ chính xác riêng của Odoo.

Các dòng PO được sắp theo `(sequence, id)` rồi băm trên ID, thứ tự, loại dòng, sản phẩm, mô tả, UoM, số lượng, đơn giá, taxes, ngày dự kiến và write version. `state_witness` băm snapshot material gồm tenant/company, resource, vendor, currency, amount, line digest, state, write version và grant/creator. `write_date` là tín hiệu bổ sung chứ không phải bộ đếm luôn tăng cho mọi ORM write; thay đổi giá trị material vẫn phải được đối chiếu trực tiếp. Trường của addon bên ngoài có thể đổi hiệu lực xác nhận nhưng không nằm trong schema CBI v1 thì cần phiên bản serializer mới và kiểm thử mới; kết quả hiện tại không bao phủ chúng.

Agent yêu cầu xác nhận PO khi nó ở trạng thái ban đầu. Nếu policy buộc human review, PEP chuyển PO sang `to approve`, sau đó **dựng lại và lưu** CBI của trạng thái pending này. Phê duyệt phải gắn với post-transition intent, không dùng intent của draft trước đó. Khi human xử lý, row lock không được giữ xuyên thời gian chờ; lúc thực thi cuối, PEP khóa và tái dựng để phát hiện stale intent. Đây là mấu chốt của RQ2, gồm cả chống parameter substitution, stale approval và nonce dùng cho PO khác. Chứng cứ được giới hạn ở các trường/đường ORM và lịch xen kẽ đã liệt kê trong [EVAL-01 ledger](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md).

### 3.6. Approval Capability và Separation of Duties

AC v1 là bằng chứng mật mã cho việc một human độc lập, đang có quyền, đã duyệt **một** pending CBI/witness/command/grant với `approval_id`, expiry và one-time ID. Payload còn chứa tenant/company, agent/delegator/creator, approver và purpose cố định `odoo.purchase_order.confirm`. PDP ký bằng approval key ring tách khỏi delegation proof key ring; Odoo lưu và gửi lại capability nhưng không nắm secret của approval signer. HMAC xác nhận tính toàn vẹn đối với bên giữ khóa, không chứng minh chữ ký pháp lý hay chống chối bỏ sau key compromise. [Approval contract](../technical-spec/APPROVAL_CAPABILITY.md) xác định payload, envelope và lifecycle.

Issuance lấy người duyệt từ Odoo `env.user` đã xác thực, kiểm tra internal user, tenant/company, role, quyền PDP hiện hành và SoD. Approver không được là agent, người tạo PO hoặc delegator. Approval không phục hồi một grant đã hết hạn/thu hồi, không biến `DENY` thành `ALLOW` và không chuyển thẳng PO sang trạng thái cuối. Trạng thái bền vững đi qua `pending → approved → consumed` hoặc các nhánh terminal `rejected`, `invalidated`, `expired`. `consumed` chỉ được ghi trong transaction chứa final PO mutation. Reissue cùng request có kiểm tra idempotency; AC cho intent mới phải có nhận dạng mới, không tái sử dụng one-time ID cũ. Điều này trả lời RQ3 trong phạm vi SoD và database lifecycle được thử nghiệm.

### 3.7. Lựa chọn thiết kế và giới hạn

Mô hình dùng proof/intent để ràng buộc **nội dung** quyền, và lock/fence/transaction để ràng buộc **thời điểm** quyền. Hash một request trước lúc gọi PDP không ngăn trạng thái ERP đổi trước commit; khóa một PO cũng không tự làm policy/revocation bên ngoài đứng yên. Bởi vậy đường cuối phải đọc lại PO và approval, lấy quyết định hiện hành, khóa các fence cần thiết và để DB kiểm tra expiry tại deferred boundary. Đổi lại, coarse authority epoch có thể tăng contention; chi phí này chưa được đo. Quy trình khôi phục publication thủ công chưa có kiểm chứng end-to-end. Những điểm này là điều kiện vận hành của thiết kế, không được bỏ khỏi phát biểu kết quả.

## Chương 4. Hiện thực trên Odoo 17 và Go PDP

### 4.1. Cấu trúc triển khai và ranh giới mã nguồn

Hiện thực là một Odoo addon làm PEP, một Go PDP cung cấp quyết định và phát hành/xác minh capability, cùng PostgreSQL lưu PO/attempt/approval và fence. Các nguồn chính:

| Thành phần | Vị trí | Vai trò |
|---|---|---|
| PO entry point | [purchase_order.py](../../custom_addons/pdp_authorizer/models/purchase_order.py), [purchase_order_final.py](../../custom_addons/pdp_authorizer/models/purchase_order_final.py) | Tách direct, pending và approved-final route; ghi trạng thái command và final PO. |
| CBI/material read | [cbi_builder.py](../../custom_addons/pdp_authorizer/models/cbi_builder.py), [approval_final_intent.py](../../custom_addons/pdp_authorizer/models/approval_final_intent.py) | Flush, khóa, đọc lại PO/line và so pending/final intent. |
| Human approval | [approval_request.py](../../custom_addons/pdp_authorizer/models/approval_request.py), [approval_capability.py](../../custom_addons/pdp_authorizer/models/approval_capability.py), [approval_invalidation.py](../../custom_addons/pdp_authorizer/models/approval_invalidation.py) | Kiểm tra approver, phát hành AC, xác minh quyền hiện hành và chuyển trạng thái terminal. |
| Bypass/material guards | [purchase_order_transition_guard.py](../../custom_addons/pdp_authorizer/models/purchase_order_transition_guard.py), [purchase_order_line_guard.py](../../custom_addons/pdp_authorizer/models/purchase_order_line_guard.py) | Bảo vệ final transition và chạm parent version khi dòng thay đổi qua đường ORM đã liệt kê. |
| PDP proof/approval | [delegation_auth_v2.go](../../internal/server/delegation_auth_v2.go), [delegation_proof_v2.go](../../internal/security/delegation_proof_v2.go), [grpc_approval.go](../../internal/server/grpc_approval.go), [approval_capability.go](../../internal/security/approval_capability.go) | Ràng buộc proof V2, CheckAccess, Issue/Verify AC v1 theo tenant/subject/purpose. |
| Authority ordering | [authority_fence.py](../../custom_addons/pdp_authorizer/models/authority_fence.py), [revocation_fence.py](../../custom_addons/pdp_authorizer/models/revocation_fence.py), [erp_policy_fence.go](../../internal/storage/erp_policy_fence.go), [erp_revocation_fence.go](../../internal/storage/erp_revocation_fence.go) | Sắp thứ tự các writer đã cấu hình với protected transaction. |
| Deadline/idempotency | [authorization_attempt.py](../../custom_addons/pdp_authorizer/models/authorization_attempt.py) | Uniqueness của nonce, trạng thái command và deferred grant/approval deadline validation. |

Đường vận chuyển Odoo ↔ PDP dùng generated Protobuf, mTLS và JWT theo tenant/subject. `CheckAccess` có quyết định `ALLOW`/`DENY` và obligations; RPC AC riêng cho phát hành/xác minh. Backend policy/evaluator và Trie/DAG là hạ tầng kỹ thuật, không được đồng nhất với bằng chứng rằng toàn giao dịch ERP có độ trễ của evaluator in-memory.

### 4.2. Đường trực tiếp và tạo pending

Ở `button_confirm`, Odoo định vị một PO và, nếu là delegated route, dựng một protected request từ grant và CBI. Lệnh logic có command ID/nonce duy nhất trong `(tenant, grant)` để nhận diện retry. Sau JWT/mTLS, PDP xét proof, grant, policy/revocation và trả quyết định. Với `ALLOW` không có obligation, Odoo khóa fence thích hợp, thực hiện native transition đã guard và đánh dấu attempt `executed` trong cùng transaction. Nếu current authority/fence không còn hợp lệ, final effect không được commit.

Với `ALLOW` kèm đúng một `REQUIRE_HUMAN_APPROVAL`, Odoo ghi PO thành `to approve`, tạo một pending approval chứa **CBI dựng sau chuyển trạng thái**, kèm Activity để báo người duyệt. Attempt chuyển sang `approval_required`. Transaction pending kết thúc trước thời gian người duyệt suy nghĩ; chỉ tồn tại trạng thái không-final, chưa có AC `consumed`. `DENY`, kể cả khi mang obligation, không có đường vượt qua hard denial trên protected agent route. Các obligation khác không được hỗ trợ thì từ chối. [Nguồn Odoo entry point](../../custom_addons/pdp_authorizer/models/purchase_order.py) và [APP-P01 trong matrix](../technical-spec/EVALUATION_MATRIX.md) ràng buộc phát biểu này.

### 4.3. Ba transaction của đường cần phê duyệt

Trường hợp high-value approved được chia thành ba transaction quan sát được:

1. **Pending:** ghi `to approve`, pending CBI và Activity, sau đó commit. Không có xác nhận PO cuối.
2. **Issuance:** human trong Odoo session gọi `action_issue_capability`; Odoo khóa PO/approval, đối chiếu pending CBI, quyền và SoD; PDP phát hành AC v1; Odoo lưu `approved`, sau đó commit. PO vẫn `to approve`.
3. **Execution:** agent gọi lại lệnh gắn cùng command. Odoo khóa/tái dựng CBI và kiểm tra AC, approver, agent policy và authority hiện hành; PO/AC/attempt chỉ chuyển sang trạng thái cuối khi transaction này commit.

Việc tách transaction làm lộ một lỗi thực tế: ở bản đầu, một stored related field của dòng (`line.state`) có thể bị tính lại lúc pending commit và làm `line.write_date` thay đổi sau khi hash đã lưu. Một approval hợp lệ vì thế bị đánh dấu `intent_changed`. [Bản sửa](../../custom_addons/pdp_authorizer/models/cbi_builder.py) flush tất cả pending line fields **trước** snapshot; không bỏ write version hoặc nới phép so intent. [EVAL-02](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) chứng minh đường hợp lệ ba transaction pass và các thay đổi sau review vẫn làm AC cũ vô hiệu.

### 4.4. Cổng cuối dưới khóa

Đường approved-final trong [purchase_order_final.py](../../custom_addons/pdp_authorizer/models/purchase_order_final.py) chỉ xử lý một approval record tương ứng attempt. `pending` để PO không-final; terminal state khác `approved` bị chặn. Hàm final-intent và invalidation khóa, so sánh CBI hiện tại với pending binding, xác minh AC qua PDP và tái kiểm tra approver/SoD. Agent proof được dựng lại cho cùng locked intent; PDP phải trả `ALLOW` cho chính action/resource đó. `DENY` không được AC override. Sau khi lấy revocation fence và khóa current authority, route ghi `consumed`, gọi native `button_approve` theo ngữ cảnh guard nội bộ và ghi attempt `executed`. Nếu native transition không tới `purchase`/`done` thì phát lỗi và transaction phải rollback.

Việc guard các entry point final được đặt tại [transition guard](../../custom_addons/pdp_authorizer/models/purchase_order_transition_guard.py). Các ca `BOUND-N01` và liên quan chỉ cho phép phát biểu rằng **những route Odoo được liệt kê** không bypass PEP; không chứng minh mọi custom addon hoặc SQL trực tiếp đều bị ngăn. Các thay đổi dòng qua ORM trong phạm vi được [line guard](../../custom_addons/pdp_authorizer/models/purchase_order_line_guard.py) cập nhật parent version để tránh phantom membership. [EVAL-01 ledger](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) ghi lịch edit-first/final-first và oracle trạng thái bền vững.

### 4.5. Policy, revocation và deadline ở thời điểm cuối

PDP không cùng transaction database với mọi bước business mutation. Để đóng khoảng kiểm tra–commit **trong cấu hình testbed**, policy writer trước hết ghi một barrier ERP `ready=false`, sau đó mutate policy storage và xuất bản revision đã commit. Final Odoo route giữ khóa policy fence và so cả revision của approver lẫn agent với revision đang ready. Local authority epoch được trigger cập nhật cho các thay đổi user/group/membership/company/grant đã nêu; một thay đổi đồng thời đã commit buộc Odoo `REPEATABLE READ` retry hoặc deny thay vì dùng một snapshot cũ như hiện hành. Revocation writer của grant dùng cùng tenant/grant fence và tombstone tăng đơn điệu. [Policy fence](../../internal/storage/erp_policy_fence.go), [revocation fence](../../internal/storage/erp_revocation_fence.go) và [local authority lock](../../custom_addons/pdp_authorizer/models/authority_fence.py) thể hiện cơ chế này.

Với expiry, PostgreSQL deferred trigger trong [authorization_attempt.py](../../custom_addons/pdp_authorizer/models/authorization_attempt.py) dùng `clock_timestamp()` và kiểm tra grant cùng approval tiêu thụ ngay ở bước commit validation. Hai lịch thực nghiệm cho direct và approved đã giữ transaction mở tới sau grant expiry; commit lỗi và fresh observer thấy PO không-final, AC/command không bị tiêu thụ. Đây không phải đo riêng trường hợp AC-only expiry sau check, cũng không bảo đảm tính hợp lệ ở thời điểm WAL flush hoặc response. Một writer ngoài protocol fence, trigger bị tắt, clock sai hoặc PostgreSQL superuser bị xâm phạm nằm ngoài bảo đảm. Khôi phục publication thủ công vẫn thiếu kiểm thử end-to-end.

### 4.6. Retry, rollback và dấu vết kiểm chứng

Attempt giữ command ID và fingerprint; uniqueness ở `(tenant, grant, nonce)` ngăn một nonce dùng cho command khác trong cùng scope. Retry cùng command sau commit đọc lại terminal result thay vì gọi native mutation lần hai. Trong lịch hai session, cạnh tranh trên approval/command và database serialization cho một final PO effect, một AC `consumed` và một attempt `executed`. Lỗi được tiêm sau native transition nhưng trước commit làm PO, AC và attempt rollback cùng nhau. PDP/AC-verifier outage trên đường cuối để lại trạng thái không-final hoặc retryable. Đây là **at-most-once effect trong DB được bảo vệ**, không phải exactly-once cho thông báo, thanh toán hoặc tích hợp ngoài ERP. Kết quả được quan sát qua session mới trong [EVAL-01](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md).

## Chương 5. Đánh giá, trả lời câu hỏi nghiên cứu và giới hạn

### 5.1. Thiết kế thí nghiệm và đơn vị đếm

Đánh giá dùng ba nguồn độc lập: [EVAL-01](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) kiểm tra an ninh/chức năng, [EVAL-02](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) so sánh ba variant A/B/C, và [EVAL-03](../technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md) đo các boundary riêng. [EVAL-04](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) giới hạn cách diễn giải; nó không tạo thêm một phép chạy runtime. Các kết quả xuất phát từ source commit `304c1f5` cộng thay đổi worktree chưa commit, được ghi bằng raw artifact/source hash thay vì khẳng định một binary release tái tạo được hoàn toàn.

EVAL-01 có **25 case ID** với bằng chứng ghép qua những boundary thích hợp, không phải 25 thí nghiệm ERP end-to-end độc lập. Gate fresh-database cuối báo **75 Odoo post-tests, 0 failures, 0 errors**. Các runner riêng bao phủ concurrency/retry, 16 lịch thay đổi material, 3 grant-ordering, 3 thay đổi authority đã commit, 4 lịch policy/role sau ALLOW và 2 rollback theo grant expiry dùng clock thật. Không cộng các con số thuộc đơn vị khác nhau thành một tổng kiểm thử mới. Âm tính chỉ được chấp nhận khi fresh-session oracle không thấy **unauthorized persistent business mutation**, không chỉ vì exception được ném ra.

### 5.2. Kết quả an ninh theo bất biến

| Nhóm | Bằng chứng giới hạn | Kết quả quan sát và giới hạn |
|---|---|---|
| I1 — identity/delegation | `AUTH-P01`, `AUTH-N01`, `AUTH-N02` | Đường được cấu hình xác thực agent/tenant, grant một hop và proof; JWT/tenant/proof sai hoặc grant hết hiệu lực bị chặn. Chưa thử frontend identity lifecycle. |
| I2 — CBI/current state | `CBI-P01`, `CBI-N01`–`CBI-N05`, `TXN-N04` | Vector Go/Python khớp; các field và lịch edit được liệt kê làm intent cũ mất hiệu lực trước final. Không bao phủ mọi extension hoặc arbitrary SQL. |
| I3 — approval/SoD | `APP-P01`, `APP-P02`, `APP-N01`–`APP-N05` | Pending không-final; human độc lập phát hành AC cho đúng intent; thay thế, hết hạn, replay, wrong-role/tenant và key confusion theo retained variants bị chặn. Không có non-repudiation. |
| I4 — final/atomicity | `TXN-P01`, `TXN-P02`, `TXN-N01`–`TXN-N04` | Đường được bảo vệ khóa và tái kiểm tra, cùng transaction ghi PO/command/AC; các lịch rollback, retry, edit, authority và expiry đã liệt kê pass. Giới hạn ở DB/testbed và configured writer. |
| I5 — fail closed | `BOUND-N01`–`BOUND-N03` | Các entry point, trạng thái AC lỗi và outage được liệt kê không tạo final PO unauthorized; không suy ra coverage cho arbitrary addon hay độ sẵn sàng chung. |

Điểm quan trọng là kết quả này đã thay đổi theo bằng chứng. Một lịch sau PDP `ALLOW` trước đây tái hiện được lỗi revocation trước khi có shared grant fence. Sau khi sửa, grant-first buộc retry/deny và final-first chặn writer tới sau commit. Các policy/role và expiry schedules được bổ sung riêng. Audit giữ các thất bại cũ như lịch sử và [EVAL-01 ledger](../technical-spec/evidence/V2_EVAL_01_CASE_LEDGER_2026_09_24.md) ghi từng lần sửa/kiểm chứng; không được lấy một gate cũ làm bằng chứng của trạng thái hiện tại.

### 5.3. A/B/C trên cùng workflow

EVAL-02 dùng **11 scenario × 3 variant = 33 outcome** có before/after snapshot từ session mới. A là native Odoo dưới service account không phải superuser, vẫn có ACL/record rules; B là PDP live qua mTLS/JWT với các policy permit/forbid và ordinary per-PO approval khi cần; C là đường protected bình thường với delegation, CBI/proof, AC và final fence. A/B là adapter ablation trong runner offline, không phải cờ bypass trong addon hay so sánh ba sản phẩm triển khai độc lập. Cùng testbed và fixture logic được dùng; ID/nonce/timestamp của mỗi instance khác nhau. [EVAL-02 report](../technical-spec/evidence/V2_EVAL_02_COMPARISON_2026_09_27.md) ghi raw JSON và năm attempt lỗi chẩn đoán đã loại khỏi 33 outcome.

| Scenario | A | B | C |
|---|---|---|---|
| Direct allow 1000 USD | Final | Final | Final, một command executed |
| 2500 USD cần approval, chưa duyệt | Final | Pending | Pending, chưa phát hành AC |
| Hard policy DENY ở 2500 USD | Final | Denied | Denied |
| Human độc lập duyệt 2500 USD | Final | Final | Final, một AC consumed |
| Description sửa sau review | Final | Final | Pending, AC cũ invalidated |
| Amount đổi 2500 → 2600 sau review | Final | Final | Pending, AC cũ invalidated |
| Grant revoked trước execution | Final | Final | Denied |
| Creator cũng là delegator | Final | Final | Denied |
| Retry cùng PO | Snapshot không đổi | Snapshot không đổi | Snapshot không đổi; một command executed |
| Dùng nonce trên PO khác cùng grant | PO thứ hai final | PO thứ hai final | PO thứ hai denied |
| PDP connection unavailable | Final; không cần RPC | Denied, UNAVAILABLE | Denied, UNAVAILABLE |

Hai kết luận cần đi cùng bảng: B vẫn từ chối hard DENY/outage, và native A/B/C đều giữ nguyên snapshot trong **same-record retry** quan sát được. Do đó không thể mô tả B như policy luôn cho phép hoặc quảng bá retry thường là ưu thế độc quyền của C. Các variant gộp nhiều điều khiển; khác biệt không cô lập riêng tác động của CBI hash, AC hay policy fence. Thí nghiệm không so OPA/Cedar, không đo latency từ 33 outcome và không chứng minh mọi policy-only system có các điểm yếu này.

### 5.4. Phân phối thời gian theo boundary

EVAL-03 đo một worker tuần tự sau warm-up. Go dùng 100 warm-up và **1.000 mẫu đo cho mỗi boundary**: 3.000 API calls và 1.000 timer controls, cộng 400 warm-ups. Go chạy trên Windows `go1.26.4`; clock `QueryPerformanceCounter` có tần số 10 MHz, nominal tick 100 ns. Odoo dùng **20 warm-up + 200 giao dịch đo mỗi route**, tổng 440 row, trong đó 400 giao dịch final được commit và kiểm tra từ session mới. Odoo/PDP/PG chạy trong testbed riêng; setup, issuance, human wait và fresh observer không nằm trong `confirm_through_commit`. Mọi giá trị dưới đây là thống kê của **successful measured events**, warm-up loại khỏi phân vị, 0 lỗi ở các boundary được trình bày. Cách nội suy phân vị là `(n−1)×p`; raw samples, source hashes và môi trường nằm ở [EVAL-03 report](../technical-spec/evidence/V2_EVAL_03_MEASUREMENT_2026_09_27.md).

Go API và timer control (đơn vị µs; mỗi hàng `n=1.000`, 0 lỗi):

| Boundary | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|
| `timer_control` | 0.200 | 0.300 | 0.401 | 2.100 |
| `evaluator_single_permit` | 0.600 | 1.500 | 5.000 | 228.700 |
| `proof_v2_verify` | 11.000 | 22.820 | 42.413 | 179.200 |
| `capability_v1_verify` | 3.900 | 10.100 | 21.615 | 3,194.000 |

Giá trị timer control không được trừ khỏi các phân vị khác. Chi phí đọc QPC, dispatch và predicate đã nằm trong phép đo; evaluator là **một permit đã compile**, amount 1000, không có DB/JWT/RPC hoặc workload policy ERP đầy đủ. Hai artifact Go cũ bị loại vì clock Windows trả nhiều giá trị 0 hoặc vì oracle timer-control quá chặt; raw thất bại được giữ riêng và không đóng góp vào bảng này. Max AC lớn hơn phần lớn mẫu, chưa có phân tích nguyên nhân để biến nó thành một khẳng định tail ổn định.

Các boundary Odoo trọng tâm (đơn vị ms, 0 lỗi trong số event ghi ở cột `n`):

| Boundary | Route | n event | p50 | p95 | p99 | max |
|---|---|---:|---:|---:|---:|---:|
| `confirm_through_commit` | direct | 200 | 70.521 | 139.866 | 152.247 | 174.005 |
| `confirm_through_commit` | approved | 200 | 75.642 | 144.614 | 174.623 | 179.492 |
| `protected_button_confirm` | direct | 200 | 47.117 | 96.170 | 116.291 | 133.662 |
| `protected_button_confirm` | approved | 200 | 47.687 | 92.375 | 120.325 | 132.999 |
| `commit` | direct | 200 | 22.002 | 48.144 | 62.445 | 66.424 |
| `commit` | approved | 200 | 26.989 | 58.949 | 67.893 | 90.160 |
| `locked_cbi` | direct | 200 | 4.360 | 11.716 | 13.279 | 14.336 |
| `locked_cbi` | approved | 600 | 3.970 | 10.284 | 15.037 | 19.404 |
| `rpc_check_access` | direct | 200 | 1.964 | 5.329 | 6.242 | 6.801 |
| `rpc_check_access` | approved | 400 | 1.345 | 3.403 | 5.359 | 6.774 |
| `rpc_verify_capability` | approved | 200 | 1.061 | 2.632 | 3.305 | 4.490 |
| `native_button_approve` | direct | 200 | 0.536 | 1.607 | 2.038 | 3.487 |
| `native_button_approve` | approved | 200 | 0.541 | 1.581 | 2.349 | 2.635 |

Approved route gọi `locked_cbi` ba lần và `rpc_check_access` hai lần mỗi giao dịch; **600/400 là số event**, không phải 600/400 giao dịch độc lập. Phân phối RPC approved gộp kiểm tra approver và agent. Các span Odoo lồng nhau, nên không cộng/trừ các phân vị để tính overhead. `native_button_approve` chỉ đo phần synchronous native method, loại phần chuẩn bị và commit. Direct 1000 USD và approved 2500 USD khác giá trị, trạng thái và số bước; chênh lệch `confirm_through_commit` là mô tả hai workflow, không phải causal approval cost. Thí nghiệm không đo concurrency, throughput, cold start, recovery hay SLA production.

### 5.5. Trả lời RQ1–RQ4

**RQ1 — Agent được đại diện cho ai?** Trong đường đã thử nghiệm, JWT ràng buộc agent/tenant, mTLS bảo vệ kết nối Odoo–PDP, còn proof V2 và grant human → agent ràng buộc scope, company và thời hạn; grant/revocation phải còn hợp lệ theo giao thức writer được cấu hình. Các ca identity/proof/tenant/grant âm không dẫn đến final mutation. Kết luận có độ tin cậy cao với những trường hợp đã liệt kê, chưa đánh giá frontend handoff hoặc lifecycle của identity provider ngoài testbed.

**RQ2 — Quyền đó áp dụng cho giao dịch nào?** PEP dựng CBI từ bản ghi đã khóa, proof mang hash của intent, pending review lưu post-transition CBI và final route dựng lại toàn bộ material snapshot. Các thay đổi amount một minor unit, vendor/currency, line/state và request action/resource trong danh sách kiểm thử làm intent cũ không còn dùng được; nonce dùng trên PO khác bị từ chối. Kết luận có độ tin cậy cao trên schema/ORM schedules đã liệt kê, không chứng minh coverage cho arbitrary extension hoặc PEP bị xâm phạm.

**RQ3 — Approval có còn đúng và dùng một lần?** Human độc lập được kiểm tra quyền/SoD khi issuance và khi final; AC v1 gắn approval ID, CBI hash, witness, command, grant, approver và expiry. Đường được thử nghiệm chỉ ghi `consumed` cùng final PO/attempt trong một PostgreSQL transaction; rollback/race/retry giữ one-time semantics cho effect đó. HMAC không biến human approval thành chữ ký pháp lý, và kết quả không mở rộng sang external effects.

**RQ4 — Khác baseline ra sao và chi phí gì?** Trong 11 scenario chọn trước, C loại trừ các stale review, revoked grant, creator/delegator và cross-record nonce outcomes mà A/B cụ thể cho phép, trong khi B vẫn xử lý hard DENY/outage và cả ba có same-record retry ổn định. EVAL-03 cho biết phân phối từng boundary cùng total final workflow dưới hai route. Độ tin cậy cho kết luận **so sánh giới hạn** ở mức vừa: A/B bundle khác cơ chế, số mẫu/fixture hữu hạn, total direct/approved không matched, không có factorial baseline hoặc concurrent-load estimate. Không đủ cơ sở kết luận policy engine nào nhanh hơn hay C có overhead approval bao nhiêu theo quan hệ nhân quả.

### 5.6. Threats to validity

**Construct validity.** Định nghĩa AI agent là software principal chứ không chạy LLM thực, nên thí nghiệm nói về authorization sau lời gọi tool, không đo prompt injection hay suy luận. Một xác nhận PO là phép đại diện hữu ích cho giao dịch tác động cao nhưng không đại diện toàn bộ ERP/P2P. A/B/C là ablation bundle nên không tách được đóng góp biên của từng điều khiển.

**Internal validity.** Runner tin cậy không đi qua frontend login hay real agent tool stack. Worktree chưa commit tại `304c1f5` có raw source hashes nhưng không có signed release provenance. Kết luận authority ordering cần configured writers cùng fence, trigger nguyên vẹn và PostgreSQL UTC clock. EVAL-02 từng phát hiện false invalidation do deferred line recompute và đã sửa bằng flush; EVAL-03 từng dùng Windows clock quá thô và một oracle zero-control sai. Các artifact lỗi được giữ và loại khỏi kết quả cuối, nhưng phát hiện/sửa được lỗi không chứng minh không còn lỗi khác.

**External validity.** Chỉ một Odoo 17 Community PO route, PostgreSQL 15, fixture hữu hạn và các schedule đã liệt kê được thử. Không có SAP runtime, hệ ERP khác, custom material fields bất kỳ, SQL ngoài ORM, payment, multi-tenant scale hoặc geographically distributed effect. Fixture so sánh tài chính dùng giá trị nguyên đơn vị lớn; các vector exact minor-unit riêng không chứng minh policy xử lý mọi giá trị fractional trong A/B/C.

**Conclusion/reproducibility validity.** 25 ID, 75 post-tests, runner schedules và 33 A/B/C outcomes là những đơn vị khác nhau. Go QPC có 100 ns nominal tick và overhead dụng cụ; Odoo spans inclusive, direct/approved workload khác nhau. 200 giao dịch mỗi route chỉ cho p99 mô tả đuôi mẫu, không có confidence interval hay kiểm tra tải production. Cold start, contention của authority epoch, crash/manual publication recovery và ambiguous commit timeout chưa được đo đầy đủ. Do đó các số không hỗ trợ speedup, SLA hay general ERP superiority. [EVAL-04 ledger](../technical-spec/evidence/V2_EVAL_04_CLAIM_EVIDENCE_2026_09_28.md) cung cấp bảng ràng buộc claim để dùng khi biên tập cuối.

### 5.7. Khả năng áp dụng cho SAP và hướng mở rộng

Khi chuyển ý tưởng sang SAP, cần xác định lại: danh tính human/agent được truyền qua dịch vụ nào; business object và field nào là authoritative; approval workflow nào cho phép gắn AC với cùng action/state; transaction và extension point nào có thể tái kiểm tra trước mutation; các writer policy/revocation nào phải dùng một ordering protocol tương đương. Đây là **câu hỏi kiến trúc** theo [scope alignment](THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md), không phải mapping API hay bằng chứng tương thích. Một kiểm chứng sau này cần hiện thực SAP riêng và chạy lại test/measurement với ranh giới đó.

### 5.8. Kết luận của phần thực nghiệm

Ba lớp cơ chế được mô tả ở Chương 3 có bằng chứng ghép trên đường PO Odoo 17 đã chọn: delegation theo danh tính, ràng buộc exact intent/state và tái kiểm tra/tiêu thụ trong transaction cuối. EVAL-02 cho thấy khác biệt với hai ablation cụ thể; EVAL-03 cho biết chi phí quan sát được ở các boundary không đồng nhất. Tính đúng đắn chỉ được khẳng định trong tập điều kiện, trường dữ liệu, entry point và lịch thực nghiệm đã công bố. Kết quả này là nền tảng để đánh giá tính khả thi của authorization cho agent trong ERP, chưa phải chứng nhận hệ thống dùng ở production.
