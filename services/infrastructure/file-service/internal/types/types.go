package types

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
