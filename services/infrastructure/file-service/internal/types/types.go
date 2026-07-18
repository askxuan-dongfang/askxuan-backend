package types

// PresignReq 预签名请求
type PresignReq struct {
	FileName   string `form:"fileName"`
	ObjectType string `form:"objectType,optional,default=temp"`
	Operate    string `form:"operate,optional,default=upload"`
	ObjectName string `form:"objectName,optional"`
}

// PresignResp 预签名响应
type PresignResp struct {
	UploadUrl  string `json:"uploadUrl"`
	ObjectName string `json:"objectName"`
	ExpiresIn  int64  `json:"expiresIn"`
}

// UploadResp 上传响应
type UploadResp struct {
	ObjectName  string `json:"objectName"`
	Url         string `json:"url"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

type BackupItem struct {
	Id         string  `json:"id"`
	Filename   string  `json:"filename"`
	Size       float64 `json:"size"`
	SizeBytes  int64   `json:"sizeBytes"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Time       string  `json:"time"`
	ObjectName string  `json:"objectName"`
}

type BackupListResp struct {
	List []BackupItem `json:"list"`
}

type BackupDownloadResp struct {
	Url       string `json:"url"`
	ExpiresIn int64  `json:"expiresIn"`
}

type BackupRestoreReq struct {
	Confirm string `json:"confirm"`
}

type BackupActionResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
