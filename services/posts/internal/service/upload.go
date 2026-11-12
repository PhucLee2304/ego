package service

import (
	"context"
	"strings"
	"time"

	storage "ego/api/gen/go/storage"
	"ego/platform/httpx"
	"ego/platform/storagekey"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
)

var imageContentTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
var videoContentTypes = map[string]bool{"video/mp4": true, "video/quicktime": true}
var fileContentTypes = map[string]bool{
	"application/pdf":    true,
	"text/plain":         true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
}

func (s *service) UploadLimits() *dto.UploadLimitsResponse {
	return dto.ToUploadLimitsResponse(s.cfg, imageContentTypes, fileContentTypes, videoContentTypes)
}

// CreateUploadIntent là bước 1 của luồng upload đính kèm (trước khi có bài viết):
//   - xác định loại đính kèm (ảnh/video/file) từ content-type và kiểm tra dung lượng khai báo;
//   - nếu upload cho bài trong lớp thì kiểm tra quyền truy cập lớp;
//   - xin storage một presigned URL để client PUT file thẳng lên MinIO (không đi qua backend);
//   - lưu upload intent PENDING có hạn dùng, để bước CompleteUpload đối chiếu file thật với thông tin đã khai báo.
//
// Intent quá hạn mà không complete sẽ bị CleanupExpiredUploads dọn cả bản ghi lẫn file.
func (s *service) CreateUploadIntent(ctx context.Context, userID string, req dto.CreateUploadIntentRequest) (*dto.UploadIntentResponse, *httpx.HTTPError) {
	// Chuẩn hóa content-type: bỏ tham số (vd "; charset=utf-8"), khoảng trắng, chữ hoa.
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(req.ContentType, ";")[0]))

	// Phân loại đính kèm theo whitelist content-type; mỗi loại có giới hạn dung lượng riêng.
	// Không thuộc whitelist nào thì từ chối.
	mediaType := model.AttachmentTypeFile
	maxSize := s.cfg.FileMaxBytes
	if imageContentTypes[contentType] {
		mediaType = model.AttachmentTypeImage
		maxSize = s.cfg.ImageMaxBytes
	} else if videoContentTypes[contentType] {
		mediaType, maxSize = model.AttachmentTypeVideo, s.cfg.VideoMaxBytes
	} else if !fileContentTypes[contentType] {
		return nil, ErrInvalidAttachment
	}

	// Kiểm tra sớm dung lượng client khai báo; dung lượng thật được kiểm tra lại ở CompleteUpload.
	if req.Size > maxSize {
		return nil, ErrInvalidAttachment
	}

	// Chọn folder lưu trữ. Upload cho bài trong lớp: chỉ giáo viên được đăng bài trong lớp nên cũng chỉ giáo viên
	// được upload, và lớp phải còn hoạt động (giống CreateClassroomPost).
	folder := "posts/community"
	if req.ClassroomID != nil {
		access, err := s.ensureBelongsToClassroom(ctx, *req.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to create upload intent")
		}
		if !access.IsTeacher {
			return nil, ErrForbidden
		}
		if !access.Active {
			return nil, ErrClassroomReadOnly
		}
		folder = "posts/classrooms"
	}

	// Sinh tên file duy nhất trên storage (giữ đuôi file gốc) và xin presigned URL gắn đúng content-type,
	// để client PUT thẳng lên MinIO.
	fileName := storagekey.FileName(req.OriginalName)
	response, err := s.storageClient.GeneratePresignedUploadURL(ctx, &storage.GeneratePresignedUploadURLRequest{
		Folders:     strings.Split(folder, "/"),
		FileName:    fileName,
		ContentType: contentType,
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create upload intent")
	}

	// Lưu intent PENDING kèm thông tin đã khai báo và hạn dùng; CompleteUpload sẽ đối chiếu với file thật.
	intent := &model.PostUploadIntent{
		UserID:       userID,
		Folder:       folder,
		FileName:     fileName,
		OriginalName: req.OriginalName,
		ContentType:  contentType,
		DeclaredSize: req.Size,
		MediaType:    mediaType,
		Status:       model.UploadStatusPending,
		ExpiresAt:    time.Now().Add(s.cfg.UploadIntentTTL),
	}
	if err := s.repo.CreateUploadIntent(ctx, intent); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create upload intent")
	}
	return &dto.UploadIntentResponse{
		ID:          intent.ID,
		UploadURL:   response.Url,
		ContentType: contentType,
		ExpiresAt:   intent.ExpiresAt,
	}, nil
}

