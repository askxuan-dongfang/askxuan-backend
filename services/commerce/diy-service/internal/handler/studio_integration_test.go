package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in test: ONLY uses the disposable database created by the publication
// script, or the historical dedicated local test port 13386.
func TestStudioIntegration(t *testing.T) {
	if os.Getenv("DIY_STUDIO_TEST") != "1" {
		t.Skip("requires disposable MySQL on 127.0.0.1:13386")
	}
	ctx := context.Background()
	dsn := os.Getenv("DIY_PUBLICATION_TEST_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:13386)/askxuan_diy?charset=utf8mb4&parseTime=false&loc=Local"
	}
	if !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_diy?") {
		t.Fatal("local test schema only")
	}
	db := sqlx.NewMysql(dsn)
	for _, table := range []string{"diy_order", "diy_design", "material_sku", "material"} {
		if _, err := db.ExecCtx(ctx, "DELETE FROM "+table); err != nil {
			t.Fatal(err)
		}
	}
	secret := "diy-studio-local-test-only"
	s := &svc.ServiceContext{Config: config.Config{AuthSecret: secret}, DB: db, DiyDesignModel: model.NewDiyDesignModel(db), MaterialModel: model.NewMaterialModel(db), MaterialSkuModel: model.NewMaterialSkuModel(db), DiyOrderModel: model.NewDiyOrderModel(db)}
	mat, err := s.MaterialModel.Insert(ctx, &model.Material{Name: "青金石", Spec: "10mm", UnitPrice: 25, Unit: "颗", Category: "main_bead", DiameterMm: 10, ColorHex: "#244680", TextureKey: "fleck", Image: "/assets/diy/sources/lapis.jpg", RenderAssets: `{"beadImageUrl": "/assets/diy/sources/lapis.jpg", "imageCrop": {"x": 158, "y": 36, "width": 470, "height": 470, "imageWidth": 800, "imageHeight": 600}, "source": "licensed", "attribution": "Adam Ognisty / CC BY-SA 3.0; 原图画布裁切"}`, Stock: 50})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MaterialModel.Update(ctx, mat); err != nil {
		t.Fatalf("unchanged SKU must allow asset-only edits: %v", err)
	}
	cord, err := s.MaterialModel.Insert(ctx, &model.Material{Name: "弹力绳", Spec: "1mm", UnitPrice: 3, Unit: "条", Category: "cord", Stock: 50})
	if err != nil {
		t.Fatal(err)
	}
	server, err := rest.NewServer(rest.RestConf{Host: "127.0.0.1", Port: 18088})
	if err != nil {
		t.Fatal(err)
	}
	RegisterHandlers(server, s)
	go server.Start()
	defer server.Stop()
	for i := 0; i < 50; i++ {
		r, e := http.Get("http://127.0.0.1:18088/api/v1/diy/materials")
		if e == nil {
			r.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	token := func(id int64, role string) string {
		v, e := common.GenAccessToken(secret, common.TokenInfo{UserId: id, UserType: "user", Roles: []string{role}, ClientID: "customer"}, 7200)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	owner, other, admin := token(99001, "customer"), token(99002, "customer"), token(99003, "platform_super")
	call := func(method, path, tok string, body any, ok bool) map[string]any {
		raw, _ := json.Marshal(body)
		if body == nil {
			raw = nil
		}
		r, _ := http.NewRequest(method, "http://127.0.0.1:18088/api/v1"+path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-User-Id", "99001")
		if tok != "" {
			r.Header.Set("Authorization", "Bearer "+tok)
		}
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var env struct {
			Code    int            `json:"code"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		}
		if e = json.NewDecoder(res.Body).Decode(&env); e != nil {
			t.Fatal(e)
		}
		if (env.Code == 0) != ok {
			t.Fatalf("%s %s: code=%d message=%s expected success=%v", method, path, env.Code, env.Message, ok)
		}
		return env.Data
	}
	document, _ := json.Marshal(map[string]any{"version": 2, "wristSizeMm": 160, "fitAllowanceMm": 8, "beads": []any{map[string]any{"materialId": mat.Id, "spec": "10mm", "unitPrice": .01, "image": "https://evil.invalid/fake.jpg"}}, "cord": map[string]any{"materialId": cord.Id, "spec": "1mm", "quantity": 999}, "items": []any{map[string]any{"materialId": 9999, "quantity": 99}}})
	// Current H5 and iOS clients omit status: saving a draft cannot publish it.
	save := map[string]any{"userId": "99002", "name": "青金 · 月色", "description": "独立测试作品", "designData": string(document), "totalPrice": .01}
	call("POST", "/diy/designs", "", save, false)
	saved := call("POST", "/diy/designs", owner, save, true)
	id := int64(saved["id"].(float64))
	path := fmt.Sprintf("/diy/designs/%d", id)
	d := call("GET", path, owner, nil, true)
	if d["userId"] != "99001" || d["status"] != "private" || d["totalPrice"].(float64) != 28 {
		t.Fatalf("untrusted fields accepted: %#v", d)
	}
	// Legacy clients may still send status, but it never grants publication.
	save["status"] = "public"
	forged := call("POST", "/diy/designs", owner, save, true)
	forgedDesign := call("GET", fmt.Sprintf("/diy/designs/%.0f", forged["id"]), owner, nil, true)
	if forgedDesign["status"] != "private" || forgedDesign["publication"] != nil {
		t.Fatal("client status bypassed publication gate", forgedDesign)
	}
	delete(save, "status")
	var doc struct {
		Items []struct {
			Quantity int `json:"quantity"`
		}
		Beads []struct {
			Image string `json:"image"`
		}
	}
	if e := json.Unmarshal([]byte(d["designData"].(string)), &doc); e != nil {
		t.Fatal(e)
	}
	if len(doc.Items) != 2 || doc.Items[0].Quantity != 1 || doc.Items[1].Quantity != 1 || doc.Beads[0].Image != mat.Image {
		t.Fatalf("invalid canonical document: %+v", doc)
	}
	call("GET", path, "", nil, false)
	call("GET", path, other, nil, false)
	call("POST", path+"/copy", other, map[string]any{}, false)
	call("POST", "/diy/orders/availability", other, map[string]any{"designId": id}, false)
	call("PUT", path+"/status", other, map[string]any{"revision": 1, "status": "public"}, false)
	call("PUT", path+"/status", owner, map[string]any{"revision": 1, "status": "public"}, false)
	if d["publicationEligibility"].(map[string]any)["reason"] != "order_required" {
		t.Fatal("missing authoritative publishing gate", d)
	}
	// Fixture a confirmed, genuinely payable channel in the isolated database.
	// This is an integration state fixture, not a real payment gateway execution.
	design, err := s.DiyDesignModel.FindOne(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	designSnapshot, _ := json.Marshal(design)
	priceSnapshot, _ := json.Marshal(map[string]any{"materialFee": 28, "blessFee": 0, "totalFee": 28, "items": []map[string]any{
		{"materialId": mat.Id, "spec": "10mm", "quantity": 1, "unitPrice": 25},
		{"materialId": cord.Id, "spec": "1mm", "quantity": 1, "unitPrice": 3},
	}})
	order, err := s.DiyOrderModel.Insert(ctx, &model.DiyOrder{OrderNo: fmt.Sprintf("PUBHTTP%d", time.Now().UnixNano()), UserId: "99001", DesignId: id, MaterialFee: 28, TotalFee: 28, Status: model.DiyStatusPendingReview, PaymentStatus: "success", AddressId: 1, DesignSnapshot: string(designSnapshot), PricingSnapshot: string(priceSnapshot)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecCtx(ctx, "INSERT INTO askxuan_payment.payment(payment_no,user_id,order_type,order_no,amount,channel,status) VALUES(?,'99001','diy_order',?,28,'alipay','success')", order.OrderNo, order.OrderNo); err != nil {
		t.Fatal(err)
	}
	call("PUT", path+"/status", owner, map[string]any{"revision": 1, "status": "public"}, false)
	if _, err = db.ExecCtx(ctx, "UPDATE diy_order SET status='in_making' WHERE id=?", order.Id); err != nil {
		t.Fatal(err)
	}
	call("PUT", path+"/status", owner, map[string]any{"revision": 1, "status": "public"}, true)
	public := call("GET", path+"?view=published", owner, nil, true)
	if public["publicationEligibility"] != nil || public["publication"] == nil {
		t.Fatal("published view leaked eligibility or omitted fixed version", public)
	}
	call("GET", path, "", nil, true)
	copy := call("POST", path+"/copy", other, map[string]any{}, true)
	if copy["userId"] != "99002" || copy["status"] != "private" || int64(copy["sourceDesignId"].(float64)) != id || copy["sourcePublicationId"].(float64) < 1 || copy["publicationEligibility"].(map[string]any)["canPublish"] != false {
		t.Fatalf("copy ownership: %#v", copy)
	}
	save["id"] = id
	save["revision"] = 1
	call("POST", "/diy/designs", owner, save, false)
	call("GET", "/admin/diy/designs", owner, nil, false)
	call("GET", "/admin/diy/designs", admin, nil, true)
	call("PUT", "/admin"+path+"/status", admin, map[string]any{"revision": 2, "status": "rejected"}, true)
	call("GET", path, "", nil, false)
	moderated := call("GET", path, owner, nil, true)["publicationEligibility"].(map[string]any)
	if moderated["canPublish"] != false || moderated["reason"] != "moderation_required" {
		t.Fatal("moderation missing from authoritative eligibility", moderated)
	}
	call("GET", fmt.Sprintf("/diy/designs/%.0f", copy["id"]), other, nil, true)
	call("PUT", path+"/status", owner, map[string]any{"revision": 3, "status": "public"}, false)
	save["revision"] = 3
	call("POST", "/diy/designs", owner, save, true)
	if _, err = db.ExecCtx(ctx, "UPDATE material_sku SET stock=0 WHERE material_id=?", mat.Id); err != nil {
		t.Fatal(err)
	}
	call("PUT", path+"/status", owner, map[string]any{"revision": 4, "status": "public"}, true)
	availability := call("POST", "/diy/orders/availability", other, map[string]any{"designId": id}, true)
	if availability["orderable"] != false {
		t.Fatal("publishing reserved or bypassed depleted stock")
	}
	if _, err = db.ExecCtx(ctx, "UPDATE material_sku SET stock=50 WHERE material_id=?", mat.Id); err != nil {
		t.Fatal(err)
	}
	frozen := call("GET", path+"?view=published", owner, nil, true)
	save["revision"], save["name"], save["status"] = 5, "草稿新名字", "private"
	call("POST", "/diy/designs", owner, save, true)
	draft := call("GET", path, owner, nil, true)
	stillPublished := call("GET", path+"?view=published", owner, nil, true)
	if draft["status"] != "public" || draft["draftChanged"] != true || draft["name"] != "草稿新名字" || stillPublished["name"] != frozen["name"] || stillPublished["designData"] != frozen["designData"] {
		t.Fatal("saving draft altered public snapshot")
	}
	afterEditCopy := call("POST", path+"/copy", other, map[string]any{}, true)
	if afterEditCopy["name"] != frozen["name"].(string)+" · 副本" {
		t.Fatal("copy took mutable owner draft")
	}
	call("PUT", path+"/status", owner, map[string]any{"revision": 6, "status": "private"}, true)
	call("GET", path+"?view=published", owner, nil, false)
	var stored string
	if err = db.QueryRowCtx(ctx, &stored, "SELECT design_snapshot FROM diy_order WHERE id=?", order.Id); err != nil || stored != string(designSnapshot) {
		t.Fatal("unpublishing changed paid order", err)
	}
	if _, err = db.ExecCtx(ctx, "UPDATE diy_design SET status='public',publication_id=0 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	legacy := call("GET", path, owner, nil, true)
	if legacy["status"] != "private" || legacy["publication"] != nil || legacy["draftChanged"] != false {
		t.Fatal("unqualified legacy work mislabelled as published")
	}
	call("GET", path, "", nil, false)
	t.Run("plaza response and publication labels", func(t *testing.T) {
		if _, err = db.ExecCtx(ctx, "INSERT INTO askxuan_user.user(id,mobile,nickname) VALUES(99001,'secret-owner-99001','HTTP标签作者'),(99002,'secret-copy-99002','') ON DUPLICATE KEY UPDATE nickname=VALUES(nickname)"); err != nil {
			t.Fatal(err)
		}
		fresh := call("POST", "/diy/designs", owner, map[string]any{"userId": "99001", "name": "HTTP广场作品", "designData": string(document), "totalPrice": 28}, true)
		freshID := int64(fresh["id"].(float64))
		freshPath := fmt.Sprintf("/diy/designs/%d", freshID)
		freshDesign, err := s.DiyDesignModel.FindOne(ctx, freshID)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(freshDesign)
		confirmed, err := s.DiyOrderModel.Insert(ctx, &model.DiyOrder{OrderNo: fmt.Sprintf("PUBTAGS%d", time.Now().UnixNano()), UserId: "99001", DesignId: freshID, MaterialFee: 28, TotalFee: 28, Status: model.DiyStatusInMaking, PaymentStatus: "success", AddressId: 1, DesignSnapshot: string(raw), PricingSnapshot: string(priceSnapshot)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecCtx(ctx, "INSERT INTO askxuan_payment.payment(payment_no,user_id,order_type,order_no,amount,channel,status) VALUES(?,'99001','diy_order',?,28,'alipay','success')", confirmed.OrderNo, confirmed.OrderNo); err != nil {
			t.Fatal(err)
		}
		call("PUT", freshPath+"/status", owner, map[string]any{"revision": 1, "status": "public", "tags": map[string]any{"fiveElements": []string{"wood", "water"}, "meanings": []string{"sleep"}}}, true)
		first := call("GET", freshPath+"?view=published", "", nil, true)
		firstPublication := int64(first["publication"].(map[string]any)["id"].(float64))
		var firstSnapshot string
		if err = db.QueryRowCtx(ctx, &firstSnapshot, "SELECT design_snapshot FROM diy_design_publication WHERE id=?", firstPublication); err != nil {
			t.Fatal(err)
		}
		call("POST", freshPath+"/copy", owner, map[string]any{}, true)
		for i := 0; i < 2; i++ {
			call("POST", freshPath+"/copy", other, map[string]any{}, true)
		}
		public := call("GET", freshPath, "", nil, true)
		listed := call("GET", "/diy/designs?size=1&keyword="+url.QueryEscape("HTTP标签作者")+"&fiveElement=water&meaning=sleep", "", nil, true)
		if listed["total"].(float64) != 1 {
			t.Fatal("HTTP filtering did not use nickname and tags", listed)
		}
		listedDesign := listed["list"].([]any)[0].(map[string]any)
		owned := call("GET", freshPath, owner, nil, true)
		mine := call("GET", "/diy/my-designs", owner, nil, true)
		var mineDesign map[string]any
		for _, row := range mine["list"].([]any) {
			candidate := row.(map[string]any)
			if int64(candidate["id"].(float64)) == freshID {
				mineDesign = candidate
			}
		}
		for _, response := range []map[string]any{public, listedDesign, owned, mineDesign} {
			if response == nil || response["authorName"] != "HTTP标签作者" || response["useCount"].(float64) != 2 || len(response["fiveElements"].([]any)) != 2 || response["meanings"].([]any)[0] != "sleep" {
				t.Fatal("inconsistent plaza metadata", response)
			}
			encoded, _ := json.Marshal(response)
			if strings.Contains(string(encoded), "secret-") || response["mobile"] != nil || response["email"] != nil {
				t.Fatal("public author metadata exposed private fields")
			}
		}
		// Existing iOS and admin payloads omit tags; a metadata-only publication
		// reuses the already confirmed order and must keep the previous labels.
		preserved := call("PUT", "/admin"+freshPath+"/status", admin, map[string]any{"revision": 2, "status": "public"}, true)
		if len(preserved["fiveElements"].([]any)) != 2 || preserved["meanings"].([]any)[0] != "sleep" {
			t.Fatal("legacy admin status cleared labels", preserved)
		}
		cleared := call("PUT", freshPath+"/status", owner, map[string]any{"revision": 3, "status": "public", "tags": map[string]any{"fiveElements": []string{}, "meanings": []string{}}}, true)
		if len(cleared["fiveElements"].([]any)) != 0 || len(cleared["meanings"].([]any)) != 0 {
			t.Fatal("explicit clear did not persist", cleared)
		}
		var unchanged string
		if err = db.QueryRowCtx(ctx, &unchanged, "SELECT design_snapshot FROM diy_design_publication WHERE id=?", firstPublication); err != nil || unchanged != firstSnapshot {
			t.Fatal("labels changed historical snapshot", err)
		}
		call("PUT", freshPath+"/status", owner, map[string]any{"revision": 4, "status": "public", "tags": map[string]any{"meanings": []string{"unknown"}}}, false)
		call("GET", "/diy/designs?fiveElement=invalid", "", nil, false)
	})
	t.Log("PASS: JWT ownership, canonical pricing, paid and approved gate, fixed published view, copy isolation, CAS, moderation, stock checkout gate, draft/public independence, unpublish and legacy response")
	if fixture := os.Getenv("DIY_STUDIO_BROWSER_FIXTURE"); fixture != "" {
		raw, _ := json.Marshal(map[string]any{"owner": owner, "other": other, "admin": admin, "designId": id, "materialId": mat.Id, "cordId": cord.Id})
		if err = os.WriteFile(fixture, raw, 0600); err != nil {
			t.Fatal(err)
		}
		select {}
	}
}
