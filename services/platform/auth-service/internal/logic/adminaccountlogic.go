package logic

import (
	"context"
	"errors"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 账号管理 ============

// AdminAccountListLogic 账号列表
type AdminAccountListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountListLogic {
	return &AdminAccountListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountListLogic) AdminAccountList(req *types.AdminAccountListReq) (*types.AdminAccountListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	list, total, err := l.svcCtx.AdminAccountModel.FindList(l.ctx, req.Keyword, req.Status, page, size)
	if err != nil {
		l.Errorf("查询账号列表失败: %v", err)
		return nil, common.ErrSystem
	}

	// 批量查询角色名称
	roleMap := make(map[int64]string)
	for _, a := range list {
		if _, ok := roleMap[a.RoleId]; !ok {
			if r, err := l.svcCtx.RoleModel.FindByID(l.ctx, a.RoleId); err == nil {
				roleMap[a.RoleId] = r.Name
			}
		}
	}

	out := make([]types.AdminAccount, 0, len(list))
	for _, a := range list {
		out = append(out, types.AdminAccount{
			Id:            a.Id,
			Account:       a.Account,
			Name:          a.Name,
			RoleId:        a.RoleId,
			RoleName:      roleMap[a.RoleId],
			TempleId:      a.TempleId,
			MasterId:      a.MasterId,
			ShopId:        a.ShopId,
			Status:        a.Status,
			LastLoginTime: a.LastLoginTime,
			CreateTime:    a.CreateTime,
		})
	}

	return &types.AdminAccountListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminAccountCreateLogic 创建账号
type AdminAccountCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountCreateLogic {
	return &AdminAccountCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountCreateLogic) AdminAccountCreate(req *types.AdminAccountCreateReq) (*types.AdminAccountCreateResp, error) {
	if req.Account == "" || req.Password == "" || req.Name == "" || req.RoleId == 0 {
		return nil, common.ErrParamMissing
	}
	binding, err := validateAdminAccountBinding(l.ctx, l.svcCtx, req.RoleId, req.TempleId, req.MasterId, req.ShopId)
	if err != nil {
		return nil, err
	}

	// 账号去重
	exist, err := l.svcCtx.AdminAccountModel.FindByAccount(l.ctx, req.Account)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		l.Errorf("查询账号失败 account=%s: %v", req.Account, err)
		return nil, common.ErrSystem
	}
	if exist != nil {
		return nil, common.ErrUserAlreadyExists
	}

	status, err := initialAdminAccountStatus(l.ctx, l.svcCtx, binding.templeId, binding.templeAdmin)
	if err != nil {
		return nil, err
	}

	// MVP-1 明文存储密码。账号与寺院管理员关系必须在同一事务写入。
	account := &model.AdminAccount{
		Account:  req.Account,
		Password: req.Password,
		Name:     req.Name,
		RoleId:   req.RoleId,
		TempleId: binding.templeId,
		MasterId: binding.masterId,
		ShopId:   binding.shopId,
		Status:   status,
	}
	id, err := insertAdminAccountWithBinding(l.ctx, l.svcCtx.DB, account, binding.templeAdmin)
	if err != nil {
		l.Errorf("创建账号失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.AdminAccountCreateResp{Id: id}, nil
}

// AdminAccountUpdateLogic 更新账号
type AdminAccountUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountUpdateLogic {
	return &AdminAccountUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountUpdateLogic) AdminAccountUpdate(req *types.AdminAccountUpdateReq) (*types.AdminAccount, error) {
	// 先查询存在
	exist, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	// 合并更新字段（请求中非零值/非空字段覆盖原值）
	updated := &model.AdminAccount{
		Id:       exist.Id,
		Name:     exist.Name,
		RoleId:   exist.RoleId,
		TempleId: exist.TempleId,
		MasterId: exist.MasterId,
		ShopId:   exist.ShopId,
	}
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.RoleId != 0 {
		updated.RoleId = req.RoleId
	}
	if req.RoleId != 0 && req.RoleId != exist.RoleId {
		updated.TempleId = req.TempleId
		updated.MasterId = req.MasterId
		updated.ShopId = req.ShopId
	} else {
		if req.TempleId != "" {
			updated.TempleId = req.TempleId
		}
		if req.MasterId != "" {
			updated.MasterId = req.MasterId
		}
		if req.ShopId != 0 {
			updated.ShopId = req.ShopId
		}
	}
	binding, err := validateAdminAccountBinding(l.ctx, l.svcCtx, updated.RoleId, updated.TempleId, updated.MasterId, updated.ShopId)
	if err != nil {
		return nil, err
	}
	updated.TempleId = binding.templeId
	updated.MasterId = binding.masterId
	updated.ShopId = binding.shopId
	updated.Status = exist.Status
	if binding.templeAdmin {
		allowedStatus, statusErr := initialAdminAccountStatus(l.ctx, l.svcCtx, binding.templeId, true)
		if statusErr != nil {
			return nil, statusErr
		}
		if allowedStatus == model.AccountStatusDisabled {
			updated.Status = model.AccountStatusDisabled
		}
	}

	if err := updateAdminAccountWithBinding(l.ctx, l.svcCtx.DB, updated, binding.templeAdmin); err != nil {
		l.Errorf("更新账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	// 查询角色名
	roleName := ""
	if r, err := l.svcCtx.RoleModel.FindByID(l.ctx, updated.RoleId); err == nil {
		roleName = r.Name
	}

	return &types.AdminAccount{
		Id:            updated.Id,
		Account:       exist.Account,
		Name:          updated.Name,
		RoleId:        updated.RoleId,
		RoleName:      roleName,
		TempleId:      updated.TempleId,
		MasterId:      updated.MasterId,
		ShopId:        updated.ShopId,
		Status:        updated.Status,
		LastLoginTime: exist.LastLoginTime,
		CreateTime:    exist.CreateTime,
	}, nil
}

// AdminAccountStatusLogic 启停账号
type AdminAccountStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountStatusLogic {
	return &AdminAccountStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountStatusLogic) AdminAccountStatus(req *types.AdminAccountStatusReq) (*types.AdminAccountStatusResp, error) {
	// 校验状态值
	if req.Status != model.AccountStatusEnabled && req.Status != model.AccountStatusDisabled {
		return nil, common.ErrParam
	}

	// 校验账号存在
	account, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}
	role, roleErr := l.svcCtx.RoleModel.FindByID(l.ctx, account.RoleId)
	if roleErr != nil {
		return nil, common.ErrSystem
	}
	if req.Status == model.AccountStatusEnabled && role.Code == model.RoleCodeTempleAdmin {
		status, err := templeStatus(l.ctx, l.svcCtx, account.TempleId)
		if err != nil {
			return nil, err
		}
		if !canEnableTempleAccount(status) {
			return nil, common.ErrStatusInvalid
		}
	}

	if err := l.svcCtx.AdminAccountModel.UpdateStatus(l.ctx, req.Id, req.Status); err != nil {
		l.Errorf("更新账号状态失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	return &types.AdminAccountStatusResp{
		Id:     req.Id,
		Status: req.Status,
	}, nil
}

type adminAccountBinding struct {
	templeId    string
	masterId    string
	shopId      int64
	templeAdmin bool
}

func validateAdminAccountBinding(ctx context.Context, svcCtx *svc.ServiceContext, roleId int64, templeId, masterId string, shopId int64) (*adminAccountBinding, error) {
	role, err := svcCtx.RoleModel.FindByID(ctx, roleId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrParam
		}
		return nil, common.ErrSystem
	}

	switch role.Code {
	case model.RoleCodeTempleAdmin:
		if templeId == "" {
			return nil, common.ErrParamMissing
		}
		if err := ensureTempleExists(ctx, svcCtx, templeId); err != nil {
			return nil, err
		}
		return &adminAccountBinding{templeId: templeId, templeAdmin: true}, nil

	case model.RoleCodeMaster:
		if masterId == "" {
			return nil, common.ErrParamMissing
		}
		ownerTempleId, err := findMasterTempleId(ctx, svcCtx, masterId)
		if err != nil {
			return nil, err
		}
		if templeId != "" && templeId != ownerTempleId {
			return nil, common.ErrParam
		}
		return &adminAccountBinding{templeId: ownerTempleId, masterId: masterId}, nil

	case model.RoleCodeShopAdmin:
		if shopId == 0 {
			return nil, common.ErrParamMissing
		}
		return &adminAccountBinding{shopId: shopId}, nil

	case model.RoleCodePlatformSuper, model.RoleCodePlatformService:
		return &adminAccountBinding{}, nil

	default:
		return nil, common.ErrParam
	}
}

func ensureTempleExists(ctx context.Context, svcCtx *svc.ServiceContext, templeId string) error {
	var id int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &id, "SELECT id FROM askxuan_temple.temple WHERE code = ?", templeId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return common.ErrTempleNotFound
		}
		return common.ErrSystem
	}
	return nil
}

func findMasterTempleId(ctx context.Context, svcCtx *svc.ServiceContext, masterId string) (string, error) {
	var templeId string
	if err := svcCtx.DB.QueryRowCtx(ctx, &templeId, "SELECT temple_code FROM askxuan_master.master WHERE code = ?", masterId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return "", common.ErrMasterNotFound
		}
		return "", common.ErrSystem
	}
	return templeId, nil
}

func initialAdminAccountStatus(ctx context.Context, svcCtx *svc.ServiceContext, templeId string, templeAdmin bool) (string, error) {
	if templeId == "" || !templeAdmin {
		return model.AccountStatusEnabled, nil
	}
	status, err := templeStatus(ctx, svcCtx, templeId)
	if err != nil {
		return "", err
	}
	if canEnableTempleAccount(status) {
		return model.AccountStatusEnabled, nil
	}
	return model.AccountStatusDisabled, nil
}

func templeStatus(ctx context.Context, svcCtx *svc.ServiceContext, templeId string) (string, error) {
	var status string
	if err := svcCtx.DB.QueryRowCtx(ctx, &status, "SELECT status FROM askxuan_temple.temple WHERE code = ?", templeId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return "", common.ErrTempleNotFound
		}
		return "", common.ErrSystem
	}
	return status, nil
}

