package types

type UploadCredentialReq struct {
	UserId      string `json:"userId,optional"`
	FileName    string `json:"fileName"`
	MediaType   string `json:"mediaType"`
	ContentType string `json:"contentType"`
	FileSize    int64  `json:"fileSize,optional"`
}

type UploadCredentialResp struct {
	MediaId       int64             `json:"mediaId"`
	UploadUrl     string            `json:"uploadUrl"`
	ObjectName    string            `json:"objectName"`
	ExpiresIn     int64             `json:"expiresIn"`
	UploadHeaders map[string]string `json:"uploadHeaders"`
}

type UploadCompleteReq struct {
	Id           int64  `path:"id"`
	UserId       string `json:"userId,optional"`
	CoverMediaId int64  `json:"coverMediaId,optional"`
	ETag         string `json:"etag,optional"`
}

type MediaDetailReq struct {
	Id int64 `path:"id"`
}

type Media struct {
	Id             int64   `json:"id"`
	MediaNo        string  `json:"mediaNo"`
	OwnerId        string  `json:"ownerId"`
	MediaType      string  `json:"mediaType"`
	ContentType    string  `json:"contentType"`
	FileName       string  `json:"fileName"`
	ObjectName     string  `json:"objectName"`
	Provider       string  `json:"provider"`
	ProviderTaskId string  `json:"providerTaskId"`
	Status         string  `json:"status"`
	AuditStatus    string  `json:"auditStatus"`
	PlaybackUrl    string  `json:"playbackUrl"`
	CoverUrl       string  `json:"coverUrl"`
	CoverMediaId   int64   `json:"coverMediaId"`
	Duration       float64 `json:"duration"`
	FileSize       int64   `json:"fileSize"`
	ErrorMessage   string  `json:"errorMessage"`
	CreateTime     string  `json:"createTime"`
	UpdateTime     string  `json:"updateTime"`
}

type MediaCallbackReq struct {
	MediaId        int64   `json:"mediaId"`
	ProviderTaskId string  `json:"providerTaskId,optional"`
	Status         string  `json:"status"`
	PlaybackUrl    string  `json:"playbackUrl,optional"`
	CoverUrl       string  `json:"coverUrl,optional"`
	Duration       float64 `json:"duration,optional"`
	ErrorMessage   string  `json:"errorMessage,optional"`
}

type AuditCallbackReq struct {
	MediaId     int64  `json:"mediaId"`
	AuditStatus string `json:"auditStatus"`
	Reason      string `json:"reason,optional"`
}

type EmptyResp struct {
	Success bool `json:"success"`
}

type LiveCapabilitiesResp struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	CanStart   bool   `json:"canStart"`
}

type LiveRoomCreateReq struct {
	OwnerId       string `json:"ownerId,optional"`
	MasterId      string `json:"masterId,optional"`
	Title         string `json:"title"`
	CoverMediaId  int64  `json:"coverMediaId,optional"`
	OpenimGroupId string `json:"openimGroupId,optional"`
}

type LiveRoomActionReq struct {
	Id      int64  `path:"id"`
	OwnerId string `json:"ownerId,optional"`
}

type LiveRoomDetailReq struct {
	Id int64 `path:"id"`
}

type LiveRoomListReq struct {
	MasterId string `form:"masterId,optional"`
	Limit    int    `form:"limit,optional"`
}

type LiveRoomListResp struct {
	List []LiveRoom `json:"list"`
}

type LiveBindOpenIMReq struct {
	Id            int64  `path:"id"`
	OwnerId       string `json:"ownerId,optional"`
	OpenimGroupId string `json:"openimGroupId"`
}

type LiveRoom struct {
	Id             int64  `json:"id"`
	RoomNo         string `json:"roomNo"`
	OwnerId        string `json:"ownerId"`
	MasterId       string `json:"masterId"`
	Title          string `json:"title"`
	CoverMediaId   int64  `json:"coverMediaId"`
	Provider       string `json:"provider"`
	Status         string `json:"status"`
	OpenimGroupId  string `json:"openimGroupId"`
	PushUrl        string `json:"pushUrl"`
	WatchUrl       string `json:"watchUrl"`
	ProviderRoomId string `json:"providerRoomId"`
	StartedAt      string `json:"startedAt"`
	EndedAt        string `json:"endedAt"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}
