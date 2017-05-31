package storage_test

import (
	"reflect"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestBackups_Search(t *testing.T) {
	scenarios := []struct {
		description   string
		id            string
		backups       storage.Backups
		expected      storage.Backup
		expectedFound bool
	}{
		{
			description: "it should sort and find an id in backups",
			id:          "1234",
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test3",
					},
				},
			},
			expected: storage.Backup{
				Backup: cloud.Backup{
					ID:        "1234",
					VaultName: "test2",
				},
			},
			expectedFound: true,
		},
		{
			description: "it should find an id in backups without sorting",
			id:          "1234",
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test3",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test1",
					},
				},
			},
			expected: storage.Backup{
				Backup: cloud.Backup{
					ID:        "1234",
					VaultName: "test2",
				},
			},
			expectedFound: true,
		},
		{
			description: "it should sort and not find an id in backups",
			id:          "1232",
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test3",
					},
				},
			},
			expectedFound: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			backup, ok := scenario.backups.Search(scenario.id)

			if !reflect.DeepEqual(scenario.expected, backup) {
				t.Errorf("unexpected backup.\n%v", Diff(scenario.expected, backup))
			}

			if scenario.expectedFound != ok {
				t.Errorf("unexpected found flag, expected %t and got %t", scenario.expectedFound, ok)
			}
		})
	}
}
