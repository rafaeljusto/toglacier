package archive_test

import (
	"testing"

	"github.com/rafaeljusto/toglacier/internal/archive"
)

func TestItemInfoStatus_Useful(t *testing.T) {
	scenarios := []struct {
		description    string
		itemInfoStatus archive.ItemInfoStatus
		expected       bool
	}{
		{description: "it should consider new as useful", itemInfoStatus: archive.ItemInfoStatusNew, expected: true},
		{description: "it should consider modified as useful", itemInfoStatus: archive.ItemInfoStatusModified, expected: true},
		{description: "it should not consider unmodified as useful", itemInfoStatus: archive.ItemInfoStatusUnmodified, expected: false},
		{description: "it should not consider deleted as useful", itemInfoStatus: archive.ItemInfoStatusDeleted, expected: false},
		{description: "it should not consider unknown as useful", itemInfoStatus: archive.ItemInfoStatus("unknown"), expected: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			useful := scenario.itemInfoStatus.Useful()
			if useful != scenario.expected {
				t.Errorf("unexpected result for status “%s”", scenario.itemInfoStatus)
			}
		})
	}
}
