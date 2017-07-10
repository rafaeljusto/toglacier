package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rafaeljusto/toglacier"
	"github.com/rafaeljusto/toglacier/internal/archive"
	"github.com/rafaeljusto/toglacier/internal/cloud"
	"github.com/rafaeljusto/toglacier/internal/config"
	"github.com/rafaeljusto/toglacier/internal/report"
	"github.com/rafaeljusto/toglacier/internal/storage"
	"github.com/robfig/cron"
	"github.com/urfave/cli"
)

var (
	toGlacier  toglacier.ToGlacier
	logger     *logrus.Logger
	logFile    *os.File
	ctx        context.Context
	cancel     context.CancelFunc
	cancelFunc func()
)

func main() {
	defer logFile.Close()

	// ctx is used to abort long transactions, such as big files uploads or
	// inventories
	ctx = context.Background()
	ctx, cancel = context.WithCancel(ctx)

	app := cli.NewApp()
	app.Name = "toglacier"
	app.Usage = "Send data to AWS Glacier service"
	app.Version = config.Version
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
	app.Before = initialize
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
			Action: commandSync,
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
			Action:    commandGet,
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
			Action:    commandRemove,
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
			ArgsUsage: "[pattern]",
			Action:    commandList,
		},
		{
			Name:   "start",
			Usage:  "run the scheduler (will block forever)",
			Action: commandStart,
		},
		{
			Name:   "report",
			Usage:  "test report notification",
			Action: commandReport,
		},
		{
			Name:      "encrypt",
			Aliases:   []string{"enc"},
			Usage:     "encrypt a password or secret",
			ArgsUsage: "<password>",
			Action:    commandEncrypt,
		},
	}

	manageSignals(cancel, func() {
		if cancelFunc != nil {
			cancelFunc()
		}
	})

	app.Run(os.Args)
}

func initialize(c *cli.Context) error {
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
	if awsCloud, err = cloud.NewAWSCloud(report.NewLogger(logger), awsConfig, false); err != nil {
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

func commandSync(c *cli.Context) error {
	if !c.Bool("verbose") {
		logger.Out = ioutil.Discard
	}

	var ignorePatterns []*regexp.Regexp
	for _, pattern := range config.Current().IgnorePatterns {
		ignorePatterns = append(ignorePatterns, pattern.Value)
	}

	err := toGlacier.Backup(
		config.Current().Paths,
		config.Current().BackupSecret.Value,
		float64(config.Current().ModifyTolerance),
		ignorePatterns,
	)

	if err != nil {
		logger.Error(err)
	}

	return nil
}

func commandGet(c *cli.Context) error {
	if !c.Bool("verbose") {
		logger.Out = ioutil.Discard
	}

	if err := toGlacier.RetrieveBackup(c.Args().First(), config.Current().BackupSecret.Value, c.Bool("skip-unmodified")); err != nil {
		logger.Error(err)
	} else {
		fmt.Println("Backup recovered successfully")
	}

	return nil
}

func commandRemove(c *cli.Context) error {
	if !c.Bool("verbose") {
		logger.Out = ioutil.Discard
	}

	ids := []string{c.Args().First()}
	ids = append(ids, c.Args().Tail()...)
	if err := toGlacier.RemoveBackups(ids...); err != nil {
		logger.Error(err)
	}

	return nil
}

func commandList(c *cli.Context) error {
	if !c.Bool("verbose") {
		logger.Out = ioutil.Discard
	}

	backups, err := toGlacier.ListBackups(c.Bool("remote"))
	if err != nil {
		logger.Error(err)

	} else if len(backups) == 0 {
		return nil
	}

	var filenameMatch *regexp.Regexp
	if c.NArg() > 0 {
		fmt.Printf("Backups containing pattern “%s”\n\n", c.Args().First())

		if filenameMatch, err = regexp.Compile(c.Args().First()); err != nil {
			logger.Errorf("invalid pattern. details: %s", err)
		}
	}

	fmt.Println("Date             | Vault Name       | Archive ID")
	fmt.Printf("%s-+-%s-+-%s\n", strings.Repeat("-", 16), strings.Repeat("-", 16), strings.Repeat("-", 138))

	for _, backup := range backups {
		show := false
		if c.NArg() > 0 {
			for filename, itemInfo := range backup.Info {
				if itemInfo.Status.Useful() && (filenameMatch != nil && filenameMatch.MatchString(filename)) {
					show = true
				}
			}
		}

		if show || c.NArg() == 0 {
			fmt.Printf("%-16s | %-16s | %-138s\n", backup.Backup.CreatedAt.Format("2006-01-02 15:04"), backup.Backup.VaultName, backup.Backup.ID)
		}
	}

	return nil
}

func commandStart(c *cli.Context) error {
	var ignorePatterns []*regexp.Regexp
	for _, pattern := range config.Current().IgnorePatterns {
		ignorePatterns = append(ignorePatterns, pattern.Value)
	}

	scheduler := cron.New()

	scheduler.Schedule(config.Current().Scheduler.Backup.Value, jobFunc(func() {
		err := toGlacier.Backup(
			config.Current().Paths,
			config.Current().BackupSecret.Value,
			float64(config.Current().ModifyTolerance),
			ignorePatterns,
		)

		if err != nil {
			logger.Error(err)
		}
	}))

	scheduler.Schedule(config.Current().Scheduler.RemoveOldBackups.Value, jobFunc(func() {
		if err := toGlacier.RemoveOldBackups(config.Current().KeepBackups); err != nil {
			logger.Error(err)
		}
	}))

	scheduler.Schedule(config.Current().Scheduler.ListRemoteBackups.Value, jobFunc(func() {
		if _, err := toGlacier.ListBackups(true); err != nil {
			logger.Error(err)
		}
	}))

	scheduler.Schedule(config.Current().Scheduler.SendReport.Value, jobFunc(func() {
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
	}))

	scheduler.Start()

	stopped := make(chan bool)
	cancelFunc = func() {
		scheduler.Stop()
		stopped <- true
	}

	select {
	case <-stopped:
		// wait a small period just to give time for the scheduler to shutdown
		time.Sleep(time.Second)
	}

	return nil
}

func commandReport(c *cli.Context) error {
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
}

func commandEncrypt(c *cli.Context) error {
	if pwd, err := config.PasswordEncrypt(c.Args().First()); err != nil {
		logger.Error(err)
	} else {
		fmt.Printf("encrypted:%s\n", pwd)
	}

	return nil
}

// jobFunc is used only to implement inline functions in the scheduler.
type jobFunc func()

func (j jobFunc) Run() {
	j()
}
