package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
)

const consultationRequiredMessage = "预约前须已完成该师傅的付费咨询，且咨询仍在有效期内"

func requireValidConsultation(ctx context.Context, s *svc.ServiceContext, user, master string) error {
	if master == "" {
		return nil
	} // Whole-temple execution does not nominate a master.
	valid, err := s.ConsultationModel.HasValidPaidConsultation(ctx, user, master)
	if err != nil {
		return common.ErrSystem
	}
	if !valid {
		return common.NewBizError(40415, consultationRequiredMessage)
	}
	return nil
}

func BookingEligibility(ctx context.Context, s *svc.ServiceContext, master string) (map[string]any, error) {
	user, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if master == "" || len(master) > 64 {
		return nil, common.ErrParamInvalid
	}
	valid, err := s.ConsultationModel.HasValidPaidConsultation(ctx, user, master)
	if err != nil {
		return nil, common.ErrSystem
	}
	return map[string]any{"masterId": master, "eligible": valid, "requirement": consultationRequiredMessage}, nil
}
