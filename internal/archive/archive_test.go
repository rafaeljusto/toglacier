package archive_test

import (
	"reflect"
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

func TestInfo_FilterByStatuses(t *testing.T) {
	scenarios := []struct {
		description string
		statuses    []archive.ItemInfoStatus
		info        archive.Info
		expected    archive.Info
	}{
		{
			description: "it should filter correctly the archive information",
			statuses:    []archive.ItemInfoStatus{archive.ItemInfoStatusModified},
			info: archive.Info{
				"file1": archive.ItemInfo{
					ID:     "12345",
					Status: archive.ItemInfoStatusNew,
				},
				"file2": archive.ItemInfo{
					ID:     "12346",
					Status: archive.ItemInfoStatusDeleted,
				},
				"file3": archive.ItemInfo{
					ID:     "12347",
					Status: archive.ItemInfoStatusModified,
				},
			},
			expected: archive.Info{
				"file3": archive.ItemInfo{
					ID:     "12347",
					Status: archive.ItemInfoStatusModified,
				},
			},
		},
		{
			description: "it should filter correctly when there are no statuses",
			info: archive.Info{
				"file1": archive.ItemInfo{
					ID:     "12345",
					Status: archive.ItemInfoStatusNew,
				},
				"file2": archive.ItemInfo{
					ID:     "12346",
					Status: archive.ItemInfoStatusDeleted,
				},
				"file3": archive.ItemInfo{
					ID:     "12347",
					Status: archive.ItemInfoStatusModified,
				},
			},
			expected: make(archive.Info),
		},
		{
			description: "it should filter correctly when the status is not found",
			statuses:    []archive.ItemInfoStatus{archive.ItemInfoStatusUnmodified},
			info: archive.Info{
				"file1": archive.ItemInfo{
					ID:     "12345",
					Status: archive.ItemInfoStatusNew,
				},
				"file2": archive.ItemInfo{
					ID:     "12346",
					Status: archive.ItemInfoStatusDeleted,
				},
				"file3": archive.ItemInfo{
					ID:     "12347",
					Status: archive.ItemInfoStatusModified,
				},
			},
			expected: make(archive.Info),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			archiveInfo := scenario.info.FilterByStatuses(scenario.statuses...)
			if !reflect.DeepEqual(scenario.expected, archiveInfo) {
				t.Errorf("unexpected result.\n%v", Diff(scenario.expected, archiveInfo))
			}
		})
	}
}
