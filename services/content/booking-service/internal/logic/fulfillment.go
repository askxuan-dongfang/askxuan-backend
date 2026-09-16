package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ReceiptFile struct {
	Id          string `db:"id" json:"id"`
	BookingId   string `db:"booking_no" json:"-"`
	Owner       string `db:"owner_id" json:"-"`
	ReceiptId   string `db:"receipt_id" json:"receiptId"`
	Name        string `db:"name" json:"name"`
	ContentType string `db:"content_type" json:"contentType"`
	Size        int64  `db:"file_size" json:"size"`
	SHA256      string `db:"sha256" json:"sha256"`
}
type Receipt struct {
	Id           string        `db:"id" json:"id"`
	Summary      string        `db:"summary" json:"summary"`
	OperatorId   string        `db:"operator_id" json:"operatorId"`
	OperatorType string        `db:"operator_type" json:"operatorType"`
	Digest       string        `db:"digest" json:"digest"`
	CreatedAt    string        `db:"created_at" json:"createdAt"`
	Files        []ReceiptFile `db:"-" json:"files"`
}
type ReceiptRequest struct {
	Summary string   `json:"summary"`
	FileIds []string `json:"fileIds"`
}

const receiptFileColumns = "id,booking_no,owner_id,receipt_id,name,content_type,file_size,sha256"

// BookingAccess always checks both role and the booking's actual tenant/owner.
func BookingAccess(ctx context.Context, s *svc.ServiceContext, id string, providerOnly bool) (*model.Booking, string, string, error) {
	b, err := s.BookingModel.FindOne(ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, "", "", common.ErrBookingNotFound
		}
		return nil, "", "", common.ErrSystem
	}
	actor := strconv.FormatInt(middleware.UserIDFromCtx(ctx), 10)
	if actor == "0" {
		return nil, "", "", common.ErrUnauthorized
	}
	for _, role := range middleware.RolesFromCtx(ctx) {
		switch role {
		case "customer":
			if !providerOnly && actor == b.UserId {
				return b, actor, model.OperatorTypeUser, nil
			}
		case "temple_admin":
			if code := middleware.TempleCodeFromCtx(ctx); code != "" && code == b.TempleId {
				return b, actor, model.OperatorTypeTempleAdmin, nil
			}
		case "master":
			if middleware.MasterIDFromCtx(ctx) == 0 {
				return nil, "", "", common.ErrForbidden
			}
			m, lookupErr := s.MasterClient.GetByID(ctx, middleware.MasterIDFromCtx(ctx))
			if lookupErr != nil {
				return nil, "", "", common.ErrDependencyUnavailable
			}
			if m.Code != "" && m.Code == b.MasterId {
				return b, actor, model.OperatorTypeMaster, nil
			}
		case "platform_super":
			if !providerOnly {
				return b, actor, "platform_super", nil
			}
		}
	}
	return nil, "", "", common.ErrForbidden
}

func Fulfillment(ctx context.Context, s *svc.ServiceContext, id string) (map[string]any, error) {
	b, _, _, err := BookingAccess(ctx, s, id, false)
	if err != nil {
		return nil, err
	}
	logs, err := s.StatusLogModel.FindByBookingId(ctx, id)
	if err != nil {
		logx.WithContext(ctx).Errorf("load booking fulfillment: %v", err)
		return nil, common.ErrSystem
	}
	receipts := []Receipt{}
	err = s.DB.QueryRowsPartialCtx(ctx, &receipts, "SELECT id,summary,operator_id,operator_type,digest,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM booking_receipt WHERE booking_no=? ORDER BY sequence", id)
	if err != nil {
		logx.WithContext(ctx).Errorf("load booking fulfillment: %v", err)
		return nil, common.ErrSystem
	}
	for i := range receipts {
		receipts[i].Files = []ReceiptFile{}
		if err = s.DB.QueryRowsCtx(ctx, &receipts[i].Files, "SELECT "+receiptFileColumns+" FROM booking_receipt_file WHERE booking_no=? AND receipt_id=? ORDER BY id", id, receipts[i].Id); err != nil {
			logx.WithContext(ctx).Errorf("load booking fulfillment: %v", err)
			return nil, common.ErrSystem
		}
	}
	return map[string]any{"status": b.Status, "logs": logs, "receipts": receipts}, nil
}

