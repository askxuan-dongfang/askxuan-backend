package provider

import (
	"context"
	"errors"
)

var ErrLiveUnavailable = errors.New("live provider unavailable")

type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

type MediaProvider interface {
	Name() string
	PrepareUpload(ctx context.Context, objectName, contentType string) (string, map[string]string, int64, error)
	Stat(ctx context.Context, objectName string) (ObjectInfo, error)
	PublicURL(objectName string) string
}

type LiveSession struct {
	ProviderRoomId string
	PushURL        string
	WatchURL       string
}

type LiveProvider interface {
	Name() string
	Configured() bool
	Start(ctx context.Context, roomNo string) (LiveSession, error)
	Close(ctx context.Context, providerRoomId string) error
}

type DisabledLiveProvider struct{ ProviderName string }

func (p DisabledLiveProvider) Name() string {
	if p.ProviderName == "" {
		return "disabled"
	}
	return p.ProviderName
}

func (DisabledLiveProvider) Configured() bool { return false }

func (DisabledLiveProvider) Start(context.Context, string) (LiveSession, error) {
	return LiveSession{}, ErrLiveUnavailable
}

func (DisabledLiveProvider) Close(context.Context, string) error { return nil }
