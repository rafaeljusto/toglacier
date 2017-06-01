package storage_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/storage"
)

func TestBackups_sort(t *testing.T) {
	scenarios := []struct {
		description string
		backups     storage.Backups
		expected    storage.Backups
	}{
		{
			description: "it should sort correctly",
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			sort.Sort(scenario.backups)

			if !reflect.DeepEqual(scenario.expected, scenario.backups) {
				t.Errorf("unexpected backups.\n%v", Diff(scenario.expected, scenario.backups))
			}
		})
	}
}

func TestBackups_Add(t *testing.T) {
	scenarios := []struct {
		description string
		backup      storage.Backup
		backups     storage.Backups
		expected    storage.Backups
	}{
		{
			description: "it should add in an empty slice",
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "1234",
					VaultName: "test1",
				},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
			},
		},
		{
			description: "it should add in the correct position",
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "1235",
					VaultName: "test2",
				},
			},
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
			},
		},
		{
			description: "it should replace when the same id is found",
			backup: storage.Backup{
				Backup: cloud.Backup{
					ID:        "1235",
					VaultName: "test2",
				},
			},
			backups: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
			},
			expected: storage.Backups{
				{
					Backup: cloud.Backup{
						ID:        "1234",
						VaultName: "test1",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1235",
						VaultName: "test2",
					},
				},
				{
					Backup: cloud.Backup{
						ID:        "1236",
						VaultName: "test3",
					},
				},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			scenario.backups.Add(scenario.backup)

			if !reflect.DeepEqual(scenario.expected, scenario.backups) {
				t.Errorf("unexpected backups.\n%v", Diff(scenario.expected, scenario.backups))
			}
		})
	}
}

func TestBackups_Search(t *testing.T) {
	scenarios := []struct {
		description   string
		id            string
		backups       storage.Backups
		expected      storage.Backup
		expectedFound bool
	}{
		{
			description: "it should find an id in backups",
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
			description: "it should not find an id in backups",
			id:          "1232",
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
