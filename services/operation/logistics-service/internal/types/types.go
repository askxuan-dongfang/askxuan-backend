package types

// ExpressCompany 快递公司
type ExpressCompany struct {
	Id              int64  `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	LogoUrl         string `json:"logoUrl"`
	CustomerService string `json:"customerService"`
	Sort            int    `json:"sort"`
	Status          string `json:"status"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
}

type ExpressListReq struct {
	Code   string `form:"code,optional"`
	Name   string `form:"name,optional"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type ExpressListResp struct {
	Total int64            `json:"total"`
	List  []ExpressCompany `json:"list"`
	Page  int              `json:"page"`
	Size  int              `json:"size"`
}

type ExpressCreateReq struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	LogoUrl         string `json:"logoUrl,optional"`
	CustomerService string `json:"customerService,optional"`
	Sort            int    `json:"sort,default=0"`
}

type ExpressCreateResp struct {
	Id int64 `json:"id"`
}

type ExpressUpdateReq struct {
	Id              int64  `path:"id"`
	Name            string `json:"name,optional"`
	LogoUrl         string `json:"logoUrl,optional"`
	CustomerService string `json:"customerService,optional"`
	Sort            int    `json:"sort,optional"`
	Status          string `json:"status,optional"`
}

// FreightTemplate 运费模板
type FreightTemplate struct {
	Id           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	FreeShipping int    `json:"freeShipping"`
	Config       string `json:"config"`
	Status       string `json:"status"`
	CreateTime   string `json:"createTime"`
	UpdateTime   string `json:"updateTime"`
}

type FreightTemplateListReq struct {
	Name   string `form:"name,optional"`
	Type   string `form:"type,optional"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type FreightTemplateListResp struct {
	Total int64            `json:"total"`
	List  []FreightTemplate `json:"list"`
	Page  int              `json:"page"`
	Size  int              `json:"size"`
}

type FreightTemplateCreateReq struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	FreeShipping int    `json:"freeShipping,default=0"`
	Config       string `json:"config"`
}

type FreightTemplateCreateResp struct {
	Id int64 `json:"id"`
}

type FreightTemplateUpdateReq struct {
	Id           int64  `path:"id"`
	Name         string `json:"name,optional"`
	Type         string `json:"type,optional"`
	FreeShipping int    `json:"freeShipping,optional"`
	Config       string `json:"config,optional"`
	Status       string `json:"status,optional"`
}

// 物流追踪
type TrackTrace struct {
	Time string `json:"time"`
	Desc string `json:"desc"`
}

type TrackQueryReq struct {
	TrackingNo string `path:"trackingNo"`
}

type TrackQueryResp struct {
	TrackingNo   string       `json:"trackingNo"`
	ExpressCode  string       `json:"expressCode"`
	ExpressName  string       `json:"expressName"`
	BizType      string       `json:"bizType"`
	BizNo        string       `json:"bizNo"`
	Status       string       `json:"status"`
	Traces       []TrackTrace `json:"traces"`
	LastSyncTime string       `json:"lastSyncTime"`
}

type TracksBatchSyncReq struct {
	TrackingNos []string `json:"trackingNos,optional"`
}

type TracksBatchSyncResp struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}
