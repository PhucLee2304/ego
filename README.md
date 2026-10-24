# Ego - Backend Microservices

Hệ thống Backend Microservices được viết bằng Go, sử dụng gRPC cho liên lạc nội bộ và RESTful API (thông qua gRPC-Gateway) cho client bên ngoài.

## Services và cổng gRPC nội bộ

| Service | Port |
|---|---:|
| identity | 50052 |
| storage | 50054 |
| topics | 50055 |
| notifications | 50058 |
| learning | 50059 |

`identity` là service gộp từ `auth` + `users` (dùng chung DB `users-db`), contract gRPC ở `api/proto/identity/identity.proto` (`IdentityService`: token + user). Các service khác gọi identity qua một biến duy nhất `IDENTITY_SERVICE_ADDR` (mặc định `identity:50052`). Route public cũ `/auth/api/v1/...`, `/users/api/v1/...` vẫn giữ nguyên.

`learning` là service gộp từ `exams` + `classrooms` (một DB `learning-db`), contract gRPC `api/proto/learning/learning.proto` (`LearningService.GetClassroomPostAccess`, posts gọi qua `LEARNING_SERVICE_ADDR`, mặc định `learning:50059`). Route public cũ `/exams/api/v1/...`, `/classrooms/api/v1/...` (kể cả 2 WebSocket `/exams/api/v1/ws/exams`, `/classrooms/api/v1/ws/classrooms`) vẫn giữ nguyên, nginx chuyển về `learning`; Swagger ở `/learning/`. Seed đề: `make seed learning`.

## 🛠 Yêu cầu hệ thống (Prerequisites)

Trước khi bắt đầu, đảm bảo máy tính của bạn đã cài đặt:
- **[Go](https://golang.org/dl/)** (Phiên bản 1.21 trở lên)
- **[Docker](https://www.docker.com/)** & **Docker Compose**
- **[Protoc](https://grpc.io/docs/protoc-installation/)** (Protocol Buffers Compiler)
- **[Swaggo](https://github.com/swaggo/swag)** (`go install github.com/swaggo/swag/cmd/swag@latest`)
- **Make** (Trên Windows có thể dùng qua Git Bash, MSYS2 hoặc WSL)

## ⚙️ Hướng dẫn khởi tạo (Code Generation)

Mở terminal tại thư mục gốc `ego/` và chạy các lệnh dưới đây (Makefile đã được cấu hình tự động nhận diện Windows/Mac/Linux để chạy mượt mà):

### 1. Sinh code gRPC & Protobuf
Lệnh này sẽ duyệt qua tất cả các thư mục trong `api/proto/` và sinh ra code Go tương ứng vào `api/gen/go/`:
```bash
make protoc

# (Tùy chọn) Nếu máy chưa cài Make nhưng xài Windows, có thể chạy trực tiếp:
# .\generate.ps1 protoc
```

### 2. Sinh tài liệu Swagger (API Docs)
Lệnh này sẽ quét tất cả các microservices bên trong thư mục `services/` và tạo ra bộ tài liệu Swagger cho từng service:
```bash
make swag

# (Tùy chọn) Nếu máy chưa cài Make nhưng xài Windows, có thể chạy trực tiếp:
# .\generate.ps1 swag
```

## 🚀 Khởi chạy dự án

Toàn bộ hệ thống được đóng gói và chạy thông qua Docker Compose. Hệ thống bao gồm các thành phần:
- Microservices: identity, storage, topics, learning, notifications, posts
- Database: `PostgreSQL`
- API Gateway: `Nginx`
- Giao diện Docs: `Swagger UI`

**Bước 1:** Đảm bảo bạn đã điền đầy đủ các biến môi trường trong các file `.env` (ở thư mục gốc và bên trong từng thư mục service).

**Bước 2:** Build và khởi chạy toàn bộ hệ thống bằng Docker Compose:
```bash
docker-compose up -d --build
```

**Bước 3:** Kiểm tra trạng thái các container:
```bash
docker-compose ps
```

## 📚 Truy cập dịch vụ

Sau khi khởi chạy thành công, bạn có thể truy cập:

- **API Gateway (Nginx):** `http://localhost:<NGINX_PORT>`
- **Swagger API Docs:** `http://localhost:<NGINX_PORT>/docs` (Hoặc port riêng của container docs nếu bạn có map port ra ngoài)

*(Lưu ý: Thay `<NGINX_PORT>` bằng port thực tế bạn cấu hình trong file `.env`)*

## 🛑 Dừng dự án
Để dừng và tắt toàn bộ các container:
```bash
docker-compose down
```
