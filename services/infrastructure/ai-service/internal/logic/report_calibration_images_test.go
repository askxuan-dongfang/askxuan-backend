package logic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportCalibrationImagesKeepPayloadPrivateAndReservePerImage(t *testing.T) {
	for count := 0; count <= 2; count++ {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			in := reportCalibrationInput{Topic: "face_palm", Question: "核对合成图像", Inputs: json.RawMessage(`{"confirmed":"可见线条"}`), Draft: reportBody{Summary: "摘要原文", Content: "完整正文原文"}}
			for i := 0; i < count; i++ {
				in.ImageDataURLs = append(in.ImageDataURLs, "data:image/png;base64,"+strings.Repeat("A", 200000+i))
			}
			req, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192, ThinkingEnabled: true, ReasoningEffort: "high"}, 1048576, in)
			if err != nil {
				t.Fatal(err)
			}
			if len(req.Messages) != 1 || len(req.Messages[0].ImageDataURLs) != count {
				t.Fatalf("critic lost images: messages=%d, images=%d, want=%d", len(req.Messages), len(req.Messages[0].ImageDataURLs), count)
			}
			for i, image := range in.ImageDataURLs {
				if req.Messages[0].ImageDataURLs[i] != image {
					t.Fatal("critic changed an already loaded image")
				}
			}
			raw, _ := json.Marshal(in)
			if strings.Contains(string(raw), "base64") || strings.Contains(req.Messages[0].Content, "base64") || strings.Contains(string(raw), "ImageDataURLs") {
				t.Fatal("image bytes leaked into calibration JSON or text budget")
			}
			var received reportCalibrationInput
			if err := json.Unmarshal([]byte(req.Messages[0].Content), &received); err != nil {
				t.Fatal(err)
			}
			if received.Question != in.Question || string(received.Inputs) != string(in.Inputs) || received.Draft != in.Draft {
				t.Fatal("image transport changed the confirmed JSON or full draft")
			}
			boundary := len(req.SystemPrompt) + len(req.Messages[0].Content) + req.MaxTokens + 4096 + count*16384
			if !reportCalibrationFits(req, boundary) || reportCalibrationFits(req, boundary-1) {
				t.Fatal("strict fit must reserve exactly 16384 per image, independently of base64 size")
			}
			if _, err := reportCalibrationRequest(req, boundary, in); err != nil {
				t.Fatal("full image evidence should fit its exact boundary", err)
			}
			if _, err := reportCalibrationRequest(req, boundary-1, in); err == nil {
				t.Fatal("request construction omitted image reserve or its fixed margin")
			}
			req.Messages = append(req.Messages, provider.Message{Role: "user", Content: "仅修正审读协议，不改变图片与事实。"})
			boundary += len(req.Messages[1].Content)
			if !reportCalibrationFits(req, boundary) || reportCalibrationFits(req, boundary-1) {
				t.Fatal("protocol correction lost image reserve")
			}
		})
	}
}

func TestReportCalibrationImageRulesRespectAvailableEvidenceAndTopic(t *testing.T) {
	const shared = "不得凭单张图与未核实的常见或真实解剖分布比较；线条交叉不直接等于断裂"
	const visible = "按同一张图的可见方位核对线条与拇指方向，以从掌连接处指向指尖判断上下"
	const unseen = "图像核对只检查表述边界，不凭未见的图片纠正线条位置"
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		for _, images := range []bool{false, true} {
			if images && topic != "face_palm" {
				continue
			}
			t.Run(fmt.Sprintf("%s/images=%t", topic, images), func(t *testing.T) {
				in := reportCalibrationInput{Topic: topic, Draft: reportBody{Content: "只核对本轮图像与已确认资料。"}}
				if images {
					in.ImageDataURLs = []string{"data:image/png;base64,eA=="}
				}
				req, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192}, 65536, in)
				if err != nil {
					t.Fatal(err)
				}
				writer := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
				for _, prompt := range []string{writer, req.SystemPrompt} {
					if strings.Contains(prompt, shared) != (topic == "face_palm") {
						t.Fatal("single-image anatomy/crossing boundary leaked or disappeared")
					}
				}
				if strings.Contains(req.SystemPrompt, visible) != images || strings.Contains(req.SystemPrompt, unseen) != (topic == "face_palm" && !images) {
					t.Fatal("critic direction policy does not match whether it can see the image")
				}
				if strings.Contains(req.SystemPrompt, "单张图不能证明图外真实纹理的密度、相对深浅或光照成因") != (topic == "face_palm") {
					t.Fatal("existing unmeasured-image boundary was lost")
				}
			})
		}
	}
}

