package points

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type Store struct{ DB sqlx.SqlConn }
type Account struct {
	Balance int64 `json:"balance" db:"balance"`
}
type Entry struct {
	ID           int64  `json:"id" db:"id"`
	Kind         string `json:"kind" db:"kind"`
	Delta        int64  `json:"delta" db:"delta"`
	BalanceAfter int64  `json:"balanceAfter" db:"balance_after"`
	ReferenceNo  string `json:"referenceNo" db:"reference_no"`
	CreatedAt    string `json:"createdAt" db:"created_at"`
}
type Product struct {
	ID          int64  `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Category    string `json:"category" db:"category"`
	Description string `json:"description" db:"description"`
	Image       string `json:"image" db:"image"`
	PointsPrice int64  `json:"pointsPrice" db:"points_price"`
	Stock       int64  `json:"stock" db:"stock"`
	Status      string `json:"status" db:"status"`
	Version     int64  `json:"version" db:"version"`
}
type Order struct {
	ID           int64  `json:"id" db:"id"`
	OrderNo      string `json:"orderNo" db:"order_no"`
	UserID       string `json:"userId" db:"user_id"`
	RequestKey   string `json:"requestKey" db:"request_key"`
	ProductID    int64  `json:"productId" db:"product_id"`
	ProductName  string `json:"productName" db:"product_name"`
	ProductImage string `json:"productImage" db:"product_image"`
	Quantity     int64  `json:"quantity" db:"quantity"`
	PointsTotal  int64  `json:"pointsTotal" db:"points_total"`
	Receiver     string `json:"receiver" db:"receiver"`
	Mobile       string `json:"mobile" db:"mobile"`
	Address      string `json:"address" db:"address"`
	Status       string `json:"status" db:"status"`
	Carrier      string `json:"carrier" db:"carrier"`
	TrackingNo   string `json:"trackingNo" db:"tracking_no"`
	CreatedAt    string `json:"createdAt" db:"created_at"`
}
type RedeemRequest struct {
	ProductID     int64  `json:"productId"`
	Quantity      int64  `json:"quantity"`
	ExpectedPrice int64  `json:"expectedPrice"`
	RequestKey    string `json:"requestKey"`
	Receiver      string `json:"receiver"`
	Mobile        string `json:"mobile"`
	Address       string `json:"address"`
}

const productCols = `id,name,category,description,image,points_price,stock,status,version`
const orderCols = `id,order_no,user_id,request_key,product_id,product_name,product_image,quantity,points_total,receiver,mobile,address,status,carrier,tracking_no,created_at`

func (s Store) Account(ctx context.Context, user string) (Account, error) {
	var a Account
	err := s.DB.QueryRowCtx(ctx, &a, `SELECT balance FROM points_account WHERE user_id=?`, user)
	if errors.Is(err, sqlx.ErrNotFound) {
		err = nil
	}
	return a, err
}
func (s Store) Entries(ctx context.Context, user string, page int) ([]Entry, error) {
	rows := make([]Entry, 0)
	err := s.DB.QueryRowsCtx(ctx, &rows, `SELECT id,kind,delta,balance_after,reference_no,created_at FROM points_ledger WHERE user_id=? ORDER BY id DESC LIMIT 20 OFFSET ?`, user, (page-1)*20)
	return rows, err
}
func (s Store) Products(ctx context.Context, admin bool, page int) ([]Product, error) {
	rows := make([]Product, 0)
	where := " WHERE status='on_sale'"
	if admin {
		where = ""
	}
	err := s.DB.QueryRowsCtx(ctx, &rows, `SELECT `+productCols+` FROM points_product`+where+` ORDER BY id DESC LIMIT 20 OFFSET ?`, (page-1)*20)
	return rows, err
}
func ValidProduct(p Product) bool {
	return strings.TrimSpace(p.Name) != "" && len([]rune(p.Name)) <= 120 && len([]rune(p.Category)) <= 80 && len(p.Description) <= 20000 && len(p.Image) <= 1000 && (p.Image == "" || strings.HasPrefix(p.Image, "https://") || strings.HasPrefix(p.Image, "http://") || strings.HasPrefix(p.Image, "/")) && p.PointsPrice > 0 && p.PointsPrice <= 100000000 && p.Stock >= 0 && p.Stock <= 100000000 && (p.Status == "draft" || p.Status == "on_sale" || p.Status == "off_sale")
}
func (s Store) SaveProduct(ctx context.Context, p Product) (Product, error) {
	if !ValidProduct(p) {
		return p, ErrInvalid
	}
	if p.ID == 0 {
		r, err := s.DB.ExecCtx(ctx, `INSERT INTO points_product(name,category,description,image,points_price,stock,status) VALUES(?,?,?,?,?,?,?)`, p.Name, p.Category, p.Description, p.Image, p.PointsPrice, p.Stock, p.Status)
		if err != nil {
			return p, err
		}
		p.ID, err = r.LastInsertId()
		p.Version = 1
		return p, err
	}
	r, err := s.DB.ExecCtx(ctx, `UPDATE points_product SET name=?,category=?,description=?,image=?,points_price=?,stock=?,status=?,version=version+1 WHERE id=? AND version=?`, p.Name, p.Category, p.Description, p.Image, p.PointsPrice, p.Stock, p.Status, p.ID, p.Version)
	if err != nil {
		return p, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return p, err
	}
	if n != 1 {
		return p, ErrConflict
	}
	p.Version++
	return p, nil
}
func (s Store) Orders(ctx context.Context, user string, admin bool, page int) ([]Order, error) {
	rows := make([]Order, 0)
	where := " WHERE user_id=?"
	args := []interface{}{user}
	if admin {
		where = ""
		args = nil
	}
	args = append(args, (page-1)*20)
	err := s.DB.QueryRowsCtx(ctx, &rows, `SELECT `+orderCols+` FROM points_order`+where+` ORDER BY id DESC LIMIT 20 OFFSET ?`, args...)
	return rows, err
}
func ValidRedeem(r RedeemRequest) bool {
	return r.ProductID > 0 && r.Quantity > 0 && r.Quantity <= 99 && r.ExpectedPrice > 0 && r.ExpectedPrice <= 100000000 && len(r.RequestKey) >= 16 && len(r.RequestKey) <= 80 && strings.TrimSpace(r.Receiver) != "" && len([]rune(r.Receiver)) <= 80 && strings.TrimSpace(r.Mobile) != "" && len(r.Mobile) <= 32 && strings.TrimSpace(r.Address) != "" && len([]rune(r.Address)) <= 500
}
func (s Store) Redeem(ctx context.Context, user string, r RedeemRequest) (Order, error) {
	var result Order
	if !ValidRedeem(r) {
		return result, ErrInvalid
	}
	err := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// Serialize same-user requests, including concurrent retries of an idempotency key.
		if _, err := tx.ExecCtx(ctx, `INSERT INTO points_account(user_id) VALUES(?) ON DUPLICATE KEY UPDATE user_id=user_id`, user); err != nil {
			return err
		}
		var balance int64
		if err := tx.QueryRowCtx(ctx, &balance, `SELECT balance FROM points_account WHERE user_id=? FOR UPDATE`, user); err != nil {
			return err
		}
		err := tx.QueryRowCtx(ctx, &result, `SELECT `+orderCols+` FROM points_order WHERE user_id=? AND request_key=?`, user, r.RequestKey)
		if err == nil {
			if result.ProductID != r.ProductID || result.Quantity != r.Quantity || result.PointsTotal != r.ExpectedPrice*r.Quantity || result.Receiver != r.Receiver || result.Mobile != r.Mobile || result.Address != r.Address {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(err, sqlx.ErrNotFound) {
			return err
		}
		var p Product
		if err := tx.QueryRowCtx(ctx, &p, `SELECT `+productCols+` FROM points_product WHERE id=? FOR UPDATE`, r.ProductID); err != nil {
			return err
		}
		if p.Status != "on_sale" || p.Stock < r.Quantity || p.PointsPrice != r.ExpectedPrice {
			return ErrConflict
		}
		total := p.PointsPrice * r.Quantity
		if balance < total {
			return ErrBalance
		}
		no := "PT" + strings.ReplaceAll(uuid.NewString(), "-", "")
		ins, err := tx.ExecCtx(ctx, `INSERT INTO points_order(order_no,user_id,request_key,product_id,product_name,product_image,quantity,points_total,receiver,mobile,address) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, no, user, r.RequestKey, p.ID, p.Name, p.Image, r.Quantity, total, r.Receiver, r.Mobile, r.Address)
		if err != nil {
			return err
		}
		id, err := ins.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, `UPDATE points_product SET stock=stock-?,version=version+1 WHERE id=?`, r.Quantity, p.ID); err != nil {
			return err
		}
		if err = Change(ctx, tx, user, "redeem:"+no, "redeem", no, -total); err != nil {
			return err
		}
		return tx.QueryRowCtx(ctx, &result, `SELECT `+orderCols+` FROM points_order WHERE id=?`, id)
	})
	return result, err
}
func (s Store) Transition(ctx context.Context, user string, id int64, action, carrier, tracking string, admin bool) error {
	if action != "cancel" && action != "complete" && action != "ship" {
		return ErrInvalid
	}
	if action == "ship" && (!admin || strings.TrimSpace(carrier) == "" || strings.TrimSpace(tracking) == "" || len([]rune(carrier)) > 80 || len(tracking) > 100) {
		return ErrInvalid
	}
	// Fetch owner before transaction; immutable and used to keep account -> order -> product lock order.
	var owner string
	if err := s.DB.QueryRowCtx(ctx, &owner, `SELECT user_id FROM points_order WHERE id=?`, id); err != nil {
		return err
	}
	if !admin && owner != user {
		return ErrInvalid
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var balance int64
		if err := tx.QueryRowCtx(ctx, &balance, `SELECT balance FROM points_account WHERE user_id=? FOR UPDATE`, owner); err != nil {
			return err
		}
		var o Order
		if err := tx.QueryRowCtx(ctx, &o, `SELECT `+orderCols+` FROM points_order WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		target := ""
		switch action {
		case "cancel":
			target = "cancelled"
		case "ship":
			target = "shipped"
		case "complete":
			target = "completed"
		}
		if o.Status == target {
			if action == "ship" && (o.Carrier != carrier || o.TrackingNo != tracking) {
				return ErrConflict
			}
			return nil
		}
		if (action == "cancel" || action == "ship") && o.Status != "pending" {
			return ErrConflict
		}
		if action == "complete" && o.Status != "shipped" {
			return ErrConflict
		}
		if action == "cancel" {
			if err := Change(ctx, tx, owner, fmt.Sprintf("cancel:%d", id), "return", o.OrderNo, o.PointsTotal); err != nil {
				return err
			}
			if _, err := tx.ExecCtx(ctx, `UPDATE points_product SET stock=stock+?,version=version+1 WHERE id=?`, o.Quantity, o.ProductID); err != nil {
				return err
			}
		}
		_, err := tx.ExecCtx(ctx, `UPDATE points_order SET status=?,carrier=IF(?='',carrier,?),tracking_no=IF(?='',tracking_no,?) WHERE id=?`, target, carrier, carrier, tracking, tracking, id)
		return err
	})
}

type Report struct {
	ProductCount int64 `json:"productCount" db:"product_count"`
	OrderCount   int64 `json:"orderCount" db:"order_count"`
	PointsSpent  int64 `json:"pointsSpent" db:"points_spent"`
	PendingCount int64 `json:"pendingCount" db:"pending_count"`
}

func (s Store) Report(ctx context.Context) (Report, error) {
	var r Report
	err := s.DB.QueryRowCtx(ctx, &r, `SELECT (SELECT COUNT(*) FROM points_product) product_count,COUNT(*) order_count,COALESCE(SUM(IF(status<>'cancelled',points_total,0)),0) points_spent,COALESCE(SUM(status='pending'),0) pending_count FROM points_order`)
	return r, err
}
