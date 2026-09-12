package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/google/uuid"
	"os"
	"strconv"
	"strings"
	"time"
)

type ChatCall struct {
	ID           string `db:"id" json:"id"`
	Conversation string `db:"conversation_id" json:"conversationId"`
	Caller       string `db:"caller_id" json:"callerId"`
	Callee       string `db:"callee_id" json:"calleeId"`
	Kind         string `db:"kind" json:"kind"`
	State        string `db:"state" json:"state"`
	Offer        string `db:"offer" json:"offer"`
	Answer       string `db:"answer" json:"answer"`
	Created      string `db:"created_at" json:"createdAt"`
}
type ChatCallReq struct {
	Id        string `path:"id"`
	Call      string `path:"call,optional"`
	Action    string `json:"action,optional"`
	Kind      string `json:"kind,optional"`
	SDP       string `json:"sdp,optional"`
	ClientID  string `json:"clientId,optional"`
	Candidate string `json:"candidate,optional"`
	After     int64  `form:"after,optional"`
}
type CallSignal struct {
	ID        int64  `db:"id" json:"id"`
	Candidate string `db:"candidate" json:"candidate"`
}

func ChatCallCapabilities(ctx context.Context, s *svc.ServiceContext, id string) (map[string]any, error) {
	ent, err := loadChatEntitlement(ctx, s, id)
	if err != nil {
		return nil, err
	}
	p, err := resolveChatParticipant(ctx, s, ent, false)
	if err != nil {
		return nil, err
	}
	urls, secret := os.Getenv("CHAT_TURN_URLS"), os.Getenv("CHAT_TURN_SECRET")
	servers := []map[string]any{}
	enabled := urls != "" && secret != ""
	if enabled {
		username := fmt.Sprintf("%d:%s", time.Now().Add(2*time.Hour).Unix(), p.SenderID)
		mac := hmac.New(sha1.New, []byte(secret))
		_, _ = mac.Write([]byte(username))
		servers = append(servers, map[string]any{"urls": strings.Split(urls, ","), "username": username, "credential": base64.StdEncoding.EncodeToString(mac.Sum(nil))})
	}
	return map[string]any{"enabled": enabled, "iceServers": servers}, nil
}
func chatCallAuth(ctx context.Context, s *svc.ServiceContext, id string, send bool) (*chatParticipant, error) {
	ent, err := loadChatEntitlement(ctx, s, id)
	if err != nil {
		return nil, err
	}
	return resolveChatParticipant(ctx, s, ent, send)
}

const callColumns = "id,conversation_id,caller_id,callee_id,kind,state,offer,COALESCE(answer,'') answer,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at"