// CompleteUpload là bước 2 của luồng upload, client gọi sau khi đã PUT file lên MinIO qua presigned URL:
//   - đối chiếu file thật trên storage (dung lượng, content-type) với thông tin đã khai báo ở CreateUploadIntent;
//   - tạo bản ghi đính kèm chưa gắn bài (post_id NULL); video ở trạng thái PROCESSING để worker chuyển mã,
//     loại khác READY ngay;
//   - đánh dấu intent COMPLETED, trả về đính kèm để client gửi ID khi tạo/sửa bài.
//
// Gọi lại nhiều lần (idempotent) vẫn trả cùng một đính kèm. Các bước gọi gRPC tới storage chạy ngoài transaction,
// rồi mới khóa intent và kiểm tra lại, để không giữ khóa dòng trong lúc gọi mạng.
func (s *service) CompleteUpload(ctx context.Context, userID string, id uint) (*dto.AttachmentResponse, *httpx.HTTPError) {
	// Lấy intent và chỉ cho chủ intent complete.
	intent, err := s.repo.GetUploadIntent(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to complete upload")
	}
	if intent.UserID != userID {
		return nil, ErrForbidden
	}

	// Đã complete trước đó (client gọi lại do retry/mạng chập chờn): trả lại đính kèm cũ, không tạo mới.
	if intent.Status == model.UploadStatusCompleted && intent.Attachment != nil {
		return dto.ToAttachmentResponse(intent.Attachment), nil
	}

	// Intent đã hủy hoặc quá hạn thì không complete được nữa (file có thể đã bị dọn).
	if intent.Status != model.UploadStatusPending || time.Now().After(intent.ExpiresAt) {
		return nil, ErrUploadExpired
	}

	// Đọc metadata file thật trên storage; client chưa upload thì lời gọi này lỗi.
	folders := strings.Split(intent.Folder, "/")
	meta, err := s.storageClient.GetObjectMetadata(ctx, &storage.GetObjectMetadataRequest{Folders: folders, FileName: intent.FileName})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to complete upload")
	}

	// Kiểm tra lại với file thật, vì client có thể khai báo một đằng mà upload một nẻo:
	// dung lượng phải > 0, không vượt giới hạn theo loại, đúng bằng dung lượng đã khai báo,
	// và content-type khớp với lúc xin presigned URL.
	maxSize := s.cfg.FileMaxBytes
	switch intent.MediaType {
	case model.AttachmentTypeImage:
		maxSize = s.cfg.ImageMaxBytes
	case model.AttachmentTypeVideo:
		maxSize = s.cfg.VideoMaxBytes
	}
	if meta.Size <= 0 || meta.Size > maxSize || meta.Size != intent.DeclaredSize || !strings.EqualFold(strings.Split(meta.ContentType, ";")[0], intent.ContentType) {
		return nil, ErrInvalidAttachment
	}

	// Lấy URL công khai để client hiển thị đính kèm.
	publicURL, err := s.storageClient.GetPublicURL(ctx, &storage.GetPublicURLRequest{Folders: folders, FileName: intent.FileName})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to complete upload")
	}

	// Dựng bản ghi đính kèm (chưa gắn bài). Video cần worker chuyển mã nên để PROCESSING, loại khác dùng được ngay.
	status := model.ProcessingStatusReady
	if intent.MediaType == model.AttachmentTypeVideo {
		status = model.ProcessingStatusProcessing
	}
	attachment := &model.PostAttachment{
		UserID:           userID,
		URL:              publicURL.Url,
		ObjectKey:        intent.Folder + "/" + intent.FileName,
		MediaType:        intent.MediaType,
		OriginalName:     intent.OriginalName,
		ContentType:      intent.ContentType,
		Size:             meta.Size,
		ProcessingStatus: status,
	}

	// Khóa intent (SELECT ... FOR UPDATE) rồi kiểm tra lại trạng thái, vì trong lúc gọi storage ở trên
	// có thể đã có request complete khác hoặc worker dọn intent quá hạn.
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		lockedIntent, err := repo.GetUploadIntentForUpdate(ctx, id)
		if err != nil {
			return err
		}
		// Request khác đã complete trong lúc kiểm tra storage: trả lại đính kèm đó.
		if lockedIntent.Status == model.UploadStatusCompleted && lockedIntent.AttachmentID != nil {
			attachment, err = repo.GetAttachment(ctx, *lockedIntent.AttachmentID)
			return err
		}
		// Đã bị hủy/dọn trong lúc đó.
		if lockedIntent.Status != model.UploadStatusPending {
			return ErrUploadExpired
		}
		// Tạo đính kèm và đánh dấu intent COMPLETED trỏ tới nó, cùng một transaction.
		if err := repo.CreateAttachment(ctx, attachment); err != nil {
			return err
		}
		lockedIntent.Status, lockedIntent.AttachmentID = model.UploadStatusCompleted, &attachment.ID
		return repo.SaveUploadIntent(ctx, lockedIntent)
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to complete upload")
	}
	return dto.ToAttachmentResponse(attachment), nil
}

