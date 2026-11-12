package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	storage "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/services/posts/internal/model"

	"github.com/google/uuid"
)

// videoProcessTimeout giới hạn tải, chuyển mã và upload một video, để MinIO/ffmpeg treo không làm kẹt worker.
const videoProcessTimeout = 15 * time.Minute

// probeResult là phần JSON cần dùng trong output của ffprobe (stream video đầu tiên).
type probeResult struct {
	Streams []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Duration string `json:"duration"`
	} `json:"streams"`
}

// ProcessVideos do worker videos gọi định kỳ: lấy tối đa limit video đang PROCESSING và chuyển mã lần lượt từng cái
// (xem processVideo). Thành công thì processVideo tự lưu READY; lỗi thì tăng số lần thử, đủ 3 lần thì FAILED
// và không bị lấy lại nữa. Lỗi của từng video chỉ log, không dừng cả lượt.
func (s *service) ProcessVideos(ctx context.Context, limit int) error {
	items, err := s.repo.ListVideoAttachmentsForProcessing(ctx, limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		// Tăng số lần thử trước khi xử lý; thành công thì giá trị này được lưu cùng kết quả trong processVideo.
		item.ProcessingAttempts++
		if err := s.processVideo(ctx, item); err != nil {
			logger.Log.Error().Err(err).Uint("attachment", item.ID).Msg("[WORKER] Video processing failed")
			// Lỗi: lưu lại số lần thử để lượt sau thử tiếp; đủ 3 lần thì chuyển FAILED để dừng hẳn.
			if item.ProcessingAttempts >= 3 {
				item.ProcessingStatus = model.ProcessingStatusFailed
			}
			if err := s.repo.SaveAttachment(ctx, item); err != nil {
				logger.Log.Error().Err(err).Uint("attachment", item.ID).Msg("[WORKER] Failed to save video processing attempt")
			}
		}
	}
	return nil
}