func canEnableTempleAccount(status string) bool {
	return status == "正常" || status == "推荐"
}

func insertAdminAccountWithBinding(ctx context.Context, db sqlx.SqlConn, account *model.AdminAccount, templeAdmin bool) (int64, error) {
	var id int64
	err := db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `INSERT INTO admin_account
			(account,password,name,role_id,temple_id,master_id,shop_id,status)
			VALUES (?,?,?,?,?,?,?,?)`, account.Account, account.Password, account.Name,
			account.RoleId, account.TempleId, account.MasterId, account.ShopId, account.Status)
		if err != nil {
			return err
		}
		id, err = result.LastInsertId()
		if err != nil || !templeAdmin {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO askxuan_temple.temple_admin
			(temple_code,account_id,role) VALUES (?,?,'admin')`, account.TempleId, id)
		return err
	})
	return id, err
}

func updateAdminAccountWithBinding(ctx context.Context, db sqlx.SqlConn, account *model.AdminAccount, templeAdmin bool) error {
	return db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `UPDATE admin_account SET name=?,role_id=?,temple_id=?,master_id=?,shop_id=?,status=? WHERE id=?`,
			account.Name, account.RoleId, account.TempleId, account.MasterId, account.ShopId, account.Status, account.Id); err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple_admin WHERE account_id = ?", account.Id); err != nil {
			return err
		}
		if !templeAdmin {
			return nil
		}
		_, err := session.ExecCtx(ctx, `INSERT INTO askxuan_temple.temple_admin
			(temple_code,account_id,role) VALUES (?,?,'admin')`, account.TempleId, account.Id)
		return err
	})
}
