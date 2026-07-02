package model

import (
	"sync"
)

// 活动类型
const (
	ActivityTypeLimitedDiscount = "limited_discount" // 限时折扣
	ActivityTypeFestival        = "festival"         // 节日活动
	ActivityTypeTempleEvent     = "temple_event"     // 寺院法会
)

// Activity 营销活动
type Activity struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Config    string `json:"config"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// ---- 内存存储（MVP-4 阶段不连 DB）----

type activityStore struct {
	mu   sync.RWMutex
	list []Activity
	seq  int64
}

var globalActivityStore = &activityStore{
	list: []Activity{
		{Id: 1, Name: "中元节祈福活动", Type: ActivityTypeFestival, StartTime: "2026-08-01 00:00:00", EndTime: "2026-08-15 23:59:59", Config: `{"discount":0.9}`, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 2, Name: "灵隐寺盂兰盆法会", Type: ActivityTypeTempleEvent, StartTime: "2026-08-10 09:00:00", EndTime: "2026-08-10 17:00:00", Config: `{"templeId":"T001"}`, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
	},
	seq: 2,
}

// ===== Activity CRUD =====

// ListActivities 活动列表
func ListActivities(status, aType string, page, size int) ([]Activity, int64) {
	globalActivityStore.mu.RLock()
	defer globalActivityStore.mu.RUnlock()
	filtered := make([]Activity, 0, len(globalActivityStore.list))
	for _, a := range globalActivityStore.list {
		if status != "" && a.Status != status {
			continue
		}
		if aType != "" && a.Type != aType {
			continue
		}
		filtered = append(filtered, a)
	}
	return paginateActivity(filtered, page, size)
}

// InsertActivity 新建活动
func InsertActivity(a Activity) Activity {
	globalActivityStore.mu.Lock()
	defer globalActivityStore.mu.Unlock()
	globalActivityStore.seq++
	a.Id = globalActivityStore.seq
	if a.Status == "" {
		a.Status = StatusEnabled
	}
	a.CreatedAt = nowStr()
	globalActivityStore.list = append(globalActivityStore.list, a)
	return a
}

// UpdateActivity 更新活动
func UpdateActivity(id int64, a Activity) (Activity, bool) {
	globalActivityStore.mu.Lock()
	defer globalActivityStore.mu.Unlock()
	for i := range globalActivityStore.list {
		if globalActivityStore.list[i].Id == id {
			if a.Name != "" {
				globalActivityStore.list[i].Name = a.Name
			}
			if a.Type != "" {
				globalActivityStore.list[i].Type = a.Type
			}
			if a.StartTime != "" {
				globalActivityStore.list[i].StartTime = a.StartTime
			}
			if a.EndTime != "" {
				globalActivityStore.list[i].EndTime = a.EndTime
			}
			if a.Config != "" {
				globalActivityStore.list[i].Config = a.Config
			}
			if a.Status != "" {
				globalActivityStore.list[i].Status = a.Status
			}
			return globalActivityStore.list[i], true
		}
	}
	return Activity{}, false
}

func paginateActivity(in []Activity, page, size int) ([]Activity, int64) {
	total := int64(len(in))
	start, end := pageRange(page, size, len(in))
	return in[start:end], total
}
