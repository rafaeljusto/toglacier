package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jasonlvhit/gocron"
	"github.com/rafaeljusto/toglacier"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/urfave/cli"
)

func main() {
	var toGlacier toglacier.ToGlacier
	var logger *logrus.Logger
	var logFile *os.File
	defer logFile.Close()

	// ctx is used to abort long transactions, such as big files uploads or
	// inventories
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	var cancelFunc func()

	app := cli.NewApp()
	app.Name = "toglacier"
	app.Usage = "Send data to AWS Glacier service"
	app.Version = "2.0.0"
	app.Authors = []cli.Author{
		{
			Name:  "Rafael Dantas Justo",
			Email: "adm@rafael.net.br",
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "Tool configuration file (YAML)",
		},
	}
	app.Before = func(c *cli.Context) error {
		config.Default()

		var err error

		if c.String("config") != "" {
			if err = config.LoadFromFile(c.String("config")); err != nil {
				fmt.Printf("error loading configuration file. details: %s\n", err)
				return err
			}
		}

		if err = config.LoadFromEnvironment(); err != nil {
			fmt.Printf("error loading configuration from environment variables. details: %s\n", err)
			return err
		}

		logger = logrus.New()
		logger.Out = os.Stdout

		// optionally set logger output file defined in configuration. if not
		// defined stdout will be used
		if config.Current().Log.File != "" {
			if logFile, err = os.OpenFile(config.Current().Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm); err != nil {
				fmt.Printf("error opening log file “%s”. details: %s\n", config.Current().Log.File, err)
				return err
			}

			// writes to the stdout and to the log file
			logger.Out = io.MultiWriter(os.Stdout, logFile)
		}

		switch config.Current().Log.Level {
		case config.LogLevelDebug:
			logger.Level = logrus.DebugLevel
		case config.LogLevelInfo:
			logger.Level = logrus.InfoLevel
		case config.LogLevelWarning:
			logger.Level = logrus.WarnLevel
		case config.LogLevelError:
			logger.Level = logrus.ErrorLevel
		case config.LogLevelFatal:
			logger.Level = logrus.FatalLevel
		case config.LogLevelPanic:
			logger.Level = logrus.PanicLevel
		}

		awsConfig := cloud.AWSConfig{
			AccountID:       config.Current().AWS.AccountID.Value,
			AccessKeyID:     config.Current().AWS.AccessKeyID.Value,
			SecretAccessKey: config.Current().AWS.SecretAccessKey.Value,
			Region:          config.Current().AWS.Region,
			VaultName:       config.Current().AWS.VaultName,
		}

		var awsCloud cloud.Cloud
		if awsCloud, err = cloud.NewAWSCloud(logger, awsConfig, false); err != nil {
			fmt.Printf("error initializing AWS cloud. details: %s\n", err)
			return err
		}

		var localStorage storage.Storage
		switch config.Current().Database.Type {
		case config.DatabaseTypeAuditFile:
			localStorage = storage.NewAuditFile(logger, config.Current().Database.File)
		case config.DatabaseTypeBoltDB:
			localStorage = storage.NewBoltDB(logger, config.Current().Database.File)
		}

		toGlacier = toglacier.ToGlacier{
			Context: ctx,
			Archive: archive.NewTARBuilder(logger),
			Envelop: archive.NewOFBEnvelop(logger),
			Cloud:   awsCloud,
			Storage: localStorage,
			Logger:  logger,
		}

		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "sync",
			Usage: "backup now the desired paths to AWS Glacier",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose,v",
					Usage: "show what is happening behind the scenes",
				},
			},
			Action: func(c *cli.Context) error {
				if !c.Bool("verbose") {
					logger.Out = ioutil.Discard
				}

				if err := toGlacier.Backup(config.Current().Paths, config.Current().BackupSecret.Value); err != nil {
					logger.Error(err)
				}
				return nil
			},
		},
		{
			Name:  "get",
			Usage: "retrieve a specific backup from AWS Glacier",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "skip-unmodified,s",
					Usage: "ignore files unmodified in disk since the backup",
				},
				cli.BoolFlag{
					Name:  "verbose,v",
					Usage: "show what is happening behind the scenes",
				},
			},
			ArgsUsage: "<archiveID>",
			Action: func(c *cli.Context) error {
				if !c.Bool("verbose") {
					logger.Out = ioutil.Discard
				}

				if err := toGlacier.RetrieveBackup(c.Args().First(), config.Current().BackupSecret.Value, c.Bool("skip-unmodified")); err != nil {
					logger.Error(err)
				} else {
					fmt.Println("Backup recovered successfully")
				}
				return nil
			},
		},
		{
			Name:    "remove",
			Aliases: []string{"rm"},
			Usage:   "remove backups from AWS Glacier",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose,v",
					Usage: "show what is happening behind the scenes",
				},
			},
			ArgsUsage: "<archiveID> [archiveID ...]",
			Action: func(c *cli.Context) error {
				if !c.Bool("verbose") {
					logger.Out = ioutil.Discard
				}

				ids := []string{c.Args().First()}
				ids = append(ids, c.Args().Tail()...)
				if err := toGlacier.RemoveBackups(ids...); err != nil {
					logger.Error(err)
				}
				return nil
			},
		},
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list all backups sent to AWS Glacier or that contains a specific file",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "remote,r",
					Usage: "retrieve the list from AWS Glacier (long wait)",
				},
				cli.BoolFlag{
					Name:  "verbose,v",
					Usage: "show what is happening behind the scenes",
				},
			},
			ArgsUsage: "[file]",
			Action: func(c *cli.Context) error {
				if !c.Bool("verbose") {
					logger.Out = ioutil.Discard
				}

				backups, err := toGlacier.ListBackups(c.Bool("remote"))
				if err != nil {
					logger.Error(err)

				} else if len(backups) == 0 {
					return nil
				}

				if c.NArg() > 0 {
					fmt.Printf("Backups containing filename %s\n\n", c.Args().First())
				}

				fmt.Println("Date             | Vault Name       | Archive ID")
				fmt.Printf("%s-+-%s-+-%s\n", strings.Repeat("-", 16), strings.Repeat("-", 16), strings.Repeat("-", 138))

				for _, backup := range backups {
					show := false
					if c.NArg() > 0 {
						for filename, itemInfo := range backup.Info {
							if itemInfo.Status.Useful() && strings.HasSuffix(filename, c.Args().First()) {
								show = true
							}
						}
					}

					if show || c.NArg() == 0 {
						fmt.Printf("%-16s | %-16s | %-138s\n", backup.Backup.CreatedAt.Format("2006-01-02 15:04"), backup.Backup.VaultName, backup.Backup.ID)
					}
				}
				return nil
			},
		},
		{
			Name:  "start",
			Usage: "run the scheduler (will block forever)",
			Action: func(c *cli.Context) error {
				scheduler := gocron.NewScheduler()
				scheduler.Every(1).Day().At("00:00").Do(func() {
					if err := toGlacier.Backup(config.Current().Paths, config.Current().BackupSecret.Value); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("01:00").Do(func() {
					if err := toGlacier.RemoveOldBackups(config.Current().KeepBackups); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(4).Weeks().At("12:00").Do(func() {
					if _, err := toGlacier.ListBackups(true); err != nil {
						logger.Error(err)
					}
				})
				scheduler.Every(1).Weeks().At("06:00").Do(func() {
					emailInfo := toglacier.EmailInfo{
						Sender:   toglacier.EmailSenderFunc(smtp.SendMail),
						Server:   config.Current().Email.Server,
						Port:     config.Current().Email.Port,
						Username: config.Current().Email.Username,
						Password: config.Current().Email.Password.Value,
						From:     config.Current().Email.From,
						To:       config.Current().Email.To,
						Format:   report.Format(config.Current().Email.Format),
					}

					if err := toGlacier.SendReport(emailInfo); err != nil {
						logger.Error(err)
					}
				})

				stopped := scheduler.Start()
				cancelFunc = func() {
					close(stopped)
				}

				select {
				case <-stopped:
					// wait a small period just to give time for the scheduler to shutdown
					time.Sleep(time.Second)
				}

				return nil
			},
		},
		{
			Name:  "report",
			Usage: "test report notification",
			Action: func(c *cli.Context) error {
				test := report.NewTest()
				test.Errors = append(test.Errors, errors.New("simulated error 1"))
				test.Errors = append(test.Errors, errors.New("simulated error 2"))
				test.Errors = append(test.Errors, errors.New("simulated error 3"))

				report.Add(test)

				emailInfo := toglacier.EmailInfo{
					Sender:   toglacier.EmailSenderFunc(smtp.SendMail),
					Server:   config.Current().Email.Server,
					Port:     config.Current().Email.Port,
					Username: config.Current().Email.Username,
					Password: config.Current().Email.Password.Value,
					From:     config.Current().Email.From,
					To:       config.Current().Email.To,
					Format:   report.Format(config.Current().Email.Format),
				}

				if err := toGlacier.SendReport(emailInfo); err != nil {
					logger.Error(err)
				}

				return nil
			},
		},
		{
			Name:      "encrypt",
			Aliases:   []string{"enc"},
			Usage:     "encrypt a password or secret",
			ArgsUsage: "<password>",
			Action: func(c *cli.Context) error {
				if pwd, err := config.PasswordEncrypt(c.Args().First()); err != nil {
					logger.Error(err)
				} else {
					fmt.Printf("encrypted:%s\n", pwd)
				}
				return nil
			},
		},
	}

	manageSignals(cancel, cancelFunc)
	app.Run(os.Args)
}
