# Quy ước code backend (`ego/`)

Chuẩn gốc là `services/identity`. Gặp chỗ nào chưa rõ thì mở identity ra xem và làm theo.

**Phạm vi áp dụng:**
- Bắt buộc cho service mới và cho mọi service đang được refactor (`learning` — gộp từ exams + classrooms — đã theo chuẩn này).
- Code cũ ở các service khác (topics, storage, notifications, posts) chỉ phải theo khi đụng tới phần đó; không refactor hàng loạt.

---

## 1. Layout một service

```
services/<svc>/
├── cmd/server/main.go        # wiring bằng tay, không chứa logic nghiệp vụ
├── config/config.go          # AppConfig + LoadAppConfig, getEnv(key, fallback)
├── database/connection.go    # Connect + Migrate
├── internal/
│   ├── model/                # GORM struct + hằng enum
│   ├── dto/                  # JSON request/response + mapper ToXxx
│   ├── repository/           # truy vấn GORM
│   ├── service/              # nghiệp vụ
│   └── handler/              # HTTP
├── rpc/                      # gRPC server cho service khác (mỏng)
├── workers/                  # vòng lặp ticker, chỉ gọi service
├── integration/              # test //go:build integration
├── Dockerfile
└── .env                      # gitignored
```

### 1.1. Service nhiều domain

Service có nhiều domain (ví dụ `learning` = exam, attempt, classroom, room, assignment…) **vẫn dùng đúng layout trên**: mỗi layer là **một package duy nhất**, không chia package theo module, không có `internal/app`. Domain được tách bằng **file**:

```
internal/
├── model/       exam.go, attempt.go, classroom.go, room.go, assignment.go, index.go…
├── dto/         exam.go, attempt.go, classroom.go, room.go, gradebook.go…
├── repository/  repository.go (interface + struct + NewRepository + Transaction), exam.go, attempt.go, classroom.go…
├── service/     service.go (interface + struct + New + sentinel), exam.go, submit.go, member.go, room.go…
├── handler/     handler.go (interface + New + RegisterRoutes), exam.go, attempt.go, classroom.go…
└── socket/      nếu có WebSocket
```

- Interface `Service`/`Repository` nhóm method theo domain bằng comment `// Domain`.
- Mỗi domain vẫn "sở hữu" bảng của nó để chia file cho rõ (query bảng attempts nằm ở `repository/attempt.go`…), nhưng các domain gọi nhau trực tiếp vì cùng package.
- Không gọi nhau qua gRPC/HTTP trong cùng tiến trình.

---

## 2. Mỗi layer: một interface, một struct, một constructor

**File định nghĩa** của layer (`handler.go`, `service.go`, `repository.go`, `rpc/server.go`) chứa:
- interface export, các method nhóm theo domain bằng comment `// Domain`;
- struct private;
- constructor trả về **interface**;
- sentinel error;
- các interface hẹp tới phụ thuộc bên ngoài: chỉ khai báo method thực sự dùng, để test bằng fake.

Method được tách theo domain ra `<domain>.go` trong cùng package. Mẫu: `services/identity/internal/service/service.go` (sentinel, `Service`, `StorageClient` hẹp, `service`, `New`).

| Layer | Constructor |
|---|---|
| handler | `func New(s service.Service) Handler` |
| service | `func New(...) Service` |
| repository | `func NewRepository(db *gorm.DB) Repository` |
| rpc | `func New(svc service.Service) <pkg>.<Xxx>ServiceServer` |

**Kích thước file:**
- Nên ≤ 300 dòng.
- Quá 400 dòng thì **bắt buộc** tách theo domain. Không được có file "god" kiểu `service.go` 1700 dòng.
- Không tính file sinh tự động (`docs/`, `api/gen/`).

---

## 3. Lỗi

- **Method phục vụ HTTP** trả `*httpx.HTTPError`:
  - tạo bằng `httpx.NewError(status, "[TAG] Message", cause)`;
  - hoặc `httpx.FromError(err, msg)`, hàm này map `gorm.ErrRecordNotFound` → 404, gRPC code → HTTP, còn lại → 500.
- **Method cho rpc, worker, middleware hoặc domain khác gọi nội bộ** trả `error`, dùng sentinel `var ErrXxx = errors.New(...)` và so sánh bằng `errors.Is`.
- Không để `gorm.ErrRecordNotFound` lọt ra khỏi service. Đổi nó thành sentinel của domain.
- Không có adapter riêng kiểu `HTTPService` chỉ để đổi lỗi. Service trả thẳng `*httpx.HTTPError` cho method HTTP.
- Tag message: `[ERROR]`, `[UNAUTHORIZED]`, `[FORBIDDEN]`, `[NOT_FOUND]`, `[CONFLICT]`…

---

## 4. Handler và route

Mẫu: `RegisterRoutes` trong `services/identity/internal/handler/handler.go`, `UpdateMe` (kèm godoc) trong `services/identity/internal/handler/user.go`.

- Route dùng pattern Go 1.22 dạng `"METHOD /path"`, trên mux `api` đã mount `/api/v1/` bằng `http.StripPrefix`. Route cần đăng nhập bọc `mw.Handle(...)`, cần role thì thêm `rm.RequireRole("admin")(...)`.
- User hiện tại lấy bằng `jwt.GetUserID(r.Context())`. Body decode bằng `httpx.DecodeJSON`, lỗi decode trả `httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err)`. Lỗi ghi bằng `httpx.Error`, kết quả ghi bằng `httpx.JSON`.
- Path param đọc bằng `httpx.ParsePathID`. Query đọc bằng `httpx.DecodeQuery`.
- Phân trang dùng `httpx.PaginationQuery` → `query.Normalize()` → `httpx.ToPaginatedResponse`.
- Handler không chứa logic nghiệp vụ. Mỗi method gồm decode, gọi service, ghi response.
- Mỗi handler có Swagger godoc đầy đủ: `@Summary`, `@Tags`, `@Accept`, `@Produce`, `@Security BearerAuth` (nếu cần đăng nhập), `@Success`, `@Failure {object} httpx.HTTPError`, `@Router`.

