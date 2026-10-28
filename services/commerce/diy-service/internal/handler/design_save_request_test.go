package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/askxuan/diy-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Exercise the same parser as designSaveHandler: encoding/json alone ignores
// go-zero's required-field tags and would miss this client/server regression.
func TestDesignSaveRequestParse(t *testing.T) {
	beads := make([]map[string]any, 36)
	for i := range beads {
		beads[i] = map[string]any{
			"slotId": "slot-" + strconv.Itoa(i), "position": i, "materialId": 1, "skuId": nil,
			"materialName": "白玉", "spec": "8mm", "unitPrice": 4,
			"subtype": "main_bead", "image": "/assets/diy/jade.jpg",
			"diameterMm": 8, "renderAssets": "{}",
		}
	}
	document, err := json.Marshal(map[string]any{
		"version": 2, "wristSizeMm": 165, "fitAllowanceMm": 8,
		"beads": beads, "cord": nil,
		"items": []map[string]any{{"materialId": 1, "spec": "8mm", "quantity": 36}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func() map[string]any {
		return map[string]any{
			"userId": "99001", "name": "我的手串11", "description": "1",
			"designData": string(document), "totalPrice": 144,
			"blessServiceCode": nil,
		}
	}
	parse := func(body map[string]any) (types.DesignSaveReq, error) {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/diy/designs", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		var req types.DesignSaveReq
		err = httpx.Parse(r, &req)
		return req, err
	}
	for _, tc := range []struct {
		name   string
		update bool
		status string
	}{
		{name: "create without publication status"},
		{name: "update without publication status", update: true},
		{name: "legacy status remains parseable", status: "public"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := request()
			if tc.update {
				body["id"], body["revision"] = int64(17), int64(3)
			}
			if tc.status != "" {
				body["status"] = tc.status
			}
			req, err := parse(body)
			if err != nil {
				t.Fatalf("client save request rejected: %v", err)
			}
			if req.Name != "我的手串11" || req.Description != "1" || req.DesignData != string(document) || req.Status != tc.status {
				t.Fatalf("save fields changed during parsing: %+v", req)
			}
			if tc.update && (req.Id != 17 || req.Revision != 3) {
				t.Fatalf("update identity changed: id=%d revision=%d", req.Id, req.Revision)
			}
		})
	}
	for _, field := range []string{"userId", "name", "designData", "totalPrice"} {
		t.Run("missing required "+field, func(t *testing.T) {
			body := request()
			delete(body, field)
			_, err := parse(body)
			if err == nil || !strings.Contains(err.Error(), `field "`+field+`" is not set`) {
				t.Fatalf("expected missing %s error, got %v", field, err)
			}
		})
	}
}
