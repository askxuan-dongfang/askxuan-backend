package model

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type catalogConn struct {
	sqlx.SqlConn
	queries []string
	args    [][]any
}

func (c *catalogConn) QueryRowCtx(_ context.Context, v any, query string, args ...any) error {
	c.queries = append(c.queries, query)
	c.args = append(c.args, args)
	*v.(*int64) = 43
	return nil
}
func (c *catalogConn) QueryRowsCtx(_ context.Context, v any, query string, args ...any) error {
	c.queries = append(c.queries, query)
	c.args = append(c.args, args)
	*v.(*[]*Product) = []*Product{{Id: 9, Price: 12.35}}
	return nil
}
func TestCatalogFiltersApplyBeforePaginationAndToTotal(t *testing.T) {
	conn := &catalogConn{}
	products, total, err := NewProductModel(conn).FindList(context.Background(), 7, "香' OR 1=1 --", "on_shelf", 2, 20, CatalogOptions{Sort: "price_asc", InStock: true})
	if err != nil || total != 43 || len(products) != 1 {
		t.Fatalf("unexpected result: %v %d %v", products, total, err)
	}
	for _, query := range conn.queries {
		if !strings.Contains(query, "category_id = ?") || !strings.Contains(query, "status = ?") || !strings.Contains(query, "stock > 0") || strings.Contains(query, "OR 1=1") {
			t.Fatalf("unsafe/inconsistent filtering: %s", query)
		}
	}
	if !strings.Contains(conn.queries[1], "ORDER BY price ASC, id DESC LIMIT ?, ?") {
		t.Fatalf("sort must precede paging: %s", conn.queries[1])
	}
	if !reflect.DeepEqual(conn.args[1][:3], conn.args[0]) || !reflect.DeepEqual(conn.args[1][3:], []any{20, 20}) {
		t.Fatalf("wrong paging/filter args: %#v", conn.args)
	}
}
func TestCatalogRejectsUnknownSortBeforeQuery(t *testing.T) {
	for _, sort := range []string{"price; DROP TABLE product", "stock", "PRICE_ASC"} {
		conn := &catalogConn{}
		_, _, err := NewProductModel(conn).FindList(context.Background(), 0, "", "on_shelf", 1, 20, CatalogOptions{Sort: sort})
		if err == nil || len(conn.queries) != 0 {
			t.Fatalf("unsafe sort accepted: %s", sort)
		}
	}
}
func TestCatalogDefaultAndInvalidPagingRemainCompatible(t *testing.T) {
	conn := &catalogConn{}
	_, _, err := NewProductModel(conn).FindList(context.Background(), 0, "", "", -1, 1000)
	if err != nil || !strings.Contains(conn.queries[1], "ORDER BY create_time DESC, id DESC") || !reflect.DeepEqual(conn.args[1], []any{0, 20}) {
		t.Fatalf("unexpected default query: %v %#v %v", conn.queries, conn.args, err)
	}
}
