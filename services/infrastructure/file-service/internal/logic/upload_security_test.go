package logic

import (
	"bytes"
	"context"
	"testing"

	"github.com/askxuan/file-service/internal/svc"
	"github.com/minio/minio-go/v7"
)

func TestUploadRejectsActiveContentBeforeStorage(t *testing.T) {
	logic := NewUploadLogic(context.Background(), &svc.ServiceContext{MinIOClient: &minio.Client{}})
	for _, payload := range [][]byte{
		[]byte("<html><script>alert(1)</script></html>"),
		[]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"),
	} {
		if _, err := logic.UploadFromReader("photo.png", "image/png", int64(len(payload)), bytes.NewReader(payload)); err == nil {
			t.Fatal("accepted active content based on client-supplied image type")
		}
	}
}
