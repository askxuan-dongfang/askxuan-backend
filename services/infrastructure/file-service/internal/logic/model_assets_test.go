package logic

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/askxuan/file-service/internal/config"
	"image"
	"image/png"
	"io/fs"
	"testing"
)

func testGLB(doc string) []byte {
	for len(doc)%4 != 0 {
		doc += " "
	}
	b := make([]byte, 20+len(doc))
	copy(b, "glTF")
	binary.LittleEndian.PutUint32(b[4:], 2)
	binary.LittleEndian.PutUint32(b[8:], uint32(len(b)))
	binary.LittleEndian.PutUint32(b[12:], uint32(len(doc)))
	copy(b[16:], "JSON")
	copy(b[20:], doc)
	return b
}
func testUSDZ(t *testing.T, name string, mode fs.FileMode) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	h := &zip.FileHeader{Name: name, Method: zip.Store}
	h.SetMode(mode)
	f, e := w.CreateHeader(h)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.Write([]byte("#usda 1.0\ndef Xform \"Model\" {}"))
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestModelAssetBoundaries(t *testing.T) {
	if e := ValidateModelAsset("glb", testGLB(validGLBDoc)); e != nil {
		t.Fatal(e)
	}
	for _, doc := range []string{`{"meshes":[{}],"images":[{"uri":"https://evil.example/x.png"}]}`, `{"meshes":[{}],"buffers":[{"uri":"file:///x"}]}`, `{"meshes":[{}],"skins":[{}]}`, `{"meshes":[{}],"animations":[{}]}`} {
		if ValidateModelAsset("glb", testGLB(doc)) == nil {
			t.Fatal("unsafe GLB accepted", doc)
		}
	}
	if e := ValidateModelAsset("usdz", testUSDZ(t, "model.usda", 0644)); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct {
		name string
		mode fs.FileMode
	}{{"../model.usda", 0644}, {"/model.usda", 0644}, {"model.usda", fs.ModeSymlink | 0644}, {"run.html", 0644}} {
		if ValidateModelAsset("usdz", testUSDZ(t, v.name, v.mode)) == nil {
			t.Fatal("unsafe USDZ accepted", v)
		}
	}
	if ValidateModelAsset("glb", bytes.Repeat([]byte("x"), (32<<20)+1)) == nil {
		t.Fatal("oversized model accepted")
	}
}
func TestModelingPhotoOnlyReadsOwnUploadedObject(t *testing.T) {
	c := config.MinIOConf{PublicBaseURL: "https://site.example/media"}
	object := "temp/0123456789abcdef0123456789abcdef.png"
	for _, raw := range []string{"https://site.example/media/bucket/" + object, "/media/bucket/" + object} {
		if key, e := photoKey(c, "bucket", raw); e != nil || key != object {
			t.Fatal(raw, key, e)
		}
	}
	for _, raw := range []string{"https://evil.example/media/bucket/" + object, "http://127.0.0.1/media/bucket/" + object, "//evil.example/media/bucket/" + object, "/media/bucket/../../secret", "/media/other/" + object, "/media/bucket/diy-models/model.glb"} {
		if _, e := photoKey(c, "bucket", raw); e == nil {
			t.Fatal("non-owned photo accepted", raw)
		}
	}
}

const validGLBDoc = `{"asset":{"version":"2.0"},"accessors":[{"count":3,"type":"VEC3","componentType":5126}],"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],"nodes":[{"mesh":0}],"scenes":[{"nodes":[0]}]}`

func TestModelDecodedBudgets(t *testing.T) {
	tests := map[string]func(map[string]any){
		"accessor allocation": func(d map[string]any) { d["accessors"].([]any)[0].(map[string]any)["count"] = 1000000000 },
		"mesh instance multiplication": func(d map[string]any) {
			d["accessors"].([]any)[0].(map[string]any)["count"] = 60000
			d["nodes"] = []any{map[string]any{"mesh": 0}, map[string]any{"mesh": 0}}
		},
		"shared node": func(d map[string]any) {
			d["nodes"] = []any{map[string]any{"children": []int{2}}, map[string]any{"children": []int{2}}, map[string]any{"mesh": 0}}
		},
		"cyclic node":         func(d map[string]any) { d["nodes"] = []any{map[string]any{"children": []int{0}, "mesh": 0}} },
		"compressed geometry": func(d map[string]any) { d["extensionsUsed"] = []string{"KHR_draco_mesh_compression"} },
		"sparse accessor": func(d map[string]any) {
			d["accessors"].([]any)[0].(map[string]any)["sparse"] = map[string]any{"count": 3}
		},
		"too many nodes": func(d map[string]any) { d["nodes"] = make([]any, 129) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal([]byte(validGLBDoc), &d); err != nil {
				t.Fatal(err)
			}
			mutate(d)
			b, _ := json.Marshal(d)
			if ValidateModelAsset("glb", testGLB(string(b))) == nil {
				t.Fatal("unsafe allocation accepted")
			}
		})
	}
	for _, test := range []struct {
		name                 string
		width, height, count int
		wantOK               bool
	}{
		{"valid embedded", 32, 32, 1, true}, {"oversized edge", 4097, 1, 1, false}, {"aggregate pixels", 2048, 2048, 5, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, test.width, test.height))); err != nil {
				t.Fatal(err)
			}
			var d map[string]any
			_ = json.Unmarshal([]byte(validGLBDoc), &d)
			images := make([]any, test.count)
			for i := range images {
				images[i] = map[string]any{"uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())}
			}
			d["images"] = images
			b, _ := json.Marshal(d)
			if got := ValidateModelAsset("glb", testGLB(string(b))) == nil; got != test.wantOK {
				t.Fatalf("accepted=%v want=%v", got, test.wantOK)
			}
		})
	}
}

func TestUSDZRejectsCompressedExternalAndOversizedTextures(t *testing.T) {
	var hugePNG bytes.Buffer
	if err := png.Encode(&hugePNG, image.NewNRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"compressed", "external", "duplicate", "texture"} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			w := zip.NewWriter(&archive)
			method := uint16(zip.Store)
			if name == "compressed" {
				method = zip.Deflate
			}
			f, err := w.CreateHeader(&zip.FileHeader{Name: "model.usda", Method: method})
			if err != nil {
				t.Fatal(err)
			}
			data := "#usda 1.0\ndef Xform \"Model\" {}"
			if name == "external" {
				data += "\nasset texture = @https://evil.example/image.png@"
			}
			_, _ = f.Write([]byte(data))
			if name == "duplicate" {
				f, _ = w.CreateHeader(&zip.FileHeader{Name: "model.usda", Method: zip.Store})
				_, _ = f.Write([]byte(data))
			}
			if name == "texture" {
				f, _ = w.CreateHeader(&zip.FileHeader{Name: "texture.png", Method: zip.Store})
				_, _ = f.Write(hugePNG.Bytes())
			}
			if err = w.Close(); err != nil {
				t.Fatal(err)
			}
			if ValidateModelAsset("usdz", archive.Bytes()) == nil {
				t.Fatal("unsafe USDZ accepted")
			}
		})
	}
}
