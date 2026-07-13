package model

import (
	"errors"
	"testing"
)

func TestDiyPaymentOrderValidatePayment(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		fee           float64
		status        string
		paymentStatus string
		amount        float64
		wantErr       error
	}{
		{name: "valid", owner: "u1", fee: 88.8, status: "pending_review", amount: 88.8},
		{name: "owner mismatch", owner: "u2", fee: 88.8, status: "pending_review", amount: 88.8, wantErr: ErrDiyOrderOwnerMismatch},
		{name: "not payable", owner: "u1", fee: 88.8, status: "in_making", amount: 88.8, wantErr: ErrDiyOrderNotPayable},
		{name: "already paid", owner: "u1", fee: 88.8, status: "pending_review", paymentStatus: "success", amount: 88.8, wantErr: ErrDiyOrderNotPayable},
		{name: "tampered amount", owner: "u1", fee: 88.8, status: "pending_review", amount: 0.01, wantErr: ErrDiyOrderAmountChanged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiyPaymentOrder(diyPaymentOrder{
				UserId: tt.owner, TotalFee: tt.fee, Status: tt.status, PaymentStatus: tt.paymentStatus,
			}, "u1", tt.amount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateDiyPaymentOrder() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
