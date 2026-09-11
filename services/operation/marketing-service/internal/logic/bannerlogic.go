package logic

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Published schedules always use China Standard Time, independent of server TZ.
var marketingZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func parseMarketingTime(value string, end bool) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		t, err := time.ParseInLocation(layout, value, marketingZone)
		if err == nil {
			if end && layout == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Second)
			}
			return t, nil
		}
	}
	return time.Time{}, errors.New("时间格式应为 YYYY-MM-DD HH:mm:ss")
}
func inTimeRange(start, end string) bool { return inTimeRangeAt(start, end, time.Now()) }
func inTimeRangeAt(start, end string, now time.Time) bool {
	if start != "" {
		t, err := parseMarketingTime(start, false)
		if err != nil || now.Before(t) {
			return false
		}
	}
	if end != "" {
		t, err := parseMarketingTime(end, true)
		if err != nil || now.After(t) {
			return false
		}
	}
	return true
}
func validMediaURL(value string) bool {
	if strings.ContainsAny(value, "\\\r\n") {
		return false
	}
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (u.Scheme == "https" && u.Host != "" && u.User == nil) || (u.Scheme == "" && u.Host == "" && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//"))
}

var targetID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validBannerTarget(kind, value string) bool {
	switch kind {
	case "temple", "master", "product", "service", "activity", "reward":
		return targetID.MatchString(value)
	case "ai", "diy":
		return value == ""
	case "ad_landing":
		return value == "/c/ai" || value == "/c/diy" || value == "/c/shop" || value == "/c/rewards" || value == "/c/temples" || value == "/c/masters" || value == "/c/services"
	}
	return false
}
func validateBanner(b model.Banner) error {
	invalid := func(s string) error { return common.NewBizError(40001, s) }
	if strings.TrimSpace(b.Title) == "" || utf8.RuneCountInString(b.Title) > 64 {
		return invalid("标题需为 1–64 个字符")
	}
	if b.Placement != model.BannerPlacementHome {
		return invalid("不支持的展示位置")
	}
	if b.Status != model.StatusDraft && b.Status != model.StatusEnabled && b.Status != model.StatusDisabled {
		return invalid("无效的发布状态")
	}
	if b.Sort < 0 {
		return invalid("排序不能小于 0")
	}
	if len(b.ImageUrl) > 255 || len(b.LinkValue) > 255 {
		return invalid("图片或跳转地址过长")
	}
	if b.ImageUrl != "" && !validMediaURL(b.ImageUrl) {
		return invalid("请使用站内图片路径或 HTTPS 图片")
	}
	var start, end time.Time
	var err error
	if b.StartTime != "" {
		start, err = parseMarketingTime(b.StartTime, false)
		if err != nil {
			return invalid(err.Error())
		}
	}
	if b.EndTime != "" {
		end, err = parseMarketingTime(b.EndTime, true)
		if err != nil {
			return invalid(err.Error())
		}
	}
	if !start.IsZero() && !end.IsZero() && !end.After(start) {
		return invalid("结束时间应晚于开始时间")
	}
	if b.Status == model.StatusEnabled {
		if b.ImageUrl == "" || !validBannerTarget(b.LinkType, b.LinkValue) {
			return invalid("上架前请补全图片与有效跳转目标")
		}
		if !end.IsZero() && time.Now().After(end) {
			return invalid("投放时间已结束，请调整后再上架")
		}
	}
	return nil
}
func bannerToType(b model.Banner) types.Banner {
	return types.Banner{Id: b.Id, Title: b.Title, Placement: b.Placement, ImageUrl: b.ImageUrl, LinkType: b.LinkType, LinkValue: b.LinkValue, Sort: b.Sort, Status: b.Status, StartTime: b.StartTime, EndTime: b.EndTime, CreatedAt: b.CreatedAt}
}
func bannerList(ctx context.Context, req *types.BannerListReq, public bool) (*types.BannerListResp, error) {
	page, size := req.Page, req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	placement, status, at := req.Placement, req.Status, ""
	if public {
		status = model.StatusEnabled
		if placement == "" {
			placement = model.BannerPlacementHome
		}
		at = time.Now().In(marketingZone).Format("2006-01-02 15:04:05")
	}
	list, total, err := model.ListBanners(ctx, status, placement, page, size, at)
	if err != nil {
		return nil, err
	}
	out := make([]types.Banner, 0, len(list))
	for _, b := range list {
		out = append(out, bannerToType(b))
	}
	return &types.BannerListResp{Total: total, List: out, Page: page, Size: size}, nil
}

type CustomerBannerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerBannerListLogic(ctx context.Context, s *svc.ServiceContext) *CustomerBannerListLogic {
	return &CustomerBannerListLogic{logx.WithContext(ctx), ctx, s}
}
func (l *CustomerBannerListLogic) BannerList(r *types.BannerListReq) (*types.BannerListResp, error) {
	return bannerList(l.ctx, r, true)
}

type AdminBannerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerListLogic(ctx context.Context, s *svc.ServiceContext) *AdminBannerListLogic {
	return &AdminBannerListLogic{logx.WithContext(ctx), ctx, s}
}
func (l *AdminBannerListLogic) List(r *types.BannerListReq) (*types.BannerListResp, error) {
	return bannerList(l.ctx, r, false)
}

type AdminBannerCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerCreateLogic(ctx context.Context, s *svc.ServiceContext) *AdminBannerCreateLogic {
	return &AdminBannerCreateLogic{logx.WithContext(ctx), ctx, s}
}
func (l *AdminBannerCreateLogic) Create(r *types.BannerCreateReq) (*types.IdResp, error) {
	placement := r.Placement
	if placement == "" {
		placement = model.BannerPlacementHome
	}
	b := model.Banner{Title: strings.TrimSpace(r.Title), Placement: placement, ImageUrl: r.ImageUrl, LinkType: r.LinkType, LinkValue: r.LinkValue, Sort: r.Sort, Status: model.StatusDraft, StartTime: r.StartTime, EndTime: r.EndTime}
	if err := validateBanner(b); err != nil {
		return nil, err
	}
	b, err := model.InsertBanner(l.ctx, b)
	if err != nil {
		return nil, err
	}
	return &types.IdResp{Id: b.Id}, nil
}

type AdminBannerUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerUpdateLogic(ctx context.Context, s *svc.ServiceContext) *AdminBannerUpdateLogic {
	return &AdminBannerUpdateLogic{logx.WithContext(ctx), ctx, s}
}
func (l *AdminBannerUpdateLogic) Update(r *types.BannerUpdateReq) (*types.IdResp, error) {
	b, err := model.FindBanner(l.ctx, r.Id)
	if errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.NewBizError(40404, "广告不存在")
	}
	if err != nil {
		return nil, err
	}
	if r.Title != nil {
		b.Title = *r.Title
	}
	if r.Placement != nil {
		b.Placement = *r.Placement
	}
	if r.ImageUrl != nil {
		b.ImageUrl = *r.ImageUrl
	}
	if r.LinkType != nil {
		b.LinkType = *r.LinkType
	}
	if r.LinkValue != nil {
		b.LinkValue = *r.LinkValue
	}
	if r.Sort != nil {
		b.Sort = *r.Sort
	}
	if r.Status != nil {
		b.Status = *r.Status
	}
	if r.StartTime != nil {
		b.StartTime = *r.StartTime
	}
	if r.EndTime != nil {
		b.EndTime = *r.EndTime
	}
	// Operators must still be able to take malformed historical content offline.
	onlyDisable := r.Status != nil && *r.Status == model.StatusDisabled && r.Title == nil && r.Placement == nil && r.ImageUrl == nil && r.LinkType == nil && r.LinkValue == nil && r.Sort == nil && r.StartTime == nil && r.EndTime == nil
	if !onlyDisable {
		if err = validateBanner(b); err != nil {
			return nil, err
		}
	}
	_, err = model.UpdateBanner(l.ctx, r.Id, *r)
	if err != nil {
		return nil, err
	}
	return &types.IdResp{Id: r.Id}, nil
}
