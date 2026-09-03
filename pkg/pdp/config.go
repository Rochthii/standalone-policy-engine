package pdp

import (
	"time"

	"standalone-policy-engine/internal/storage"

	"github.com/redis/go-redis/v9"
)

// Config chứa các thiết lập cấu hình khi khởi tạo In-Process Embedded PDP SDK.
type Config struct {
	// Storage kết nối trực tiếp đến PostgreSQL (nếu chạy in-process với DB).
	Storage *storage.Storage

	// AllowedTenants là danh sách Whitelist các Tenant mà microservice này phục vụ.
	// SDK sẽ CHỈ nạp và đồng bộ chính sách cho các tenant này để ngăn ngừa tràn bộ nhớ (OOM).
	AllowedTenants []string

	// SyncInterval là chu kỳ đồng bộ nền dự phòng (mặc định 10 giây).
	SyncInterval time.Duration

	// RedisClient kết nối Redis Pub/Sub để nhận sự kiện cập nhật nóng tức thì (tùy chọn).
	RedisClient redis.UniversalClient
}
