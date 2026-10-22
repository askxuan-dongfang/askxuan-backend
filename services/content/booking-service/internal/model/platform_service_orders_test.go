package model

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type serviceOrderQueryCall struct {
	query string
	args  []interface{}
}

// Embedding an unset SqlConn means a write or transaction would fail the test;
// the platform reader must only count and read a bounded page.
type serviceOrderReadConn struct {
	sqlx.SqlConn
	calls []serviceOrderQueryCall
	fail  bool
}

func (c *serviceOrderReadConn) QueryRowCtx(_ context.Context, target interface{}, query string, args ...interface{}) error {
	c.calls = append(c.calls, serviceOrderQueryCall{query, args})
	if c.fail {
		return errors.New("database unavailable")
	}
	*target.(*int64) = 24
	return nil
}
func (c *serviceOrderReadConn) QueryRowsCtx(_ context.Context, target interface{}, query string, args ...interface{}) error {
	c.calls = append(c.calls, serviceOrderQueryCall{query, args})
	switch rows := target.(type) {
	case *[]*Booking:
		*rows = []*Booking{{Id: "B-page-2"}}
	case *[]*Consultation:
		*rows = []*Consultation{{Id: "C-page-2"}}
	default:
		panic("unexpected read result")
	}
	return nil
}

func TestPlatformServiceReaderFiltersBothCountAndBoundedPage(t *testing.T) {
	for _, consultation := range []bool{false, true} {
		t.Run(map[bool]string{false: "bookings", true: "consultations"}[consultation], func(t *testing.T) {
			conn := &serviceOrderReadConn{}
			r := NewPlatformServiceOrderReader(conn)
			q := PlatformServiceOrderQuery{Q: "'%_! OR 1=1", Status: "pending", PaymentStatus: "success", TempleID: "T2", MasterID: "M7", From: "2026-10-01", To: "2026-10-22", Page: 2, Size: 10}
			var total int64
			var err error
			if consultation {
				_, total, err = r.Consultations(context.Background(), q)
			} else {
				_, total, err = r.Bookings(context.Background(), q)
			}
			if err != nil || total != 24 || len(conn.calls) != 2 {
				t.Fatalf("total=%d calls=%d err=%v", total, len(conn.calls), err)
			}
			count, page := conn.calls[0], conn.calls[1]
			countWhere := strings.SplitN(count.query, " WHERE ", 2)[1]
			pageWhere := strings.SplitN(strings.SplitN(page.query, " WHERE ", 2)[1], " ORDER BY ", 2)[0]
			primaryKey := "id"
			if consultation {
				primaryKey = "order_no"
			}
			if countWhere != pageWhere || !strings.HasSuffix(page.query, "ORDER BY create_time DESC,"+primaryKey+" DESC LIMIT ?,?") {
				t.Fatalf("count/page predicates or ordering differ: %s; %s", count.query, page.query)
			}
			if strings.Contains(page.query, q.Q) || !strings.Contains(pageWhere, "create_time<DATE_ADD(?,INTERVAL 1 DAY)") {
				t.Fatal("search must be bound and end date inclusive")
			}
			if !reflect.DeepEqual(count.args, page.args[:len(page.args)-2]) || !reflect.DeepEqual(page.args[len(page.args)-2:], []interface{}{10, 10}) {
				t.Fatal("page must use the same bound filters and real offset/limit")
			}
			wantPrefix := []interface{}{"pending", "success", "T2", "M7", "2026-10-01 00:00:00", "2026-10-22 00:00:00"}
			if !reflect.DeepEqual(count.args[:len(wantPrefix)], wantPrefix) {
				t.Fatal("provider, status, payment and date filters are missing")
			}
			if got := count.args[len(wantPrefix)]; got != "%'!%!_!! OR 1=1%" {
				t.Fatalf("wildcard escaping=%v", got)
			}
		})
	}
}

func TestPlatformServiceReaderDoesNotDisguiseDatabaseFailureAsEmptyList(t *testing.T) {
	conn := &serviceOrderReadConn{fail: true}
	rows, _, err := NewPlatformServiceOrderReader(conn).Bookings(context.Background(), PlatformServiceOrderQuery{Page: 1, Size: 20})
	if err == nil || rows != nil || len(conn.calls) != 1 {
		t.Fatal("a database failure must propagate and not run a second query")
	}
}
