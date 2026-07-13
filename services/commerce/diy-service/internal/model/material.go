package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 材料分类常量
const (
	MaterialCategoryMainBead   = "main_bead"   // 主珠
	MaterialCategorySpacer     = "spacer"      // 隔片
	MaterialCategoryBuddhaHead = "buddha_head" // 佛头
	MaterialCategoryPendant    = "pendant"     // 吊坠
	MaterialCategoryTassel     = "tassel"      // 流苏
	MaterialCategoryThreeWay   = "three_way"   // 三通
	MaterialCategoryCord       = "cord"        // 绳线
)

// 材料状态常量
const (
	MaterialStatusOnShelf  = "on_shelf"
	MaterialStatusOffShelf = "off_shelf"
)

const (
	materialTable     = "material" // 已迁入 askxuan_diy 库
	materialSkuTable  = "material_sku"
	extraServiceTable = "extra_service" // 已迁入 askxuan_diy 库
)

// Material 材料表
type Material struct {
	Id           int64   `db:"id" json:"id"`
	Name         string  `db:"name" json:"name"`
	Spec         string  `db:"spec" json:"spec"`
	UnitPrice    float64 `db:"unit_price" json:"unitPrice"`
	Unit         string  `db:"unit" json:"unit"`
	Category     string  `db:"category" json:"category"`
	FiveElements string  `db:"five_elements" json:"fiveElements"`
	Image        string  `db:"image" json:"image"`
	Stock        int     `db:"stock" json:"stock"`
	Status       string  `db:"status" json:"status"`
}

// MaterialSku 材料规格表
type MaterialSku struct {
	Id         int64   `db:"id" json:"id"`
	MaterialId int64   `db:"material_id" json:"materialId"`
	Spec       string  `db:"spec" json:"spec"`
	Price      float64 `db:"price" json:"price"`
	Stock      int     `db:"stock" json:"stock"`
}

// ExtraService 加持服务（查 askxuan.extra_service 表）
type ExtraService struct {
	Id          int64   `db:"id" json:"id"`
	Code        string  `db:"code" json:"code"`
	Name        string  `db:"name" json:"name"`
	TempleCode  string  `db:"temple_code" json:"templeCode"`
	MasterCode  string  `db:"master_code" json:"masterCode"`
	Price       float64 `db:"price" json:"price"`
	Description string  `db:"description" json:"description"`
}

// MaterialModel 材料模型接口
type MaterialModel interface {
	Insert(ctx context.Context, data *Material) (*Material, error)
	FindOne(ctx context.Context, id int64) (*Material, error)
	FindList(ctx context.Context, category, keyword string, page, size int) ([]*Material, int64, error)
	Update(ctx context.Context, data *Material) error
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// MaterialSkuModel 材料SKU接口
type MaterialSkuModel interface {
	Insert(ctx context.Context, data *MaterialSku) (*MaterialSku, error)
	ListByMaterialId(ctx context.Context, materialId int64) ([]*MaterialSku, error)
}

// ExtraServiceModel 加持服务接口
type ExtraServiceModel interface {
	FindList(ctx context.Context, page, size int) ([]*ExtraService, error)
}

// ===== MaterialModel 实现 =====

type defaultMaterialModel struct {
	conn sqlx.SqlConn
}

func NewMaterialModel(conn sqlx.SqlConn) MaterialModel {
	return &defaultMaterialModel{conn: conn}
}

func (m *defaultMaterialModel) Insert(ctx context.Context, data *Material) (*Material, error) {
	if data.Status == "" {
		data.Status = MaterialStatusOnShelf
	}
	query := fmt.Sprintf(`INSERT INTO %s (name, spec, unit_price, unit, category, five_elements, image, stock, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, materialTable)
	result, err := m.conn.ExecCtx(ctx, query, data.Name, data.Spec, data.UnitPrice, data.Unit, data.Category, data.FiveElements, data.Image, data.Stock, data.Status)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func (m *defaultMaterialModel) FindOne(ctx context.Context, id int64) (*Material, error) {
	var mat Material
	query := fmt.Sprintf(`SELECT id, name, spec, unit_price, unit, category, five_elements, image, stock, status FROM %s WHERE id = ?`, materialTable)
	err := m.conn.QueryRowCtx(ctx, &mat, query, id)
	if err != nil {
		return nil, err
	}
	return &mat, nil
}

func (m *defaultMaterialModel) FindList(ctx context.Context, category, keyword string, page, size int) ([]*Material, int64, error) {
	where := "1=1"
	var args []interface{}
	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}
	if keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, materialTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Material{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, name, spec, unit_price, unit, category, five_elements, image, stock, status FROM %s WHERE %s ORDER BY id ASC LIMIT ?, ?`, materialTable, where)
	listArgs := append(args, offset, size)
	var list []*Material
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultMaterialModel) Update(ctx context.Context, data *Material) error {
	query := fmt.Sprintf(`UPDATE %s SET name=?, spec=?, unit_price=?, unit=?, category=?, five_elements=?, image=?, stock=? WHERE id=?`, materialTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.Spec, data.UnitPrice, data.Unit, data.Category, data.FiveElements, data.Image, data.Stock, data.Id)
	return err
}

func (m *defaultMaterialModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf(`UPDATE %s SET status=? WHERE id=?`, materialTable)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

// ===== MaterialSkuModel 实现 =====

type defaultMaterialSkuModel struct {
	conn sqlx.SqlConn
}

func NewMaterialSkuModel(conn sqlx.SqlConn) MaterialSkuModel {
	return &defaultMaterialSkuModel{conn: conn}
}

func (m *defaultMaterialSkuModel) Insert(ctx context.Context, data *MaterialSku) (*MaterialSku, error) {
	query := fmt.Sprintf(`INSERT INTO %s (material_id, spec, price, stock) VALUES (?, ?, ?, ?)`, materialSkuTable)
	result, err := m.conn.ExecCtx(ctx, query, data.MaterialId, data.Spec, data.Price, data.Stock)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func (m *defaultMaterialSkuModel) ListByMaterialId(ctx context.Context, materialId int64) ([]*MaterialSku, error) {
	query := fmt.Sprintf(`SELECT id, material_id, spec, price, stock FROM %s WHERE material_id = ?`, materialSkuTable)
	var list []*MaterialSku
	err := m.conn.QueryRowsCtx(ctx, &list, query, materialId)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ===== ExtraServiceModel 实现 =====

type defaultExtraServiceModel struct {
	conn sqlx.SqlConn
}

func NewExtraServiceModel(conn sqlx.SqlConn) ExtraServiceModel {
	return &defaultExtraServiceModel{conn: conn}
}

func (m *defaultExtraServiceModel) FindList(ctx context.Context, page, size int) ([]*ExtraService, error) {
	offset := (page - 1) * size
	query := fmt.Sprintf(`SELECT id, code, name, temple_code, master_code, price, description FROM %s ORDER BY id ASC LIMIT ?, ?`, extraServiceTable)
	var list []*ExtraService
	err := m.conn.QueryRowsCtx(ctx, &list, query, offset, size)
	if err != nil {
		return nil, err
	}
	return list, nil
}