func lockBooking(ctx context.Context, tx sqlx.Session, id string) (string, error) {
	var status string
	err := tx.QueryRowCtx(ctx, &status, "SELECT status FROM booking WHERE booking_no=? FOR UPDATE", id)
	return status, err
}
func appendTransition(ctx context.Context, tx sqlx.Session, id, from, to, actor, kind, remark string) error {
	if !model.CanTransit(from, to) {
		return common.ErrBookingStatusInvalid
	}
	if _, err := tx.ExecCtx(ctx, "UPDATE booking SET status=? WHERE booking_no=? AND status=?", to, id, from); err != nil {
		return err
	}
	res, err := tx.ExecCtx(ctx, "INSERT INTO booking_status_log(booking_id,from_status,to_status,operator_id,operator_type,remark,create_time) VALUES(?,?,?,?,?,?,NOW())", id, from, to, actor, kind, remark)
	if err != nil {
		return err
	}
	logID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	// A committed transition always has a durable notification, even during MQ outages.
	_, err = tx.ExecCtx(ctx, `INSERT INTO event_outbox(event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at)
 SELECT CONCAT('booking:fulfillment:',?), 'booking',booking_no,CONCAT('booking.',?),'booking.events','',
 JSON_OBJECT('bookingId',booking_no,'userId',CAST(user_id AS CHAR),'templeId',temple_code,'templeName',temple_name,'masterId',master_code,'masterName',master_name,'serviceName',service_name,'bookingDate',CAST(booking_date AS CHAR),'serviceFee',service_fee,'meritMoney',merit_money,'totalFee',total_fee,'action',?,'time',DATE_FORMAT(NOW(),'%Y-%m-%d %H:%i:%s')),
 'pending',0,NOW(),NOW(),NOW() FROM booking WHERE booking_no=?`, logID, to, to, id)
	return err
}

