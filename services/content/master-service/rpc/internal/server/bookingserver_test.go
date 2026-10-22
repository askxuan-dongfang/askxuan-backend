package server

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/rpc/master"
)

type parentGateMasterModel struct {
	model.MasterModel
	data   *model.Master
	parent bool
	err    error
}

func (m *parentGateMasterModel) FindByCode(context.Context, string) (*model.Master, error) {
	return m.data, nil
}
func (m *parentGateMasterModel) ParentAllowsNewBusiness(context.Context, string) (bool, error) {
	return m.parent, m.err
}
func (m *parentGateMasterModel) FindTempleNameByCode(context.Context, string) (string, error) {
	return "Parent", nil
}
func TestBookingMasterUsesEffectiveParentEligibilityWithoutChangingSubject(t *testing.T) {
	for _, tc := range []struct {
		name   string
		parent bool
		auth   string
		err    error
		want   string
	}{
		{"parent eligible", true, "已认证", nil, "normal"},
		{"parent blocked", false, "已认证", nil, "banned"},
		{"master uncertified", true, "待审核", nil, "banned"},
		{"dependency fails closed", false, "已认证", errors.New("db unavailable"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &model.Master{Id: 42, Code: "M42", DharmaName: "Master", TempleCode: "T42", PlatformStatus: "normal", ShelfStatus: "on_shelf", AuthStatus: tc.auth}
			server := NewMasterBookingServer(&svc.ServiceContext{MasterModel: &parentGateMasterModel{data: data, parent: tc.parent, err: tc.err}})
			out, err := server.GetBookingMaster(context.Background(), &master.GetBookingMasterReq{MasterCode: "M42"})
			if tc.err != nil {
				if err == nil {
					t.Fatal("dependency failure allowed booking")
				}
				return
			}
			if err != nil || out.PlatformStatus != tc.want || out.Id != 42 || out.Code != "M42" || out.DharmaName != "Master" {
				t.Fatalf("effective eligibility lost existing-order identity: %+v %v", out, err)
			}
			if data.PlatformStatus != "normal" || data.ShelfStatus != "on_shelf" {
				t.Fatal("parent restriction mutated child record")
			}
		})
	}
}
