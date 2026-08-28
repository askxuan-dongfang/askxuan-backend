package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

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
	MaterialStatusOnShelf         = "on_shelf"
	MaterialStatusOffShelf        = "off_shelf"
	BlessingServiceStatusOnShelf  = "on_shelf"
	BlessingServiceStatusOffShelf = "off_shelf"
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
	MaterialType string  `db:"material_type" json:"materialType"`
	Shape        string  `db:"shape" json:"shape"`
	DiameterMm   float64 `db:"diameter_mm" json:"diameterMm"`
	ColorHex     string  `db:"color_hex" json:"colorHex"`
	TextureKey   string  `db:"texture_key" json:"textureKey"`
	Finish       string  `db:"finish" json:"finish"`
	Translucency float64 `db:"translucency" json:"translucency"`
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
	Status      string  `db:"status" json:"status"`
	CreateTime  string  `db:"create_time" json:"createTime"`
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
	FindList(ctx context.Context, page, size int) ([]*ExtraService, int64, error)
	FindListByStatus(ctx context.Context, status string, page, size int) ([]*ExtraService, int64, error)
	FindOne(ctx context.Context, id int64) (*ExtraService, error)
	FindByCode(ctx context.Context, code string) (*ExtraService, error)
	Insert(ctx context.Context, data *ExtraService) (*ExtraService, error)
	Update(ctx context.Context, data *ExtraService) error
	Delete(ctx context.Context, id int64) (bool, error)
}

// ===== MaterialModel 实现 =====

type defaultMaterialModel struct {
	conn sqlx.SqlConn
}

func NewMaterialModel(conn sqlx.SqlConn) MaterialModel {
	return &defaultMaterialModel{conn: conn}
}

func normalizeMaterialPresentation(data *Material) {
	fiveElements := map[string]string{"金": "metal", "木": "wood", "水": "water", "火": "fire", "土": "earth"}
	if normalized, ok := fiveElements[data.FiveElements]; ok {
		data.FiveElements = normalized
	}
	if data.MaterialType == "" {
		switch data.Category {
		case MaterialCategoryCord:
			data.MaterialType = "cord"
		case MaterialCategoryTassel:
			data.MaterialType = "textile"
		case MaterialCategorySpacer, MaterialCategoryThreeWay:
			data.MaterialType = "metal"
		default:
			data.MaterialType = "gemstone"
		}
	}
	if data.Shape == "" {
		shapeByCategory := map[string]string{
			MaterialCategorySpacer: "disc", MaterialCategoryBuddhaHead: "buddha_head",
			MaterialCategoryPendant: "pendant", MaterialCategoryTassel: "tassel",
			MaterialCategoryThreeWay: "three_way", MaterialCategoryCord: "cord",
		}
		data.Shape = shapeByCategory[data.Category]
		if data.Shape == "" {
			data.Shape = "round"
		}
	}
	if data.DiameterMm <= 0 && data.Category != MaterialCategoryCord {
		data.DiameterMm = 10
	}
	if len(data.ColorHex) != 7 || data.ColorHex[0] != '#' {
		colors := map[string]string{"wood": "#5D936C", "fire": "#B93631", "earth": "#A36C22", "metal": "#B9C2C4", "water": "#315B96"}
		data.ColorHex = colors[data.FiveElements]
		if data.ColorHex == "" {
			data.ColorHex = "#8A6E4A"
		}
	}
	if data.TextureKey == "" {
		data.TextureKey = "plain"
	}
	if data.Finish == "" {
		data.Finish = "polished"
	}
	if data.Translucency < 0 {
		data.Translucency = 0
	} else if data.Translucency > 1 {
		data.Translucency = 1
	}
}

