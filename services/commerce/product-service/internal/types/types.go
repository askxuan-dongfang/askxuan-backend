package types

// Product 商品
type Product struct {
	Id                int64          `json:"id"`
	ProductNo         string         `json:"productNo"`
	Name              string         `json:"name"`
	CategoryId        int64          `json:"categoryId"`
	CategoryName      string         `json:"categoryName"`
	Description       string         `json:"description"`
	MainImage         string         `json:"mainImage"`
	Status            string         `json:"status"`
	Price             float64        `json:"price"`
	MarketPrice       float64        `json:"marketPrice"`
	Stock             int            `json:"stock"`
	Tags              string         `json:"tags"`
	FreightTemplateId int64          `json:"freightTemplateId"`
	Skus              []ProductSku   `json:"skus"`
	Images            []ProductImage `json:"images"`
	CreateTime        string         `json:"createTime"`
	UpdateTime        string         `json:"updateTime"`
}

// ProductSku 商品规格
type ProductSku struct {
	Id        int64   `json:"id"`
	ProductId int64   `json:"productId"`
	SpecName  string  `json:"specName"`
	SpecValue string  `json:"specValue"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	SkuNo     string  `json:"skuNo"`
}

// ProductCategory 商品分类
type ProductCategory struct {
	Id       int64             `json:"id"`
	ParentId int64             `json:"parentId"`
	Name     string            `json:"name"`
	Level    int               `json:"level"`
	Sort     int               `json:"sort"`
	Children []ProductCategory `json:"children"`
}

// ProductImage 商品图片
type ProductImage struct {
	Id        int64  `json:"id"`
	ProductId int64  `json:"productId"`
	ImageUrl  string `json:"imageUrl"`
	Sort      int    `json:"sort"`
	Type      string `json:"type"`
}

// ===== C端请求/响应 =====

type CustomerProductListReq struct {
	CategoryId int64  `form:"categoryId,optional"`
	Keyword    string `form:"keyword,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type CustomerProductListResp struct {
	Total int64     `json:"total"`
	List  []Product `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

type CustomerProductDetailReq struct {
	Id int64 `path:"id"`
}

// ===== C端收藏 =====

type ProductFavoriteReq struct {
	Id int64 `path:"id"`
}

type FavoriteResp struct {
	Favorited bool `json:"favorited"`
}

type FavoritesResp struct {
	List []Product `json:"list"`
}

type CustomerCategoryTreeReq struct{}

type CustomerCategoryTreeResp struct {
	List []ProductCategory `json:"list"`
}

type IntentionTag struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	LandingType  string `json:"landingType"`
	LandingValue string `json:"landingValue"`
	ActionTitle  string `json:"actionTitle"`
	Sort         int    `json:"sort"`
	Status       string `json:"status"`
}

type IntentionTagListResp struct {
	List []IntentionTag `json:"list"`
}

type AdminIntentionTagCreateReq struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description,optional"`
	Icon         string `json:"icon,optional"`
	LandingType  string `json:"landingType,optional"`
	LandingValue string `json:"landingValue,optional"`
	ActionTitle  string `json:"actionTitle,optional"`
	Sort         int    `json:"sort,optional"`
}

type AdminIntentionTagUpdateReq struct {
	Code         string `path:"code"`
	Name         string `json:"name"`
	Description  string `json:"description,optional"`
	Icon         string `json:"icon,optional"`
	LandingType  string `json:"landingType,optional"`
	LandingValue string `json:"landingValue,optional"`
	ActionTitle  string `json:"actionTitle,optional"`
	Sort         int    `json:"sort,optional"`
}

type AdminIntentionTagStatusReq struct {
	Code   string `path:"code"`
	Status string `json:"status"`
}

type IntentionResource struct {
	ResourceType string  `json:"resourceType"`
	SourceId     string  `json:"sourceId"`
	Title        string  `json:"title"`
	Subtitle     string  `json:"subtitle"`
	Price        float64 `json:"price"`
	Image        string  `json:"image"`
	OrderTarget  string  `json:"orderTarget"`
	TempleCode   string  `json:"templeCode,omitempty"`
	ServiceCode  string  `json:"serviceCode,omitempty"`
	MasterCode   string  `json:"masterCode,omitempty"` // 双轨大师资源：大师编码
}

type CustomerIntentionReq struct {
	Code string `form:"code,optional"`
	Page int    `form:"page,default=1"`
	Size int    `form:"size,default=20"`
}

type CustomerIntentionResp struct {
	Tags  []IntentionTag      `json:"tags"`
	Total int64               `json:"total"`
	List  []IntentionResource `json:"list"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
}

// ===== 商城台请求/响应 =====

type AdminProductListReq struct {
	CategoryId int64  `form:"categoryId,optional"`
	Keyword    string `form:"keyword,optional"`
	Status     string `form:"status,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type AdminProductListResp struct {
	Total int64     `json:"total"`
	List  []Product `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

type AdminProductCreateReq struct {
	Name              string  `json:"name"`
	CategoryId        int64   `json:"categoryId"`
	Description       string  `json:"description"`
	MainImage         string  `json:"mainImage"`
	Price             float64 `json:"price"`
	MarketPrice       float64 `json:"marketPrice"`
	Stock             int     `json:"stock"`
	Tags              string  `json:"tags"`
	FreightTemplateId int64   `json:"freightTemplateId"`
}

type AdminProductCreateResp struct {
	Id int64 `json:"id"`
}

type AdminProductDetailReq struct {
	Id int64 `path:"id"`
}

type AdminProductUpdateReq struct {
	Id                int64   `path:"id"`
	Name              string  `json:"name"`
	CategoryId        int64   `json:"categoryId"`
	Description       string  `json:"description"`
	MainImage         string  `json:"mainImage"`
	Price             float64 `json:"price"`
	MarketPrice       float64 `json:"marketPrice"`
	Stock             int     `json:"stock"`
	Tags              string  `json:"tags"`
	FreightTemplateId int64   `json:"freightTemplateId"`
}

type AdminProductDeleteReq struct {
	Id int64 `path:"id"`
}

type AdminProductStatusReq struct {
	Id     int64  `path:"id"`
	Status string `json:"status"`
}

type AdminProductStatusResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

type AdminSkuCreateReq struct {
	ProductId int64   `path:"id"`
	SpecName  string  `json:"specName"`
	SpecValue string  `json:"specValue"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	SkuNo     string  `json:"skuNo"`
}

type AdminSkuCreateResp struct {
	Id int64 `json:"id"`
}

type AdminSkuUpdateReq struct {
	ProductId int64   `path:"id"`
	SkuId     int64   `path:"skuId"`
	SpecName  string  `json:"specName"`
	SpecValue string  `json:"specValue"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	SkuNo     string  `json:"skuNo"`
}

type AdminCategoryListReq struct {
	ParentId int64 `form:"parentId,optional"`
	Page     int   `form:"page,default=1"`
	Size     int   `form:"size,default=50"`
}

type AdminCategoryListResp struct {
	Total int64             `json:"total"`
	List  []ProductCategory `json:"list"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

type AdminCategoryCreateReq struct {
	ParentId int64  `json:"parentId"`
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Sort     int    `json:"sort"`
}

type AdminCategoryCreateResp struct {
	Id int64 `json:"id"`
}

type AdminCategoryUpdateReq struct {
	Id       int64  `path:"id"`
	ParentId int64  `json:"parentId"`
	Name     string `json:"name"`
	Level    int    `json:"level"`
	Sort     int    `json:"sort"`
}

type AdminCategoryDeleteReq struct {
	Id int64 `path:"id"`
}