// processVideo chuyển mã một video:
//  1. tải file gốc từ storage về thư mục tạm;
//  2. ffprobe đọc kích thước, thời lượng; ffmpeg chuyển sang mp4 H.264 tối đa 720p (+faststart để phát ngay trên web)
//     và cắt khung hình đầu làm thumbnail;
//  3. upload video mới và thumbnail vào cùng folder với file gốc;
//  4. cập nhật bản ghi đính kèm sang file mới, trạng thái READY;
//  5. cuối cùng mới xóa file gốc.
//
// Lỗi ở bất kỳ bước nào thì xóa những file đã upload ở bước 3, bản ghi giữ nguyên để lần sau thử lại.
// err là named return để defer biết hàm kết thúc lỗi hay không.
func (s *service) processVideo(ctx context.Context, item *model.PostAttachment) (err error) {
	// Giới hạn thời gian cho cả quy trình; hết giờ thì hủy tải/upload và kill ffmpeg (exec.CommandContext).
	ctx, cancel := context.WithTimeout(ctx, videoProcessTimeout)
	defer cancel()

	// Thư mục tạm chứa file gốc, video đã chuyển mã và thumbnail; xóa khi xong dù thành công hay lỗi.
	dir, err := os.MkdirTemp("", "post-video-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	input, output, thumbnail := filepath.Join(dir, "input"), filepath.Join(dir, "output.mp4"), filepath.Join(dir, "thumbnail.jpg")

	// Tải file gốc qua URL công khai về input.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %s", response.Status)
	}
	file, err := os.Create(input)
	if err != nil {
		return err
	}
	if _, err = file.ReadFrom(response.Body); err != nil {
		file.Close()
		return err
	}
	file.Close()

	// Đọc width/height/duration của stream video đầu tiên; không có stream video thì không phải video hợp lệ.
	probeOutput, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,duration", "-of", "json", input).Output()
	if err != nil {
		return err
	}
	var probe probeResult
	if err := json.Unmarshal(probeOutput, &probe); err != nil || len(probe.Streams) == 0 {
		return fmt.Errorf("invalid video metadata")
	}

	// Chuyển mã: giữ tỉ lệ, chiều cao tối đa 720 (video nhỏ hơn thì giữ nguyên), H.264 + AAC,
	// +faststart đưa metadata lên đầu file để trình duyệt phát được khi chưa tải hết.
	if err := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", input, "-vf", "scale=-2:min(720\\,ih)", "-c:v", "libx264", "-preset", "fast", "-crf", "23", "-c:a", "aac", "-movflags", "+faststart", output).Run(); err != nil {
		return err
	}
	// Thumbnail: lấy khung hình đầu tiên của video đã chuyển mã.
	if err := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "0", "-i", output, "-frames:v", "1", "-q:v", "3", thumbnail).Run(); err != nil {
		return err
	}
	// Dung lượng video mới (thường khác file gốc) để lưu vào bản ghi.
	sizeInfo, err := os.Stat(output)
	if err != nil {
		return err
	}

	// Video đã xử lý nằm cùng folder với file gốc (posts/community hoặc posts/classrooms), tên mới ngẫu nhiên.
	parts := strings.Split(strings.Trim(item.ObjectKey, "/"), "/")
	folder := slices.Clip(parts[:len(parts)-1])
	videoName, thumbName := uuid.NewString()+".mp4", uuid.NewString()+".jpg"
	// Lỗi sau khi đã upload thì xóa file vừa upload, để lần thử lại không để rác trên storage.
	var uploaded []string
	defer func() {
		if err == nil {
			return
		}
		for _, name := range uploaded {
			_, _ = s.storageClient.DeleteObject(context.WithoutCancel(ctx), &storage.DeleteObjectRequest{Folders: folder, FileName: name})
		}
	}()

	// Upload video mới rồi thumbnail; ghi lại tên từng file đã upload để defer ở trên dọn khi lỗi.
	videoURL, err := s.uploadProcessed(ctx, folder, videoName, "video/mp4", output)
	if err != nil {
		return err
	}
	uploaded = append(uploaded, videoName)
	thumbURL, err := s.uploadProcessed(ctx, folder, thumbName, "image/jpeg", thumbnail)
	if err != nil {
		return err
	}
	uploaded = append(uploaded, thumbName)

	// Trỏ bản ghi sang video mới kèm metadata và đánh dấu READY; nhớ key cũ để xóa file gốc sau khi lưu.
	originalKey := item.ObjectKey
	duration, _ := strconv.ParseFloat(probe.Streams[0].Duration, 64)
	item.URL, item.ObjectKey, item.ContentType = videoURL, strings.Join(append(folder, videoName), "/"), "video/mp4"
	item.Size, item.Width, item.Height, item.DurationSeconds, item.ThumbnailURL = sizeInfo.Size(), &probe.Streams[0].Width, &probe.Streams[0].Height, &duration, &thumbURL
	item.ProcessingStatus = model.ProcessingStatusReady
	if err = s.repo.SaveAttachment(ctx, item); err != nil {
		return err
	}
	// Chỉ xóa file gốc khi bản ghi đã trỏ sang video mới.
	if originalKey != item.ObjectKey && len(parts) > 1 {
		_, _ = s.storageClient.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: folder, FileName: parts[len(parts)-1]})
	}
	return nil
}

// uploadProcessed upload một file trên đĩa lên storage và trả URL công khai:
// xin presigned URL nội bộ (Internal: backend gọi thẳng MinIO trong mạng docker), PUT file kèm Content-Type
// và Content-Length, rồi lấy URL công khai để lưu vào bản ghi.
func (s *service) uploadProcessed(ctx context.Context, folders []string, name, contentType, path string) (string, error) {
	presigned, err := s.storageClient.GeneratePresignedUploadURL(ctx, &storage.GeneratePresignedUploadURLRequest{Folders: folders, FileName: name, ContentType: contentType, Internal: true})
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presigned.Url, file)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload status %s", resp.Status)
	}
	public, err := s.storageClient.GetPublicURL(ctx, &storage.GetPublicURLRequest{Folders: folders, FileName: name})
	if err != nil {
		return "", err
	}
	return public.Url, nil
}
