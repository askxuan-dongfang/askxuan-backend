package model

import "sync"

// ============ 法师资料扩展 - 内存存储（bio / pricing） ============

// MasterProfileExt 法师资料扩展字段（DB schema 未含 bio/pricing，MVP 阶段内存存储）
type MasterProfileExt struct {
	Bio     string `json:"bio"`
	Pricing string `json:"pricing"`
}

// profileExtStore 内存存储
type profileExtStore struct {
	mu   sync.RWMutex
	data map[string]MasterProfileExt // key = masterCode
}

// globalProfileExtStore 全局资料扩展存储
var globalProfileExtStore = &profileExtStore{
	data: map[string]MasterProfileExt{
		"M001": {
			Bio:     "普陀山出家，擅长祈福法事与超度仪轨，弘法二十余载。",
			Pricing: "预约法事 200-800 元 / DIY加持 300-500 元",
		},
		"M002": {
			Bio:     "武当山修道，精通道教科仪与养生功法。",
			Pricing: "道教科仪 500-1200 元 / 养生咨询 200 元",
		},
	},
}

// GetProfileExt 获取法师资料扩展
func GetProfileExt(masterCode string) MasterProfileExt {
	globalProfileExtStore.mu.RLock()
	defer globalProfileExtStore.mu.RUnlock()
	if ext, ok := globalProfileExtStore.data[masterCode]; ok {
		return ext
	}
	return MasterProfileExt{}
}

// UpsertProfileExt 更新或插入法师资料扩展
func UpsertProfileExt(masterCode string, ext MasterProfileExt) {
	globalProfileExtStore.mu.Lock()
	defer globalProfileExtStore.mu.Unlock()
	globalProfileExtStore.data[masterCode] = ext
}
