package logic

import (
	"github.com/askxuan/common"
	"net/url"
	"strings"
	"unicode/utf8"
)

func validateExperienceSource(experience bool, name, link, note string) error {
	if utf8.RuneCountInString(name) > 100 || len(link) > 1000 || utf8.RuneCountInString(note) > 500 {
		return common.ErrParam
	}
	if experience && (strings.TrimSpace(name) == "" || strings.TrimSpace(link) == "") {
		return common.NewBizError(common.ErrParam.Code, "体验商品需要填写案例来源和原商品链接")
	}
	if link != "" {
		u, err := url.Parse(link)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return common.NewBizError(common.ErrParam.Code, "来源链接需为有效 HTTPS 地址")
		}
	}
	return nil
}