// This checks actual writer/critic wire transport with a local synthetic PNG,
// not the accuracy of a model's visual interpretation or a live report.
func TestReportCalibrationImagesReach32KCandidateAndProtocolRepair(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII="
	png, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, padding            string
		window, wantReviewTokens int
	}{
		{name: "known_32k_candidate", window: 1048576, wantReviewTokens: 32768},
		{name: "image_reserve_keeps_fitting_16k", padding: strings.Repeat("x", 10000), window: 65536, wantReviewTokens: 16384},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var imageReads atomic.Int32
			images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				imageReads.Add(1)
				_, _ = w.Write(png)
			}))
			t.Cleanup(images.Close)
			imageURL := strings.Replace(images.URL, "127.0.0.1", "localhost", 1) + "/synthetic.png"
			wantImage := "data:image/png;base64," + pngBase64
			body := reportBody{Summary: strings.Repeat("合成图像用于校验同图审读参数。", 3), Content: "## 可见依据\n" + strings.Repeat("只描述本轮可见内容，不代表真实模型质量。", 20) + tc.padding + "\n## 对照\n按画面方位核对连接与指尖。\n## 行动\n不能确认处保留限制。"}
			rawBody, _ := json.Marshal(body)
			writers, reviews := 0, 0
			p := provider.NewOpenAICompatible("http://calibration-images.invalid", "fixture", "fixture-vision", "fixture-vision")
			p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				if r.Method == http.MethodGet && r.URL.Path == "/models" {
					fmt.Fprint(w, `{"data":[{"id":"fixture-vision","context_window":1048576,"max_output_tokens":393216}]}`)
				} else {
					var req map[string]any
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						return nil, err
					}
					critic := isReportCalibrationFixtureRequest(req)
					messages := req["messages"].([]any)
					var wireImages []string
					var firstUserText string
					for _, message := range messages {
						m := message.(map[string]any)
						if m["role"] != "user" {
							continue
						}
						text, _ := m["content"].(string)
						if parts, ok := m["content"].([]any); ok {
							for _, part := range parts {
								p := part.(map[string]any)
								if p["type"] == "text" {
									text += fmt.Sprint(p["text"])
								} else if p["type"] == "image_url" {
									wireImages = append(wireImages, fmt.Sprint(p["image_url"].(map[string]any)["url"]))
								}
							}
						}
						if strings.Contains(text, pngBase64) || strings.Contains(text, "data:image/") {
							t.Error("base64 leaked into user JSON or text")
						}
						if firstUserText == "" {
							firstUserText = text
						}
					}
					if len(wireImages) != 1 || wireImages[0] != wantImage {
						t.Errorf("writer/critic/protocol correction must receive the same image exactly once, got %d", len(wireImages))
					}
					wantTokens, wantEffort := 16384, "low"
					if critic {
						wantTokens, wantEffort = tc.wantReviewTokens, "high"
					}
					thinking, _ := req["thinking"].(map[string]any)
					if req["max_tokens"] != float64(wantTokens) || req["reasoning_effort"] != wantEffort || thinking["type"] != "enabled" || req["model"] != "fixture-vision" {
						t.Errorf("image transport changed model or reasoning/output policy: tokens=%v", req["max_tokens"])
					}
					if critic {
						reviews++
						var input reportCalibrationInput
						if json.Unmarshal([]byte(firstUserText), &input) != nil || input.Topic != "face_palm" || input.Draft != body || string(input.Inputs) != `{}` {
							t.Error("multimodal critic lost exact draft or safe confirmed inputs")
						}
						result := `{"issues":[]}`
						if reviews == 1 {
							result = `{"issues":null}`
						}
						writeReportCalibrationFixture(w, result)
					} else {
						writers++
						event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(rawBody)}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
					}
				}
				response := w.Result()
				response.Request = r
				return response, nil
			})})
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			for _, stage := range []string{"checking", "calculating", "writing"} {
				mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), Provider: p, Models: provider.NewCatalog(p), Guard: agent.NewGuard(20000, nil), ImageLoader: agent.NewImageLoader([]string{"localhost"}, 1024), SkillModel: reportSkills{&model.AISkill{Code: "face_palm", Status: model.SkillStatusEnabled, InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`}}}
			s.AIConfig.ContextWindow = tc.window
			inputs, _ := json.Marshal(map[string]any{"_reportImages": []map[string]any{{"mediaId": 7, "url": imageURL, "contentType": "image/png"}}})
			got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "face_palm", Question: "核对已授权的合成图像", InputsJSON: string(inputs), ChaptersJSON: `["可见依据","对照","行动"]`})
			if err != nil || got != body || imageReads.Load() != 1 || writers != 1 || reviews != 2 || doc.ModelCalls != 3 || doc.ToolCalls != 0 || usage.PromptTokens != 25 || usage.CompletionTokens != 16 || usage.ReasoningTokens != 7 {
				t.Fatalf("changed read/call/usage bounds: err=%v bodyEqual=%t imageReads=%d writers=%d reviews=%d models=%d tools=%d usage=%+v", err, got == body, imageReads.Load(), writers, reviews, doc.ModelCalls, doc.ToolCalls, usage)
			}
			persisted, _ := json.Marshal(map[string]any{"body": got, "document": doc})
			if strings.Contains(string(persisted), pngBase64) || strings.Contains(string(persisted), "data:image/") {
				t.Fatal("image bytes leaked into persisted report JSON")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