- `main.go` khai báo `@title`, `@BasePath /<svc>/api/v1` và `@securityDefinitions.apikey BearerAuth`.

---

## 5. Model và DTO

- `model`: chỉ chứa GORM struct (embed `gorm.Model`), tag gorm, kiểu enum có hằng. Không chứa logic gọi DB.
- `dto`:
  - JSON camelCase;
  - request: giữ quy ước sẵn có của service (identity dùng `XxxBody`, `learning` giữ `XxxRequest` từ exams/classrooms); service mới dùng `XxxBody`. Không đổi tên type chỉ để cho đồng bộ;
  - response đặt tên `XxxResponse` hoặc tên thực thể;
  - validate bằng tag `validate:"required,notblank"`;
  - field PATCH tùy chọn dùng con trỏ.
- Mapper model → dto là hàm `ToXxx` trong `dto`.
- **Không bao giờ đổi JSON tag** của API đang có client dùng, kể cả khi đổi tên type.

---

## 6. Repository và transaction

- Mọi truy vấn đều dùng `r.db.WithContext(ctx)`.
- Phân trang nhận `limit, offset int32`, trả `([]*model.X, int64, error)`.
- Raw SQL (partial index, trigger, constraint) đặt thành hằng trong `model/index.go` và chạy trong `Migrate`.
- **Transaction:** một kiểu duy nhất, như identity (kể cả service nhiều domain): `Repository.Transaction(ctx, fn func(Repository) error)` bọc `platformdb.WithTx` và truyền `NewRepository(tx)` cho `fn`. Mẫu: `services/identity/internal/repository/repository.go`.
  - Vì chỉ có một `Repository`, một transaction ghi được bảng của mọi domain (vd attempt + submission).
  - Cần gọi logic của service (domain khác) bên trong tx thì dùng bản sao service gắn tx-repo, `s.withRepo(repo)`. `Transaction` lồng bên trong thành SAVEPOINT. Mẫu: `withRepo` trong `services/learning/internal/service/service.go`, dùng ở `assignment.go` (`createPrivateExam`).
  - Không dùng rollback bù (xóa bản ghi vừa tạo, khôi phục field) khi mọi thứ nằm trong một DB: gom vào một tx.
  - Side effect ra ngoài (push WebSocket, LiveKit, gọi gRPC) phải chạy **sau khi commit**: gom event trong tx, publish sau. Mẫu: `withRepoDeferringEvents` + `publishPendingEvents` trong cùng file.

---

## 7. gRPC (`rpc/`)

- `server.go` chứa struct embed `Unimplemented<Xxx>ServiceServer` và một field `service`. Method tách theo domain ra file riêng.
- Method rpc **mỏng**: gọi service, map sentinel sang `platformRpc.PublicError(codes.X, "[ERROR] ...")`, map model sang proto. Lỗi khác trả nguyên để `platformRpc.ErrorInterceptor` log và che thành `Internal`.
- Client gRPC tạo trong `main.go` hoặc `app`, có `platformRpc.TimeoutInterceptor(5*time.Second)`. Consumer tới identity dùng `IDENTITY_SERVICE_ADDR`.
- Proto đặt ở `api/proto/<domain>/*.proto`, sinh bằng `make protoc`.

---

## 8. Thông báo và outbox

- Event thông báo được ghi vào bảng outbox **cùng transaction** với thay đổi nghiệp vụ.
- Worker gửi sang notifications với idempotency key bằng event key.
- Không gọi notifications RPC trực tiếp trong transaction nghiệp vụ.
- Outbox chỉ dùng khi phải nói chuyện với **service khác** (tách DB). Hai phần trong cùng một service/DB thì ghi cùng transaction, không dùng outbox hay reconciler.

---

## 9. Config, khởi động, log

- `config.LoadAppConfig()` đọc env bằng `getEnv(key, fallback)`. Fallback là giá trị docker network (`identity:50052`…).
- Secret (JWT, mật khẩu, API key) không có fallback.
- Config dùng chung nhiều service đặt ở `platform/config`.
- Thứ tự trong `main.go` (hoặc `app.Run`):
  1. logger
  2. config
  3. DB và Migrate
  4. gRPC client
  5. repository
  6. service
  7. middleware
  8. handler
  9. worker
  10. HTTP (`/health`, `/docs/`, `/api/v1/`) và gRPC server
  11. graceful shutdown (HTTP 10s, rồi `GracefulStop`)
- Log dùng `logger.Log` (zerolog), message có tag `[CONFIG]`, `[STARTUP]`, `[SHUTDOWN]`, `[CRITICAL]`, `[WORKER]`. Không dùng `log.Println`.

---

## 10. Test

- **Unit test** đặt cạnh code, cùng package, viết table-driven với `t.Run`.
- **Fake** viết tay, embed interface để chỉ cài các method cần. Không dùng thư viện mock. Mẫu: `fakeUserRepository` trong `services/identity/internal/service/auth_test.go`.

- Phụ thuộc không dùng tới thì truyền `nil`.
- **Integration test** đặt trong `services/<svc>/integration/`:
  - có `//go:build integration` và `package integration_test`;
  - DB lấy từ env `<SVC>_TEST_DSN`;
  - mỗi test tự reset schema rồi gọi `Migrate`;
  - helper gọi `t.Helper()`.
- Thêm DSN mới vào `docker-compose.test.yml` và `scripts/test.{sh,ps1}`.
