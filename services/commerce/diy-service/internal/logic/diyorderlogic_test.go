package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDesignOrderItemsSupportsVersionTwoDocument(t *testing.T) {
	raw := `{
		"version":2,
		"wristSizeMm":160,
		"fitAllowanceMm":5,
		"beads":[
			{"slotId":"slot-1","position":0,"materialId":2,"materialName":"星月菩提","spec":"10mm","unitPrice":1,"subtype":"main_bead","diameterMm":10}
		],
		"cord":{"materialId":14,"materialName":"弹力绳","spec":"","unitPrice":1,"quantity":1,"subtype":"cord"},
		"items":[
			{"materialId":2,"materialName":"星月菩提","spec":"10mm","unitPrice":1,"quantity":12,"subtype":"main_bead"},
			{"materialId":14,"materialName":"弹力绳","spec":"","unitPrice":1,"quantity":1,"subtype":"cord"}
		]
	}`

	items, err := parseDesignOrderItems(raw)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int64(2), items[0].MaterialId)
	require.Equal(t, 12, items[0].Quantity)
	require.Equal(t, "cord", items[1].Subtype)
}

func TestParseDesignOrderItemsKeepsEmptyVersionTwoDocumentEmpty(t *testing.T) {
	items, err := parseDesignOrderItems(`{"version":2,"beads":[],"items":[]}`)
	require.NoError(t, err)
	require.Empty(t, items)
}
