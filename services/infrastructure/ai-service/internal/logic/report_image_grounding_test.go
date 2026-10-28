package logic

import (
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
)

func TestImageReportReviewSeparatesVisibleAppearanceFromUnmeasuredCauses(t *testing.T) {
	const claim = "这种平光会削弱纹线深浅的层次，也会让细纹显得比实际更密。"
	for _, topic := range []string{"face_palm", "dream"} {
		request, err := reportCalibrationRequest(provider.Request{Model: "fixture", MaxTokens: 8192, ThinkingEnabled: true, ReasoningEffort: "high"}, 32768, reportCalibrationInput{Topic: topic, Draft: reportBody{Content: claim}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(request.Messages[0].Content, claim) || request.ReasoningEffort != "high" || request.MaxTokens != 8192 {
			t.Fatal("review changed the draft or reasoning policy")
		}
		imageRule := "单张图不能证明图外真实纹理的密度、相对深浅或光照成因"
		if strings.Contains(request.SystemPrompt, imageRule) != (topic == "face_palm") {
			t.Fatal("image evidence boundary must apply only to image reports")
		}
	}
	instruction := reportInstruction(&Report{SkillCode: "face_palm"}, &model.AISkill{})
	if !strings.Contains(instruction, "不能把画面纹理与不可见的真实纹理作比较") {
		t.Fatal("image writer still treats unmeasured appearance comparisons as facts")
	}
}
