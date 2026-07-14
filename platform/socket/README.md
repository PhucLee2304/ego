# Platform Socket

`platform/socket` là nền WebSocket dùng chung cho các service cần cập nhật realtime.

Package này xử lý các phần hạ tầng của kết nối:

- nâng cấp kết nối WebSocket
- lấy token từ `Authorization: Bearer ...`
- vòng đời kết nối
- ping/pong heartbeat
- định tuyến message theo `type`
- format response chung dạng `ack` / `error`

Các service vẫn tự xử lý nghiệp vụ của mình. Ví dụ, service `exams` có thể đăng ký handler
`attempt.answer.updated` để validate attempt và upsert answer.

## Route

Package nay chi cung cap `http.Handler`. Moi service tu mount route rieng de tranh dung nhau.

Vi du voi service `exams`:

- route trong service: `/ws/exams`
- route day du qua gateway cua exams: `/api/v1/ws/exams`

## Message Envelope

Client gửi lên server:

```json
{
  "type": "attempt.answer.updated",
  "requestId": "client-generated-id",
  "payload": {
    "attemptId": 12,
    "questionId": 123,
    "selectedOptionId": 456
  }
}
```

Server trả về khi xử lý thành công:

```json
{
  "type": "ack",
  "requestId": "client-generated-id",
  "success": true
}
```

Server trả về khi xử lý lỗi:

```json
{
  "type": "error",
  "requestId": "client-generated-id",
  "success": false,
  "code": "attempt_not_active",
  "message": "attempt is not active"
}
```

`ack` nghĩa là server đã chạy handler tương ứng thành công. Với các event cập nhật answer,
handler chỉ nên trả thành công sau khi upsert database thành công.