func expireChatCalls(ctx context.Context, s *svc.ServiceContext) error {
	_, err := s.DB.ExecCtx(ctx, `UPDATE chat_call SET state=IF(state='ringing','missed','ended'),active_key=NULL,ended_at=NOW() WHERE active_key IS NOT NULL AND ((state='ringing' AND created_at<NOW()-INTERVAL 60 SECOND) OR (state='active' AND (caller_seen<NOW()-INTERVAL 45 SECOND OR callee_seen<NOW()-INTERVAL 45 SECOND)) OR created_at<NOW()-INTERVAL 2 HOUR)`)
	return err
}
func ChatCallGet(ctx context.Context, s *svc.ServiceContext, req *ChatCallReq) (map[string]any, error) {
	p, err := chatCallAuth(ctx, s, req.Id, false)
	if err != nil {
		return nil, err
	}
	if err = expireChatCalls(ctx, s); err != nil {
		return nil, common.ErrSystem
	}
	var calls []ChatCall
	query := "SELECT " + callColumns + " FROM chat_call WHERE conversation_id=?"
	args := []any{req.Id}
	if req.Call != "" {
		query += " AND id=?"
		args = append(args, req.Call)
	} else {
		query += " AND active_key IS NOT NULL"
	}
	query += " ORDER BY created_at DESC LIMIT 1"
	if err = s.DB.QueryRowsCtx(ctx, &calls, query, args...); err != nil {
		return nil, common.ErrSystem
	}
	if len(calls) == 0 {
		return map[string]any{"call": nil, "signals": []CallSignal{}}, nil
	}
	c := calls[0]
	if p.SenderID != c.Caller && p.SenderID != c.Callee {
		return nil, common.ErrForbidden
	}
	var signals []CallSignal
	if err = s.DB.QueryRowsCtx(ctx, &signals, "SELECT id,candidate FROM chat_call_signal WHERE call_id=? AND sender_id<>? AND id>? ORDER BY id LIMIT 100", c.ID, p.SenderID, req.After); err != nil {
		return nil, common.ErrSystem
	}
	if signals == nil {
		signals = []CallSignal{}
	}
	return map[string]any{"call": c, "signals": signals}, nil
}
func validSDP(sdp string) bool {
	return len(sdp) > 0 && len(sdp) < 64000 && strings.HasPrefix(sdp, "v=0")
}
func ChatCallAction(ctx context.Context, s *svc.ServiceContext, req *ChatCallReq) (map[string]any, error) {
	requireSend := req.Action == "start" || req.Action == "accept"
	p, err := chatCallAuth(ctx, s, req.Id, requireSend)
	if err != nil {
		return nil, err
	}
	if err = expireChatCalls(ctx, s); err != nil {
		return nil, common.ErrSystem
	}
	if req.Action == "start" {
		if os.Getenv("CHAT_TURN_SECRET") == "" || os.Getenv("CHAT_TURN_URLS") == "" {
			return nil, common.NewBizError(50331, "通话服务暂不可用")
		}
		if (req.Kind != "audio" && req.Kind != "video") || !validSDP(req.SDP) {
			return nil, common.ErrParamInvalid
		}
		callID := req.ClientID
		if _, err = uuid.Parse(callID); err != nil {
			return nil, common.ErrParamInvalid
		}
		var existing []ChatCall
		if err = s.DB.QueryRowsCtx(ctx, &existing, "SELECT "+callColumns+" FROM chat_call WHERE id=?", callID); err != nil {
			return nil, common.ErrSystem
		}
		if len(existing) > 0 {
			if existing[0].Caller != p.SenderID || existing[0].Conversation != req.Id {
				return nil, common.ErrForbidden
			}
			return map[string]any{"call": existing[0]}, nil
		}
		_, err = s.DB.ExecCtx(ctx, "INSERT INTO chat_call(id,conversation_id,caller_id,callee_id,kind,offer,active_key) VALUES(?,?,?,?,?,?,?)", callID, req.Id, p.SenderID, p.ReceiverID, req.Kind, req.SDP, req.Id)
		if err != nil {
			return nil, common.NewBizError(40931, "当前已有通话，请稍后重试")
		}
		req.Call = callID
	} else {
		var c ChatCall
		if err = s.DB.QueryRowCtx(ctx, &c, "SELECT "+callColumns+" FROM chat_call WHERE id=? AND conversation_id=?", req.Call, req.Id); err != nil {
			return nil, common.NewBizError(40431, "通话不存在")
		}
		if c.Caller != p.SenderID && c.Callee != p.SenderID {
			return nil, common.ErrForbidden
		}
		switch req.Action {
		case "accept":
			if c.Callee != p.SenderID || !validSDP(req.SDP) {
				return nil, common.ErrForbidden
			}
			if c.State == "active" {
				return map[string]any{"call": c}, nil
			}
			if c.State != "ringing" {
				return nil, common.NewBizError(40931, "通话已结束")
			}
			_, err = s.DB.ExecCtx(ctx, "UPDATE chat_call SET state='active',answer=?,callee_seen=NOW(),caller_seen=NOW(),updated_at=NOW() WHERE id=? AND state='ringing'", req.SDP, c.ID)
		case "reject", "end":
			if req.Action == "reject" && c.Callee != p.SenderID {
				return nil, common.ErrForbidden
			}
			state := "ended"
			if req.Action == "reject" {
				state = "rejected"
			}
			_, err = s.DB.ExecCtx(ctx, "UPDATE chat_call SET state=?,active_key=NULL,ended_at=NOW(),updated_at=NOW() WHERE id=? AND active_key IS NOT NULL", state, c.ID)
		case "heartbeat":
			column := "callee_seen"
			if c.Caller == p.SenderID {
				column = "caller_seen"
			}
			_, err = s.DB.ExecCtx(ctx, "UPDATE chat_call SET "+column+"=NOW() WHERE id=? AND active_key IS NOT NULL", c.ID)
		case "candidate":
			if c.State != "active" && c.State != "ringing" {
				return nil, common.NewBizError(40931, "通话已结束")
			}
			if req.ClientID == "" || len(req.ClientID) > 64 || len(req.Candidate) > 8192 || !json.Valid([]byte(req.Candidate)) {
				return nil, common.ErrParamInvalid
			}
			var count int64
			if err = s.DB.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM chat_call_signal WHERE call_id=? AND sender_id=?", c.ID, p.SenderID); err != nil {
				return nil, common.ErrSystem
			}
			if count >= 100 {
				return nil, common.NewBizError(42931, "通话连接信息过多")
			}
			_, err = s.DB.ExecCtx(ctx, "INSERT IGNORE INTO chat_call_signal(call_id,sender_id,client_id,candidate) VALUES(?,?,?,?)", c.ID, p.SenderID, req.ClientID, req.Candidate)
		default:
			return nil, common.ErrParamInvalid
		}
		if err != nil {
			return nil, common.ErrSystem
		}
	}
	return ChatCallGet(ctx, s, req)
}

func ChatIncomingCall(ctx context.Context, s *svc.ServiceContext) (map[string]any, error) {
	identity := ""
	if id := middleware.MasterIDFromCtx(ctx); id > 0 {
		identity = "m_" + strconv.FormatInt(id, 10)
	} else {
		user, err := authenticatedUserID(ctx)
		if err != nil {
			return nil, err
		}
		identity = "u_" + user
	}
	if err := expireChatCalls(ctx, s); err != nil {
		return nil, common.ErrSystem
	}
	var calls []ChatCall
	if err := s.DB.QueryRowsCtx(ctx, &calls, "SELECT "+callColumns+" FROM chat_call WHERE callee_id=? AND active_key IS NOT NULL AND state='ringing' ORDER BY created_at LIMIT 1", identity); err != nil {
		return nil, common.ErrSystem
	}
	if len(calls) == 0 {
		return map[string]any{"call": nil}, nil
	}
	return map[string]any{"call": calls[0]}, nil
}
