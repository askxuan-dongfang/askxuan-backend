package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/google/uuid"
)

const MaxChatAttachmentBytes int64 = 20 << 20

func chatFilesDir() string {
	if dir := os.Getenv("CHAT_FILES_DIR"); dir != "" {
		return dir
	}
	return "/var/lib/askxuan/chat"
}

type attachmentRow struct {
	Id           string  `db:"id"`
	Conversation string  `db:"conversation_id"`
	Owner        string  `db:"owner_id"`
	Name         string  `db:"name"`
	ContentType  string  `db:"content_type"`
	Size         int64   `db:"file_size"`
	Duration     float64 `db:"duration"`
	Expired      int64   `db:"expired"`
}

func findAttachment(ctx context.Context, s *svc.ServiceContext, id string) (*attachmentRow, error) {
	var a attachmentRow
	err := s.DB.QueryRowCtx(ctx, &a, "SELECT id,conversation_id,owner_id,name,content_type,file_size,duration,(created_at<NOW()-INTERVAL 6 DAY) expired FROM chat_attachment WHERE id=?", id)
	return &a, err
}
func (a *attachmentRow) public() types.ChatAttachment {
	return types.ChatAttachment{Id: a.Id, Name: a.Name, ContentType: a.ContentType, Size: a.Size, Duration: a.Duration}
}
func validateChatPayload(ctx context.Context, s *svc.ServiceContext, req *types.ChatMessageSendReq, owner string) (string, string, string, error) {
	kind := req.Kind
	if kind == "" {
		kind = "text"
	}
	content := strings.TrimSpace(req.Content)
	if len([]rune(content)) > maxChatMessageLength {
		return "", "", "", common.ErrParamInvalid
	}
	if kind == "text" {
		if content == "" || req.AttachmentId != "" {
			return "", "", "", common.ErrParamInvalid
		}
		return content, kind, "{}", nil
	}
	if kind != "image" && kind != "audio" && kind != "file" && kind != "video" {
		return "", "", "", common.ErrParamInvalid
	}
	a, err := findAttachment(ctx, s, req.AttachmentId)
	if err != nil || a.Conversation != req.Id || a.Owner != owner {
		return "", "", "", common.ErrForbidden
	}
	if a.Expired != 0 {
		return "", "", "", common.NewBizError(40033, "附件草稿已过期，请重新选择上传")
	}
	if kind != "file" && !strings.HasPrefix(a.ContentType, kind+"/") {
		return "", "", "", common.ErrParamInvalid
	}
	payload, _ := json.Marshal(a.public())
	label := map[string]string{"image": "图片", "audio": "语音", "video": "视频", "file": "文件"}[kind]
	if content == "" {
		content = "[" + label + "]"
		if kind == "file" {
			content += " " + a.Name
		}
	}
	return content, kind, string(payload), nil
}

// ChatUpload never exposes private uploads through the public media catalog.
func ChatUpload(w http.ResponseWriter, r *http.Request, s *svc.ServiceContext, id string) {
	ent, err := loadChatEntitlement(r.Context(), s, id)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	p, err := resolveChatParticipant(r.Context(), s, ent, true)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxChatAttachmentBytes+(1<<20))
	if err = r.ParseMultipartForm(1 << 20); err != nil {
		common.JsonError(w, common.NewBizError(40030, "文件不能超过 20 MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		common.JsonError(w, common.ErrParamInvalid)
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > MaxChatAttachmentBytes {
		common.JsonError(w, common.ErrParamInvalid)
		return
	}
	var used int64
	if err = s.DB.QueryRowCtx(r.Context(), &used, "SELECT COALESCE(SUM(file_size),0) FROM chat_attachment WHERE owner_id=? AND created_at>NOW()-INTERVAL 1 DAY", p.SenderID); err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	if used+header.Size > 200<<20 {
		common.JsonError(w, common.NewBizError(42930, "今日附件上传量已达上限"))
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	detected := http.DetectContentType(head)
	declared, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))
	contentType, err := chatContentType(detected, declared, header.Filename, head)
	if err != nil {
		common.JsonError(w, common.NewBizError(40031, err.Error()))
		return
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if len([]rune(name)) > 180 {
		name = string([]rune(name)[:180])
	}
	duration, _ := strconv.ParseFloat(r.FormValue("duration"), 64)
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0 || duration > 600 {
		duration = 0
	}
	a := &attachmentRow{Id: uuid.NewString(), Conversation: id, Owner: p.SenderID, Name: name, ContentType: contentType, Size: header.Size, Duration: duration}
	if err = os.MkdirAll(chatFilesDir(), 0700); err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	target := filepath.Join(chatFilesDir(), a.Id)
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(target)
		}
	}()
	_, err = out.Write(head)
	if err == nil {
		_, err = io.Copy(out, file)
	}
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	_, err = s.DB.ExecCtx(r.Context(), "INSERT INTO chat_attachment(id,conversation_id,owner_id,name,content_type,file_size,duration) VALUES(?,?,?,?,?,?,?)", a.Id, a.Conversation, a.Owner, a.Name, a.ContentType, a.Size, a.Duration)
	if err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	ok = true
	common.Ok(w, a.public())
}
func chatContentType(detected, declared, name string, head []byte) (string, error) {
	switch detected {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "audio/mpeg", "audio/wave", "audio/midi", "audio/aiff", "audio/ogg", "video/mp4", "video/webm", "application/ogg":
		if (detected == "video/webm" && declared == "audio/webm") || (detected == "video/mp4" && declared == "audio/mp4") {
			return declared, nil
		}
		return detected, nil
	}
	// Some browsers label MP4 audio as application/octet-stream; validate its container.
	if len(head) > 12 && string(head[4:8]) == "ftyp" {
		if strings.HasPrefix(declared, "audio/") {
			return "audio/mp4", nil
		}
		return "video/mp4", nil
	}
	ext := strings.ToLower(filepath.Ext(name))
	if strings.HasPrefix(detected, "text/plain") && ext == ".txt" {
		return "text/plain", nil
	}
	if detected == "application/zip" && (ext == ".zip" || ext == ".docx" || ext == ".xlsx" || ext == ".pptx") {
		return "application/octet-stream", nil
	}
	return "", fmt.Errorf("支持图片、音视频、PDF、文本和 Office 文件")
}
func ChatDownload(w http.ResponseWriter, r *http.Request, s *svc.ServiceContext, conversation, id string) {
	ent, err := loadChatEntitlement(r.Context(), s, conversation)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	p, err := resolveChatParticipant(r.Context(), s, ent, false)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	a, err := findAttachment(r.Context(), s, id)
	if err != nil || a.Conversation != conversation {
		common.JsonError(w, common.ErrForbidden)
		return
	}
	if a.Owner != p.SenderID {
		var count int64
		err = s.DB.QueryRowCtx(r.Context(), &count, "SELECT COUNT(*) FROM booking_chat_message WHERE booking_id=? AND status='sent' AND JSON_UNQUOTE(JSON_EXTRACT(attachment_json,'$.id'))=?", conversation, id)
		if err != nil || count == 0 {
			common.JsonError(w, common.ErrForbidden)
			return
		}
	}
	if _, err = uuid.Parse(a.Id); err != nil {
		common.JsonError(w, common.ErrForbidden)
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Name}))
	http.ServeFile(w, r, filepath.Join(chatFilesDir(), a.Id))
}
