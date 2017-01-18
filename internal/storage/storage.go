package storage

import "github.com/rafaeljusto/toglacier/internal/cloud"

type Storage interface {
	Save(cloud.Backup) error
	List() ([]cloud.Backup, error)
	Remove(id string) error
}
