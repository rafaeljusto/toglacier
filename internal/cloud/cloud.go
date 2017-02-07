package cloud

// Cloud offers all necessary operations to manage backups in the cloud.
type Cloud interface {
	// Send uploads the file to the cloud and return the backup archive
	// information.
	Send(filename string) (Backup, error)

	// List retrieves all the uploaded backups information in the cloud.
	List() ([]Backup, error)

	// Get retrieves a specific backup file and stores it locally in a file. The
	// filename storing the location of the file is returned.
	Get(id string) (filename string, err error)

	// Remove erase a specific backup from the cloud.
	Remove(id string) error
}