func SubmitReceipt(ctx context.Context, s *svc.ServiceContext, id string, req ReceiptRequest) (map[string]any, error) {
	_, actor, kind, err := BookingAccess(ctx, s, id, true)
	if err != nil {
		return nil, err
	}
	req.Summary = strings.TrimSpace(req.Summary)
	if len([]rune(req.Summary)) < 5 || len([]rune(req.Summary)) > 2000 || len(req.FileIds) < 1 || len(req.FileIds) > 8 {
		return nil, common.NewBizError(40034, "请填写 5–2000 字执行说明，并上传 1–8 个图片或视频回执")
	}
	receiptID := uuid.NewString()
	files := []ReceiptFile{}
	seen := map[string]bool{}
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		status, e := lockBooking(ctx, tx, id)
		if e != nil {
			return e
		}
		if status != model.StatusInProgress {
			return common.ErrBookingStatusInvalid
		}
		for _, fileID := range req.FileIds {
			if seen[fileID] {
				return common.ErrParamInvalid
			}
			seen[fileID] = true
			var f ReceiptFile
			if e = tx.QueryRowCtx(ctx, &f, "SELECT "+receiptFileColumns+" FROM booking_receipt_file WHERE id=? FOR UPDATE", fileID); e != nil {
				return common.ErrParamInvalid
			}
			if f.BookingId != id || f.Owner != actor || f.ReceiptId != "" {
				return common.ErrForbidden
			}
			f.ReceiptId = receiptID
			files = append(files, f)
		}
		// The digest describes this immutable receipt and the SHA256 of its media.
		payload, _ := json.Marshal(struct {
			Booking, ID, Summary, Actor, Kind string
			Files                             []ReceiptFile
		}{id, receiptID, req.Summary, actor, kind, files})
		digest := sha256.Sum256(payload)
		if _, e = tx.ExecCtx(ctx, "INSERT INTO booking_receipt(id,booking_no,summary,operator_id,operator_type,digest) VALUES(?,?,?,?,?,?)", receiptID, id, req.Summary, actor, kind, hex.EncodeToString(digest[:])); e != nil {
			return e
		}
		for _, f := range files {
			if _, e = tx.ExecCtx(ctx, "UPDATE booking_receipt_file SET receipt_id=? WHERE id=?", receiptID, f.Id); e != nil {
				return e
			}
		}
		return appendTransition(ctx, tx, id, status, model.StatusPendingReceipt, actor, kind, "提交履约回执 "+receiptID)
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "status": model.StatusPendingReceipt, "receiptId": receiptID}, nil
}
func ReceiptDecision(ctx context.Context, s *svc.ServiceContext, id, remark string, accept bool) (map[string]any, error) {
	_, actor, kind, err := BookingAccess(ctx, s, id, false)
	if err != nil {
		return nil, err
	}
	if kind != model.OperatorTypeUser {
		return nil, common.ErrForbidden
	}
	remark = strings.TrimSpace(remark)
	if !accept && (len([]rune(remark)) < 5 || len([]rune(remark)) > 200) {
		return nil, common.NewBizError(40034, "请填写 5–200 字需要补充说明的问题")
	}
	target := model.StatusInProgress
	if accept {
		target = model.StatusCompleted
		remark = "信众确认履约回执"
	}
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		status, e := lockBooking(ctx, tx, id)
		if e != nil {
			return e
		}
		if status != model.StatusPendingReceipt {
			return common.ErrBookingStatusInvalid
		}
		var count int
		if e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM booking_receipt WHERE booking_no=?", id); e != nil {
			return e
		}
		if count == 0 {
			return common.ErrBookingStatusInvalid
		}
		return appendTransition(ctx, tx, id, status, target, actor, kind, remark)
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "status": target}, nil
}
func receiptDir() string { return filepath.Join(chatFilesDir(), "booking-receipts") }
func ReceiptUpload(w http.ResponseWriter, r *http.Request, s *svc.ServiceContext, id string) {
	b, actor, _, err := BookingAccess(r.Context(), s, id, true)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	if b.Status != model.StatusInProgress {
		common.JsonError(w, common.ErrBookingStatusInvalid)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxChatAttachmentBytes+(1<<20))
	if err = r.ParseMultipartForm(1 << 20); err != nil {
		common.JsonError(w, common.NewBizError(40030, "每个回执文件不能超过 20 MB"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, h, err := r.FormFile("file")
	if err != nil {
		common.JsonError(w, common.ErrParamInvalid)
		return
	}
	defer file.Close()
	if h.Size <= 0 || h.Size > MaxChatAttachmentBytes {
		common.JsonError(w, common.ErrParamInvalid)
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	contentType := http.DetectContentType(head)
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm":
	default:
		common.JsonError(w, common.NewBizError(40031, "请上传 JPG、PNG、WebP 图片或 MP4、WebM 视频"))
		return
	}
	f := ReceiptFile{Id: uuid.NewString(), BookingId: id, Owner: actor, Name: filepath.Base(strings.ReplaceAll(h.Filename, "\\", "/")), ContentType: contentType, Size: h.Size}
	if len([]rune(f.Name)) > 180 {
		f.Name = string([]rune(f.Name)[:180])
	}
	if err = os.MkdirAll(receiptDir(), 0700); err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	path := filepath.Join(receiptDir(), f.Id)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hash), io.MultiReader(strings.NewReader(string(head)), file))
	closeErr := out.Close()
	if err != nil || closeErr != nil || size != h.Size {
		common.JsonError(w, common.ErrSystem)
		return
	}
	f.SHA256 = hex.EncodeToString(hash.Sum(nil))
	err = s.DB.TransactCtx(r.Context(), func(ctx context.Context, tx sqlx.Session) error {
		status, e := lockBooking(ctx, tx, id)
		if e != nil {
			return e
		}
		if status != model.StatusInProgress {
			return common.ErrBookingStatusInvalid
		}
		var used int64
		if e = tx.QueryRowCtx(ctx, &used, "SELECT COALESCE(SUM(file_size),0) FROM booking_receipt_file WHERE booking_no=?", id); e != nil {
			return e
		}
		if used+f.Size > 200<<20 {
			return common.NewBizError(42930, "本订单回执文件已达 200 MB 上限")
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO booking_receipt_file(id,booking_no,owner_id,name,content_type,file_size,sha256) VALUES(?,?,?,?,?,?,?)", f.Id, id, actor, f.Name, f.ContentType, f.Size, f.SHA256)
		return e
	})
	if err != nil {
		common.JsonError(w, err)
		return
	}
	committed = true
	common.Ok(w, f)
}
func ReceiptDownload(w http.ResponseWriter, r *http.Request, s *svc.ServiceContext, id, fileID string) {
	_, actor, kind, err := BookingAccess(r.Context(), s, id, false)
	if err != nil {
		common.JsonError(w, err)
		return
	}
	var f ReceiptFile
	if err = s.DB.QueryRowCtx(r.Context(), &f, "SELECT "+receiptFileColumns+" FROM booking_receipt_file WHERE id=? AND booking_no=?", fileID, id); err != nil {
		common.JsonError(w, common.NewBizError(40434, "回执文件不存在"))
		return
	}
	if f.ReceiptId == "" && (kind == model.OperatorTypeUser || actor != f.Owner) {
		common.JsonError(w, common.ErrForbidden)
		return
	}
	file, err := os.Open(filepath.Join(receiptDir(), f.Id))
	if err != nil {
		common.JsonError(w, common.NewBizError(40434, "回执文件不存在"))
		return
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil || hex.EncodeToString(hash.Sum(nil)) != f.SHA256 {
		common.JsonError(w, common.NewBizError(50034, "回执文件校验失败，请联系平台"))
		return
	}
	_, _ = file.Seek(0, io.SeekStart)
	stat, err := file.Stat()
	if err != nil {
		common.JsonError(w, common.ErrSystem)
		return
	}
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": f.Name}))
	w.Header().Set("X-Receipt-SHA256", f.SHA256)
	http.ServeContent(w, r, f.Name, stat.ModTime(), file)
}

// TransitionWithAudit couples state, capacity release and actor audit in one transaction.
func TransitionWithAudit(ctx context.Context, s *svc.ServiceContext, id, target, actor, kind, remark string) error {
	if len([]rune(remark)) > 255 {
		return common.NewBizError(40034, "操作备注不能超过 255 字")
	}
	if target == model.StatusCompleted || target == model.StatusPendingReceipt {
		return common.NewBizError(40034, "请先提交图片或视频回执，由信众确认完成")
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		status, err := lockBooking(ctx, tx, id)
		if err != nil {
			return err
		}
		if !model.CanTransit(status, target) {
			return common.ErrBookingStatusInvalid
		}
		if target == model.StatusCancelled {
			var reserved int
			if err = tx.QueryRowCtx(ctx, &reserved, "SELECT slot_reserved FROM booking WHERE booking_no=?", id); err != nil {
				return err
			}
			if reserved == 1 {
				if _, err = tx.ExecCtx(ctx, `UPDATE booking_slot_inventory i JOIN booking b ON i.temple_code=b.temple_code AND i.service_code=b.service_code AND i.booking_date=b.booking_date AND i.slot_code=b.slot_code SET i.reserved_count=GREATEST(i.reserved_count-1,0) WHERE b.booking_no=?`, id); err != nil {
					return err
				}
				if _, err = tx.ExecCtx(ctx, "UPDATE booking SET slot_reserved=0 WHERE booking_no=?", id); err != nil {
					return err
				}
			}
		}
		return appendTransition(ctx, tx, id, status, target, actor, kind, remark)
	})
}
