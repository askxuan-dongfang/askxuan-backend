package points

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func TestExperienceAwardDoesNotWritePoints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)
	mock.ExpectQuery("SELECT user_id,payment_no,order_type,order_no").WithArgs(int64(77)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "payment_no", "order_type", "order_no", "cents"}).AddRow("user", "PAY-case", "shop_order", "EXO-case", 15900))
	if err = Award(context.Background(), conn, 77); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestOrdinaryPaymentStillRecordsAward(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)
	mock.ExpectQuery("SELECT user_id,payment_no,order_type,order_no").WithArgs(int64(78)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "payment_no", "order_type", "order_no", "cents"}).AddRow("user", "PAY-normal", "shop_order", "SO-normal", 5900))
	mock.ExpectQuery("SELECT COUNT").WithArgs(int64(78)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO points_payment_award").WithArgs(int64(78), "user", int64(0)).WillReturnResult(sqlmock.NewResult(1, 1))
	if err = Award(context.Background(), conn, 78); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
