package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/config"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/askxuan/file-service/internal/types"
	"github.com/minio/minio-go/v7"
)

var modelID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var photoObject = regexp.MustCompile(`^temp/[a-f0-9]{32}\.(jpg|png|webp)$`)

func ValidateModelAsset(format string, data []byte) error {
	if len(data) < 20 || len(data) > 32<<20 {
		return errors.New("model size invalid")
	}
	if format == "glb" {
		if string(data[:4]) != "glTF" || binary.LittleEndian.Uint32(data[4:8]) != 2 || int(binary.LittleEndian.Uint32(data[8:12])) != len(data) {
			return errors.New("invalid GLB header")
		}
		n := int(binary.LittleEndian.Uint32(data[12:16]))
		if n < 2 || n > 4<<20 || n%4 != 0 || 20+n > len(data) || string(data[16:20]) != "JSON" {
			return errors.New("invalid GLB JSON")
		}
		var doc map[string]any
		if json.Unmarshal(data[20:20+n], &doc) != nil {
			return errors.New("invalid GLB manifest")
		}
		if _, ok := doc["meshes"].([]any); !ok {
			return errors.New("GLB has no mesh")
		}
		if x, _ := doc["skins"].([]any); len(x) > 0 {
			return errors.New("skinned models require authoring conversion")
		}
		if x, _ := doc["animations"].([]any); len(x) > 0 {
			return errors.New("animated models require authoring conversion")
		}
		var safe func(any) bool
		safe = func(v any) bool {
			switch x := v.(type) {
			case map[string]any:
				for k, v := range x {
					if k == "uri" {
						s, ok := v.(string)
						if !ok || (!strings.HasPrefix(s, "data:image/png;base64,") && !strings.HasPrefix(s, "data:image/jpeg;base64,")) {
							return false
						}
					}
					if !safe(v) {
						return false
					}
				}
			case []any:
				for _, v := range x {
					if !safe(v) {
						return false
					}
				}
			}
			return true
		}
		if !safe(doc) {
			return errors.New("external GLB assets are not supported")
		}
		offset := 20 + n
		var binaryChunk []byte
		for offset < len(data) {
			if offset+8 > len(data) {
				return errors.New("truncated GLB")
			}
			size := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
			if size%4 != 0 || size < 0 || offset+8+size > len(data) {
				return errors.New("invalid GLB chunk")
			}
			if string(data[offset+4:offset+8]) != "BIN\x00" || binaryChunk != nil {
				return errors.New("unsupported GLB chunk")
			}
			binaryChunk = data[offset+8 : offset+8+size]
			offset += 8 + size
		}
		return validateGLBBudget(doc, binaryChunk)
	}
	if format != "usdz" {
		return errors.New("unsupported model format")
	}
	reader, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil || len(reader.File) < 1 || len(reader.File) > 256 {
		return errors.New("invalid USDZ archive")
	}
	seen := map[string]bool{}
	var total uint64
	texturePixels := 0
	textureCount := 0
	hasScene := false
	for _, f := range reader.File {
		name := f.Name
		cleanName := strings.TrimSuffix(name, "/")
		if seen[name] || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") || cleanName == "" || path.Clean(cleanName) != cleanName || strings.HasPrefix(name, "../") {
			return errors.New("unsafe USDZ entry")
		}
		seen[name] = true
		if f.FileInfo().IsDir() {
			if f.UncompressedSize64 != 0 {
				return errors.New("invalid USDZ directory")
			}
			continue
		}
		if !f.Mode().IsRegular() {
			return errors.New("unsafe USDZ entry")
		}
		if f.Method != zip.Store || f.Flags&1 != 0 || f.UncompressedSize64 > 32<<20 {
			return errors.New("unsupported USDZ compression")
		}
		ext := strings.ToLower(path.Ext(name))
		switch ext {
		case ".usd", ".usda", ".usdc":
			hasScene = true
		case ".png", ".jpg", ".jpeg":
			textureCount++
			if textureCount > 16 {
				return errors.New("USDZ texture count exceeds limit")
			}
		default:
			return errors.New("unsupported USDZ asset")
		}
		total += f.UncompressedSize64
		if total > 64<<20 {
			return errors.New("USDZ expansion exceeds limit")
		}
		r, e := f.Open()
		if e != nil {
			return errors.New("invalid USDZ member")
		}
		b, e := io.ReadAll(io.LimitReader(r, (32<<20)+1))
		r.Close()
		if e != nil || uint64(len(b)) != f.UncompressedSize64 {
			return errors.New("corrupt USDZ member")
		}
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			pixels, err := decodedTexturePixels(b)
			if err != nil {
				return err
			}
			texturePixels += pixels
			if texturePixels > 16*1024*1024 {
				return errors.New("USDZ texture pixels exceed limit")
			}
		}
		if ext == ".usda" || ext == ".usd" {
			if bytes.HasPrefix(b, []byte("#usda")) {
				for _, ref := range regexp.MustCompile(`@([^@]*)@`).FindAllSubmatch(b, -1) {
					v := string(ref[1])
					if strings.Contains(v, ":") || strings.HasPrefix(v, "/") || strings.Contains(v, "..") || strings.Contains(v, "\\") {
						return errors.New("external USD reference")
					}
				}
			} else if !bytes.HasPrefix(b, []byte("PXR-USDC")) {
				return errors.New("invalid USD scene")
			}
		}
		if ext == ".usdc" && !bytes.HasPrefix(b, []byte("PXR-USDC")) {
			return errors.New("invalid USD crate")
		}
	}
	if !hasScene {
		return errors.New("USDZ has no scene")
	}
	return nil
}