// CancelUpload hủy một upload chưa complete (client bỏ file khỏi bài đang soạn hoặc upload thất bại):
//   - chỉ chủ intent được hủy;
//   - chỉ intent PENDING mới bị hủy; intent đã COMPLETED/CANCELLED thì bỏ qua và trả thành công (idempotent),
//     đính kèm đã complete không bị đụng tới;
//   - đánh dấu CANCELLED trong transaction, commit xong mới xóa file trên storage (side effect sau commit).
//
// Xóa file lỗi thì intent vẫn CANCELLED và trả lỗi; file sót lại không được CleanupExpiredUploads dọn
// (job chỉ quét intent PENDING).
func (s *service) CancelUpload(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	// intent và cancelled được gán trong transaction để dùng tiếp sau khi commit.
	var intent *model.PostUploadIntent
	cancelled := false
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// Khóa intent để không chạy song song với CompleteUpload của cùng intent.
		var err error
		intent, err = repo.GetUploadIntentForUpdate(ctx, id)
		if err != nil {
			return err
		}
		// Chỉ chủ intent được hủy.
		if intent.UserID != userID {
			return ErrForbidden
		}
		// Đã complete hoặc đã hủy: không làm gì, coi như hủy thành công.
		if intent.Status != model.UploadStatusPending {
			return nil
		}
		// Đánh dấu CANCELLED; từ đây CompleteUpload sẽ từ chối intent này.
		intent.Status, cancelled = model.UploadStatusCancelled, true
		return repo.SaveUploadIntent(ctx, intent)
	})

	// Lỗi, hoặc không có gì để hủy thì dừng (FromError(nil) trả nil).
	if err != nil || !cancelled {
		return httpx.FromError(err, "[ERROR] Failed to cancel upload")
	}

	// Commit xong mới xóa file client đã PUT lên storage (MinIO xóa object không tồn tại vẫn thành công).
	_, err = s.storageClient.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName})
	return httpx.FromError(err, "[ERROR] Failed to cancel upload")
}

// CleanupExpiredUploads do worker uploads gọi định kỳ, dọn file rác trên storage theo hai nhóm (mỗi nhóm tối đa limit bản ghi):
//   - intent PENDING đã quá hạn: client xin URL nhưng không complete (bỏ ngang, upload lỗi) → xóa file, đánh dấu CANCELLED;
//   - đính kèm orphan (post_id NULL) đã tạo quá UploadIntentTTL: complete xong nhưng không gắn vào bài nào,
//     hoặc bị gỡ khỏi bài khi sửa bài → xóa file, xóa hẳn bản ghi.
//
// Đính kèm của bài đã xóa (xóa mềm) vẫn giữ post_id nên không bao giờ bị dọn: xóa bài không xóa file.
// Mỗi file gọi storage một lần (storage không có xóa hàng loạt); file xóa lỗi thì bỏ qua để lần chạy sau thử lại.
// DB chỉ cập nhật một câu cho mỗi nhóm, gồm các bản ghi đã xóa file thành công; trả về số bản ghi đã dọn.
func (s *service) CleanupExpiredUploads(ctx context.Context, limit int) (int, error) {
	// Nhóm 1: intent PENDING quá hạn.
	intents, err := s.repo.ListExpiredUploadIntents(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	// Xóa file trước; lỗi thì giữ PENDING để lần sau quét lại.
	var cancelIDs []uint
	for _, intent := range intents {
		if _, err := s.storageClient.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName}); err != nil {
			continue
		}
		cancelIDs = append(cancelIDs, intent.ID)
	}
	// File đã xóa mới đánh dấu CANCELLED (một câu UPDATE), để intent không bị quét lại và không complete được nữa.
	cancelled, err := s.repo.CancelUploadIntents(ctx, cancelIDs)
	if err != nil {
		return 0, err
	}

	// Nhóm 2: đính kèm orphan (post_id NULL) tạo trước mốc now - UploadIntentTTL.
	// Đính kèm nào cũng orphan trong lúc người dùng soạn bài: complete xong là có bản ghi, nhưng phải bấm đăng
	// thì mới được gắn post_id. Nên chỉ dọn cái được tạo cách đây hơn UploadIntentTTL (hiện là 60 phút)
	// mà vẫn chưa gắn bài, coi như người dùng đã bỏ bài; cái mới complete thì để yên chờ được gắn vào bài.
	orphans, err := s.repo.ListOrphanAttachments(ctx, time.Now().Add(-s.cfg.UploadIntentTTL), limit)
	if err != nil {
		return int(cancelled), err
	}
	var deleteIDs []uint
	for _, attachment := range orphans {
		// Tách ObjectKey "posts/community/<file>" thành folder + tên file; key không có folder là dữ liệu lạ, bỏ qua.
		parts := strings.Split(strings.Trim(attachment.ObjectKey, "/"), "/")
		if len(parts) < 2 {
			continue
		}
		// Xóa file trước; lỗi thì giữ bản ghi để lần sau thử lại.
		if _, err := s.storageClient.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: parts[:len(parts)-1], FileName: parts[len(parts)-1]}); err != nil {
			continue
		}
		deleteIDs = append(deleteIDs, attachment.ID)
	}
	// Xóa hẳn (một câu DELETE) các bản ghi đã xóa file, để intent trỏ tới được SET NULL và không quét lại.
	deleted, err := s.repo.DeleteOrphanAttachmentsPermanently(ctx, deleteIDs)
	if err != nil {
		return int(cancelled), err
	}
	return int(cancelled + deleted), nil
}
