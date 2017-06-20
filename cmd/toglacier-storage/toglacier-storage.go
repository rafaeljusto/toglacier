package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/urfave/cli"
)

var from, to formatOptions
var logger = logrus.New()

func main() {
	app := cli.NewApp()
	app.Name = "toglacier-storage"
	app.Usage = "Manage local storage from toglacier tool"
	app.Version = config.Version
	app.Authors = []cli.Author{
		{
			Name:  "Rafael Dantas Justo",
			Email: "adm@rafael.net.br",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "convert",
			Usage: "migrate the storage to a new format",
			Flags: []cli.Flag{
				cli.GenericFlag{
					Name:  "from,f",
					Usage: "current local storage format",
					Value: &from,
				},
				cli.GenericFlag{
					Name:  "to,t",
					Usage: "desired local storage format",
					Value: &to,
				},
				cli.StringFlag{
					Name:  "output,o",
					Usage: "new storage format file to be created",
				},
			},
			ArgsUsage: "<db-file>",
			Action:    commandConvert,
		},
	}

	app.Run(os.Args)
}

func commandConvert(c *cli.Context) error {
	if from == to {
		fmt.Println("converting to the same format")
		return nil
	}

	if !c.Args().Present() {
		fmt.Println("input file not informed")
		return nil
	}

	output := c.String("output")
	if output == "" {
		fmt.Println("output file not informed")
		return nil
	}

	var fromStorage, toStorage storage.Storage

	switch from.value {
	case formatBoltDB:
		fromStorage = storage.NewBoltDB(logger, c.Args().First())
	case formatAuditFile:
		fromStorage = storage.NewAuditFile(logger, c.Args().First())
	default:
		fmt.Printf("unknown “from” storage “%s”\n", from.value)
	}

	backups, err := fromStorage.List()
	if err != nil {
		fmt.Printf("error reading backups. details: %s", err)
		return nil
	}

	switch to.value {
	case formatBoltDB:
		toStorage = storage.NewBoltDB(logger, output)
	case formatAuditFile:
		toStorage = storage.NewAuditFile(logger, output)
	default:
		fmt.Printf("unknown “to” storage “%s”\n", to.value)
	}

	if len(backups) == 0 {
		fmt.Println("no backups to save")
		return nil
	}

	for _, backup := range backups {
		if err := toStorage.Save(backup); err != nil {
			fmt.Printf("error saving backup “%s”. details: %s", backup.Backup.ID, err)
			return nil
		}
	}

	return nil
}

const (
	formatBoltDB    format = "boltdb"
	formatAuditFile format = "audit"
)

type format string

var possibleFormats = map[string]format{
	string(formatBoltDB):    formatBoltDB,
	string(formatAuditFile): formatAuditFile,
}

type formatOptions struct {
	value format
}

func (f *formatOptions) Set(value string) error {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)

	var ok bool
	if f.value, ok = possibleFormats[value]; !ok {
		return fmt.Errorf("possible values are %s or %s", formatBoltDB, formatAuditFile)
	}
	return nil
}

func (f formatOptions) String() string {
	return string(f.value)
}