func StoreModel(ctx context.Context, s *svc.ServiceContext, id, format string, data []byte) (*types.UploadResp, error) {
	if !modelID.MatchString(id) || ValidateModelAsset(format, data) != nil {
		return nil, common.NewBizError(7010, "模型格式无效，仅接受完整、安全的 GLB 或 USDZ，最大 32MB")
	}
	if s.MinIOClient == nil {
		return nil, common.ErrOssService
	}
	hash := sha256.Sum256(data)
	key := "diy-models/" + id + "/" + hex.EncodeToString(hash[:]) + "." + format
	mime := "model/gltf-binary"
	if format == "usdz" {
		mime = "model/vnd.usdz+zip"
	}
	_, e := s.MinIOClient.PutObject(ctx, s.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mime})
	if e != nil {
		return nil, common.ErrOssService
	}
	return &types.UploadResp{ObjectName: key, Url: buildObjectURL(s.Config.MinIO, s.Bucket, key), Size: int64(len(data)), ContentType: mime}, nil
}
func photoKey(c config.MinIOConf, bucket, raw string) (string, error) {
	base, e := url.Parse(buildObjectURL(c, bucket, ""))
	if e != nil {
		return "", common.ErrParam
	}
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || (!u.IsAbs() && u.Host != "") || u.RawQuery != "" || u.Fragment != "" {
		return "", common.ErrParam
	}
	if u.IsAbs() && (u.Scheme != base.Scheme || u.Host != base.Host) {
		return "", common.ErrParam
	}
	if !u.IsAbs() && !strings.HasPrefix(raw, "/") {
		return "", common.ErrParam
	}
	prefix := strings.TrimRight(base.Path, "/") + "/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", common.ErrParam
	}
	key := strings.TrimPrefix(u.Path, prefix)
	if !photoObject.MatchString(key) {
		return "", common.ErrParam
	}
	return key, nil
}
func ModelingPhoto(ctx context.Context, s *svc.ServiceContext, raw string) ([]byte, string, error) {
	key, e := photoKey(s.Config.MinIO, s.Bucket, raw)
	if e != nil {
		return nil, "", e
	}
	if s.MinIOClient == nil {
		return nil, "", common.ErrOssService
	}
	object, e := s.MinIOClient.GetObject(ctx, s.Bucket, key, minio.GetObjectOptions{})
	if e != nil {
		return nil, "", common.ErrOssService
	}
	defer object.Close()
	stat, e := object.Stat()
	if e != nil || stat.Size > 10<<20 {
		return nil, "", common.ErrParam
	}
	b, e := io.ReadAll(io.LimitReader(object, (10<<20)+1))
	mime := http.DetectContentType(b)
	if e != nil || len(b) > 10<<20 || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") {
		return nil, "", common.ErrParam
	}
	return b, mime, nil
}
