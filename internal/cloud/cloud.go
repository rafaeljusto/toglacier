package cloud

type Cloud interface {
	Send(filename string) (Backup, error)
	List() ([]Backup, error)
	Get(id string) (filename string, err error)
	Remove(id string) error
}