func (m *defaultMaterialModel) Insert(ctx context.Context, data *Material) (*Material, error) {
	normalizeMaterialPresentation(data)
	if data.Status == "" {
		data.Status = MaterialStatusOnShelf
	}
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		query := fmt.Sprintf(`INSERT INTO %s (name,spec,unit_price,unit,category,five_elements,material_type,shape,diameter_mm,color_hex,texture_key,finish,translucency,image,stock,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, materialTable)
		result, err := session.ExecCtx(ctx, query, data.Name, data.Spec, data.UnitPrice, data.Unit, data.Category, data.FiveElements, data.MaterialType, data.Shape, data.DiameterMm, data.ColorHex, data.TextureKey, data.Finish, data.Translucency, data.Image, data.Stock, data.Status)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		data.Id = id
		_, err = session.ExecCtx(ctx, fmt.Sprintf(`INSERT INTO %s (material_id,spec,price,stock) VALUES (?,?,?,?)`, materialSkuTable), id, data.Spec, data.UnitPrice, data.Stock)
		return err
	})
	return data, err
}

func (m *defaultMaterialModel) FindOne(ctx context.Context, id int64) (*Material, error) {
	var mat Material
	query := fmt.Sprintf(`SELECT m.id,m.name,m.spec,COALESCE((SELECT MIN(ms.price) FROM %s ms WHERE ms.material_id=m.id),m.unit_price) unit_price,m.unit,m.category,m.five_elements,m.material_type,m.shape,m.diameter_mm,m.color_hex,m.texture_key,m.finish,m.translucency,m.image,COALESCE((SELECT SUM(ms.stock) FROM %s ms WHERE ms.material_id=m.id),m.stock) stock,m.status FROM %s m WHERE m.id=?`, materialSkuTable, materialSkuTable, materialTable)
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
	listQuery := fmt.Sprintf(`SELECT m.id,m.name,m.spec,COALESCE((SELECT MIN(ms.price) FROM %s ms WHERE ms.material_id=m.id),m.unit_price) unit_price,m.unit,m.category,m.five_elements,m.material_type,m.shape,m.diameter_mm,m.color_hex,m.texture_key,m.finish,m.translucency,m.image,COALESCE((SELECT SUM(ms.stock) FROM %s ms WHERE ms.material_id=m.id),m.stock) stock,m.status FROM %s m WHERE %s ORDER BY m.id ASC LIMIT ?,?`, materialSkuTable, materialSkuTable, materialTable, where)
	listArgs := append(args, offset, size)
	var list []*Material
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultMaterialModel) Update(ctx context.Context, data *Material) error {
	normalizeMaterialPresentation(data)
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		query := fmt.Sprintf(`UPDATE %s SET name=?,spec=?,unit_price=?,unit=?,category=?,five_elements=?,material_type=?,shape=?,diameter_mm=?,color_hex=?,texture_key=?,finish=?,translucency=?,image=?,stock=? WHERE id=?`, materialTable)
		if _, err := session.ExecCtx(ctx, query, data.Name, data.Spec, data.UnitPrice, data.Unit, data.Category, data.FiveElements, data.MaterialType, data.Shape, data.DiameterMm, data.ColorHex, data.TextureKey, data.Finish, data.Translucency, data.Image, data.Stock, data.Id); err != nil {
			return err
		}
		result, err := session.ExecCtx(ctx, fmt.Sprintf(`UPDATE %s SET spec=?,price=?,stock=? WHERE material_id=?`, materialSkuTable), data.Spec, data.UnitPrice, data.Stock, data.Id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			_, err = session.ExecCtx(ctx, fmt.Sprintf(`INSERT INTO %s (material_id,spec,price,stock) VALUES (?,?,?,?)`, materialSkuTable), data.Id, data.Spec, data.UnitPrice, data.Stock)
		}
		return err
	})
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

func (m *defaultExtraServiceModel) FindList(ctx context.Context, page, size int) ([]*ExtraService, int64, error) {
	return m.FindListByStatus(ctx, "", page, size)
}

func (m *defaultExtraServiceModel) FindListByStatus(ctx context.Context, status string, page, size int) ([]*ExtraService, int64, error) {
	where := "1=1"
	var args []interface{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", extraServiceTable, where), args...); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * size
	query := fmt.Sprintf(`SELECT id, code, name, temple_code, master_code, price, description, status, DATE_FORMAT(create_time,'%%Y-%%m-%%d %%H:%%i:%%s') create_time FROM %s WHERE %s ORDER BY id ASC LIMIT ?, ?`, extraServiceTable, where)
	args = append(args, offset, size)
	var list []*ExtraService
	err := m.conn.QueryRowsCtx(ctx, &list, query, args...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultExtraServiceModel) FindByCode(ctx context.Context, code string) (*ExtraService, error) {
	var service ExtraService
	query := fmt.Sprintf(`SELECT id, code, name, temple_code, master_code, price, description, status, DATE_FORMAT(create_time,'%%Y-%%m-%%d %%H:%%i:%%s') create_time FROM %s WHERE code=?`, extraServiceTable)
	if err := m.conn.QueryRowCtx(ctx, &service, query, code); err != nil {
		return nil, err
	}
	return &service, nil
}

func (m *defaultExtraServiceModel) FindOne(ctx context.Context, id int64) (*ExtraService, error) {
	var service ExtraService
	query := fmt.Sprintf(`SELECT id, code, name, temple_code, master_code, price, description, status, DATE_FORMAT(create_time,'%%Y-%%m-%%d %%H:%%i:%%s') create_time FROM %s WHERE id=?`, extraServiceTable)
	if err := m.conn.QueryRowCtx(ctx, &service, query, id); err != nil {
		return nil, err
	}
	return &service, nil
}

func (m *defaultExtraServiceModel) Insert(ctx context.Context, data *ExtraService) (*ExtraService, error) {
	random := make([]byte, 2)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	data.Code = "E" + time.Now().Format("0601021504") + hex.EncodeToString(random)
	if data.Status == "" {
		data.Status = BlessingServiceStatusOnShelf
	}
	query := fmt.Sprintf(`INSERT INTO %s(code,name,temple_code,master_code,price,description,status) VALUES(?,?,?,?,?,?,?)`, extraServiceTable)
	result, err := m.conn.ExecCtx(ctx, query, data.Code, data.Name, data.TempleCode, data.MasterCode, data.Price, data.Description, data.Status)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}

func (m *defaultExtraServiceModel) Update(ctx context.Context, data *ExtraService) error {
	query := fmt.Sprintf(`UPDATE %s SET name=?,temple_code=?,master_code=?,price=?,description=?,status=? WHERE id=?`, extraServiceTable)
	result, err := m.conn.ExecCtx(ctx, query, data.Name, data.TempleCode, data.MasterCode, data.Price, data.Description, data.Status, data.Id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return sqlx.ErrNotFound
	}
	return nil
}

func (m *defaultExtraServiceModel) Delete(ctx context.Context, id int64) (bool, error) {
	result, err := m.conn.ExecCtx(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=?", extraServiceTable), id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
