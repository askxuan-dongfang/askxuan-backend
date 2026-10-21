package common

import "strings"

// Arguments are source-code constants, never request values. Keep per-provider tags
// in the SELECT projection so pagination totals cannot be inflated by multiple tags.
func DirectoryTagColumns(kind, codeColumn string) string {
	// Existing assignments remain visible even when the catalog disables or no
	// longer resolves a historical code. Choice lists independently hide disabled tags.
	return "COALESCE((SELECT GROUP_CONCAT(pd.tag_code ORDER BY COALESCE(dt.sort,9999),pd.tag_code SEPARATOR ',') FROM askxuan_temple.provider_directory_tag pd LEFT JOIN askxuan_temple.directory_tag dt ON dt.code=pd.tag_code AND dt.tag_group='tradition' WHERE pd.provider_kind='" + kind + "' AND pd.provider_code=" + codeColumn + "),'') AS tradition_codes"
}
func SplitDirectoryTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	return strings.Split(raw, ",")
}
