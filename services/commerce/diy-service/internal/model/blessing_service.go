package model

import (
	"fmt"
	"sync"
	"time"
)

// 加持服务状态常量
const (
	BlessingServiceStatusOnShelf  = "on_shelf"
	BlessingServiceStatusOffShelf = "off_shelf"
)

// BlessingServiceRecord 加持服务（内存存储，对应 askxuan.extra_service 表）
type BlessingServiceRecord struct {
	Id          int64   `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	TempleCode  string  `json:"templeCode"`
	TempleName  string  `json:"templeName"`
	MasterCode  string  `json:"masterCode"`
	MasterName  string  `json:"masterName"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	CreateTime  string  `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type blessingServiceStore struct {
	mu   sync.RWMutex
	list []BlessingServiceRecord
	seq  int64
}

var globalBlessingServiceStore = &blessingServiceStore{
	list: []BlessingServiceRecord{
		{
			Id:          1,
			Code:        "BS001",
			Name:        "基础加持",
			TempleCode:  "T001",
			TempleName:  "灵隐寺",
			MasterCode:  "M001",
			MasterName:  "智海法师",
			Price:       200.00,
			Description: "基础祈福加持服务",
			Status:      BlessingServiceStatusOnShelf,
			CreateTime:  "2026-06-01 10:00:00",
		},
		{
			Id:          2,
			Code:        "BS002",
			Name:        "高级开光",
			TempleCode:  "T001",
			TempleName:  "灵隐寺",
			MasterCode:  "M002",
			MasterName:  "慧明法师",
			Price:       500.00,
			Description: "高级开光加持服务",
			Status:      BlessingServiceStatusOnShelf,
			CreateTime:  "2026-06-01 10:00:00",
		},
		{
			Id:          3,
			Code:        "BS003",
			Name:        "超度法事",
			TempleCode:  "T003",
			TempleName:  "法门寺",
			MasterCode:  "M003",
			MasterName:  "寂空法师",
			Price:       800.00,
			Description: "超度法事加持服务",
			Status:      BlessingServiceStatusOffShelf,
			CreateTime:  "2026-06-01 10:00:00",
		},
	},
	seq: 3,
}

// BlessingServiceListModel 加持服务内存模型接口
type BlessingServiceListModel interface {
	FindList(ctx interface{}, page, size int) ([]*BlessingServiceRecord, int64)
	FindOne(id int64) (*BlessingServiceRecord, bool)
	Insert(data *BlessingServiceRecord) (*BlessingServiceRecord, error)
	Update(data *BlessingServiceRecord) error
	Delete(id int64) bool
	FindByCode(code string) (*BlessingServiceRecord, bool)
}

type defaultBlessingServiceListModel struct{}

// NewBlessingServiceListModel 构造加持服务内存模型
func NewBlessingServiceListModel() BlessingServiceListModel {
	return &defaultBlessingServiceListModel{}
}

// FindList 分页查询加持服务列表
func (m *defaultBlessingServiceListModel) FindList(_ interface{}, page, size int) ([]*BlessingServiceRecord, int64) {
	globalBlessingServiceStore.mu.RLock()
	defer globalBlessingServiceStore.mu.RUnlock()

	total := int64(len(globalBlessingServiceStore.list))
	start := (page - 1) * size
	if start < 0 {
		start = 0
	}
	if start > len(globalBlessingServiceStore.list) {
		start = len(globalBlessingServiceStore.list)
	}
	end := start + size
	if end > len(globalBlessingServiceStore.list) {
		end = len(globalBlessingServiceStore.list)
	}

	result := make([]*BlessingServiceRecord, 0, end-start)
	for i := start; i < end; i++ {
		r := globalBlessingServiceStore.list[i]
		result = append(result, &r)
	}
	return result, total
}

// FindOne 按 ID 查询加持服务
func (m *defaultBlessingServiceListModel) FindOne(id int64) (*BlessingServiceRecord, bool) {
	globalBlessingServiceStore.mu.RLock()
	defer globalBlessingServiceStore.mu.RUnlock()
	for i := range globalBlessingServiceStore.list {
		if globalBlessingServiceStore.list[i].Id == id {
			r := globalBlessingServiceStore.list[i]
			return &r, true
		}
	}
	return nil, false
}

// FindByCode 按服务编码查询加持服务
func (m *defaultBlessingServiceListModel) FindByCode(code string) (*BlessingServiceRecord, bool) {
	globalBlessingServiceStore.mu.RLock()
	defer globalBlessingServiceStore.mu.RUnlock()
	for i := range globalBlessingServiceStore.list {
		if globalBlessingServiceStore.list[i].Code == code {
			r := globalBlessingServiceStore.list[i]
			return &r, true
		}
	}
	return nil, false
}

// Insert 新增加持服务
func (m *defaultBlessingServiceListModel) Insert(data *BlessingServiceRecord) (*BlessingServiceRecord, error) {
	globalBlessingServiceStore.mu.Lock()
	defer globalBlessingServiceStore.mu.Unlock()

	globalBlessingServiceStore.seq++
	data.Id = globalBlessingServiceStore.seq
	if data.Code == "" {
		data.Code = fmt.Sprintf("BS%03d", data.Id)
	}
	if data.Status == "" {
		data.Status = BlessingServiceStatusOnShelf
	}
	data.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalBlessingServiceStore.list = append(globalBlessingServiceStore.list, *data)
	return data, nil
}

// Update 更新增持服务
func (m *defaultBlessingServiceListModel) Update(data *BlessingServiceRecord) error {
	globalBlessingServiceStore.mu.Lock()
	defer globalBlessingServiceStore.mu.Unlock()
	for i := range globalBlessingServiceStore.list {
		if globalBlessingServiceStore.list[i].Id == data.Id {
			// 保留 Code 和 CreateTime
			data.Code = globalBlessingServiceStore.list[i].Code
			data.CreateTime = globalBlessingServiceStore.list[i].CreateTime
			globalBlessingServiceStore.list[i] = *data
			return nil
		}
	}
	return fmt.Errorf("加持服务不存在(id=%d)", data.Id)
}

// Delete 删除加持服务
func (m *defaultBlessingServiceListModel) Delete(id int64) bool {
	globalBlessingServiceStore.mu.Lock()
	defer globalBlessingServiceStore.mu.Unlock()
	for i := range globalBlessingServiceStore.list {
		if globalBlessingServiceStore.list[i].Id == id {
			globalBlessingServiceStore.list = append(
				globalBlessingServiceStore.list[:i],
				globalBlessingServiceStore.list[i+1:]...,
			)
			return true
		}
	}
	return false
}
