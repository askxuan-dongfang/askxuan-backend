package agentops

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func TestHarnessPublishGateRequiresMatchingEvaluationEngine(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT COUNT\(\*\).*JSON_UNQUOTE\(JSON_EXTRACT\(payload,'\$\.engine'\)\)='harness'`).WithArgs(int64(3), int64(4)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	repo := SQLRepository{DB: sqlx.NewSqlConnFromDB(db), RuntimeMode: "harness"}
	ok, err := repo.Tested(context.Background(), 3, 4)
	if err != nil || ok {
		t.Fatalf("classic-only evaluation must not approve harness: %v %v", ok, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
