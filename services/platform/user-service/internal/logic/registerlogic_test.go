package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type registrationUserModel struct {
	model.UserModel
	found     *model.User
	findErr   error
	insertID  int64
	insertErr error
	inserted  *model.User
}

func (m *registrationUserModel) FindByMobile(_ context.Context, _ string) (*model.User, error) {
	return m.found, m.findErr
}

func (m *registrationUserModel) Insert(_ context.Context, user *model.User) (int64, error) {
	m.inserted = user
	return m.insertID, m.insertErr
}

func TestRegisterWithoutVerificationCode(t *testing.T) {
	users := &registrationUserModel{findErr: sqlx.ErrNotFound, insertID: 28}
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{UserModel: users})

	resp, err := logic.Register(&types.RegisterReq{Mobile: " 13900000028 ", Nickname: " 注册演示用户 "})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp.UserId != 28 || resp.Mobile != "13900000028" || resp.Nickname != "注册演示用户" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if users.inserted == nil || users.inserted.Mobile != "13900000028" || users.inserted.Password != "123456" {
		t.Fatalf("unexpected inserted user: %+v", users.inserted)
	}
}

func TestRegisterIgnoresLegacyCode(t *testing.T) {
	users := &registrationUserModel{findErr: sqlx.ErrNotFound, insertID: 29}
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{UserModel: users})

	resp, err := logic.Register(&types.RegisterReq{Mobile: "13900000029", Code: "not-a-real-code"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp.Nickname != "善信0029" {
		t.Fatalf("default nickname = %q", resp.Nickname)
	}
}

func TestRegisterRejectsInvalidMobile(t *testing.T) {
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{UserModel: &registrationUserModel{}})
	_, err := logic.Register(&types.RegisterReq{Mobile: "12345"})
	if !errors.Is(err, common.ErrParamInvalid) {
		t.Fatalf("Register() error = %v, want ErrParamInvalid", err)
	}
}

func TestRegisterRejectsExistingMobile(t *testing.T) {
	users := &registrationUserModel{found: &model.User{Id: 1, Mobile: "13800138000"}}
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{UserModel: users})
	_, err := logic.Register(&types.RegisterReq{Mobile: "13800138000"})
	if !errors.Is(err, common.ErrUserAlreadyExists) {
		t.Fatalf("Register() error = %v, want ErrUserAlreadyExists", err)
	}
}

func TestRegisterMapsConcurrentDuplicate(t *testing.T) {
	users := &registrationUserModel{
		findErr:   sqlx.ErrNotFound,
		insertErr: &mysql.MySQLError{Number: 1062, Message: "duplicate"},
	}
	logic := NewRegisterLogic(context.Background(), &svc.ServiceContext{UserModel: users})
	_, err := logic.Register(&types.RegisterReq{Mobile: "13900000030"})
	if !errors.Is(err, common.ErrUserAlreadyExists) {
		t.Fatalf("Register() error = %v, want ErrUserAlreadyExists", err)
	}
}
