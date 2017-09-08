package config

import (
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/robfig/cron"
	"gopkg.in/yaml.v2"
)

// prefix is used as a namespace identifier for environment variables.
const prefix = "toglacier"

var config unsafe.Pointer

// Config stores all the necessary information to send backups to the cloud and
// keep track in the local storage.
type Config struct {
	Paths           []string   `yaml:"paths"`
	KeepBackups     int        `yaml:"keep backups" split_words:"true"`
	BackupSecret    aesKey     `yaml:"backup secret" split_words:"true"`
	ModifyTolerance Percentage `yaml:"modify tolerance" split_words:"true"`
	IgnorePatterns  []Pattern  `yaml:"ignore patterns" split_words:"true"`
	Cloud           CloudType  `yaml:"cloud"`

	Scheduler struct {
		Backup            Scheduler `yaml:"backup"`
		RemoveOldBackups  Scheduler `yaml:"remove old backups" split_words:"true"`
		ListRemoteBackups Scheduler `yaml:"list remote backups" split_words:"true"`
		SendReport        Scheduler `yaml:"send report" split_words:"true"`
	} `yaml:"scheduler" envconfig:"scheduler"`

	Database struct {
		Type DatabaseType `yaml:"type"`
		File string       `yaml:"file"`
	} `yaml:"database" envconfig:"db"`

	Log struct {
		File  string   `yaml:"file"`
		Level LogLevel `yaml:"level"`
	} `yaml:"log" envconfig:"log"`

	Email struct {
		Server   string      `yaml:"server"`
		Port     int         `yaml:"port"`
		Username string      `yaml:"username"`
		Password encrypted   `yaml:"password"`
		From     string      `yaml:"from"`
		To       []string    `yaml:"to"`
		Format   EmailFormat `yaml:"format"`
	} `yaml:"email" envconfig:"email"`

	AWS struct {
		AccountID       encrypted `yaml:"account id" split_words:"true"`
		AccessKeyID     encrypted `yaml:"access key id" split_words:"true"`
		SecretAccessKey encrypted `yaml:"secret access key" split_words:"true"`
		Region          string    `yaml:"region"`
		VaultName       string    `yaml:"vault name" split_words:"true"`
	} `yaml:"aws" envconfig:"aws"`

	GCS struct {
		Project     string `yaml:"project"`
		Bucket      string `yaml:"bucket"`
		AccountFile string `yaml:"account file" split_words:"true"`
	} `yaml:"gcs" envconfig:"gcs"`
}

// Current return the actual system configuration, stored internally in a global
// variable.
func Current() *Config {
	return (*Config)(atomic.LoadPointer(&config))
}

// Update modify the current system configuration.
func Update(c *Config) {
	atomic.StorePointer(&config, unsafe.Pointer(c))
}

// Default defines all default configuration values.
func Default() {
	c := Current()
	if c == nil {
		c = new(Config)
	}

	c.KeepBackups = 10
	c.Cloud = CloudTypeAWS
	c.Scheduler.Backup.Value, _ = cron.Parse("0 0 0 * * *")             // everyday at 00:00:00
	c.Scheduler.RemoveOldBackups.Value, _ = cron.Parse("0 0 1 * * FRI") // every friday at 01:00:00
	c.Scheduler.ListRemoteBackups.Value, _ = cron.Parse("0 0 12 1 * *") // every first day of the month at 12:00:00
	c.Scheduler.SendReport.Value, _ = cron.Parse("0 0 6 * * FRI")       // every friday at 06:00:00
	c.Database.Type = DatabaseTypeBoltDB
	c.Database.File = path.Join("var", "log", "toglacier", "toglacier.db")
	c.Log.Level = LogLevelError
	c.Email.Format = EmailFormatHTML

	Update(c)
}

// LoadFromFile parse an YAML file and fill the system configuration parameters.
// On error it will return an Error type encapsulated in a traceable error. To
// retrieve the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func LoadFromFile(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.WithStack(newError(filename, ErrorCodeReadingFile, err))
	}

	c := Current()
	if c == nil {
		c = new(Config)
	}

	if err = yaml.Unmarshal(content, c); err != nil {
		return errors.WithStack(newError(filename, ErrorCodeParsingYAML, err))
	}

	Update(c)
	return nil
}

// LoadFromEnvironment analysis all project environment variables. On error it
// will return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func LoadFromEnvironment() error {
	c := Current()
	if c == nil {
		c = new(Config)
	}

	if err := envconfig.Process(prefix, c); err != nil {
		return errors.WithStack(newError("", ErrorCodeReadingEnvVars, err))
	}

	Update(c)
	return nil
}

const (
	// CloudTypeAWS will backup archives to Amazon AWS Glacier cloud service.
	CloudTypeAWS CloudType = "aws"

	// CloudTypeGCS will backup archives to Google Cloud Storage service.
	CloudTypeGCS CloudType = "gcs"
)

var cloudTypeValid = map[string]bool{
	string(CloudTypeAWS): true,
	string(CloudTypeGCS): true,
}

// CloudType defines the cloud service type that will be used to manage
// archives.
type CloudType string

// UnmarshalText ensure that the cloud type defined in the configuration is
// valid.
func (c *CloudType) UnmarshalText(value []byte) error {
	cloudType := string(value)
	cloudType = strings.TrimSpace(cloudType)
	cloudType = strings.ToLower(cloudType)

	if ok := cloudTypeValid[cloudType]; !ok {
		return newError("", ErrorCodeCloudType, nil)
	}

	*c = CloudType(cloudType)
	return nil
}

const (
	// DatabaseTypeAuditFile use a human readable file, that stores one backup
	// information per line. As the structure is simple, this database format will
	// have less features than other types.
	DatabaseTypeAuditFile DatabaseType = "audit-file"

	// DatabaseTypeBoltDB use a fast key/value storage that stores all binary
	// content in only one file. For more information please check
	// https://github.com/boltdb/bolt
	DatabaseTypeBoltDB DatabaseType = "boltdb"
)

var databaseTypeValid = map[string]bool{
	string(DatabaseTypeAuditFile): true,
	string(DatabaseTypeBoltDB):    true,
}

// DatabaseType determinate what type of strategy will be used to store the
// local backups information.
type DatabaseType string

// UnmarshalText ensure that the database type defined in the configuration is
// valid.
func (d *DatabaseType) UnmarshalText(value []byte) error {
	databaseType := string(value)
	databaseType = strings.TrimSpace(databaseType)
	databaseType = strings.ToLower(databaseType)

	if ok := databaseTypeValid[databaseType]; !ok {
		return newError("", ErrorCodeDatabaseType, nil)
	}

	*d = DatabaseType(databaseType)
	return nil
}

const (
	// LogLevelDebug usually only enabled when debugging. Very verbose logging.
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo general operational entries about what's going on inside the
	// application.
	LogLevelInfo LogLevel = "info"

	// LogLevelWarning non-critical entries that deserve eyes.
	LogLevelWarning LogLevel = "warning"

	// LogLevelError used for errors that should definitely be noted.
	LogLevelError LogLevel = "error"

	// LogLevelFatal it will terminates the process after the the entry is logged.
	LogLevelFatal LogLevel = "fatal"

	// LogLevelPanic highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	LogLevelPanic LogLevel = "panic"
)

var logLevelValid = map[string]bool{
	string(LogLevelDebug):   true,
	string(LogLevelInfo):    true,
	string(LogLevelWarning): true,
	string(LogLevelError):   true,
	string(LogLevelFatal):   true,
	string(LogLevelPanic):   true,
}

// LogLevel determinate the verbosity of the log entries.
type LogLevel string

// UnmarshalText ensure that the log level defined in the configuration is
// valid.
func (l *LogLevel) UnmarshalText(value []byte) error {
	logLevel := string(value)
	logLevel = strings.TrimSpace(logLevel)
	logLevel = strings.ToLower(logLevel)

	if ok := logLevelValid[logLevel]; !ok {
		return newError("", ErrorCodeLogLevel, nil)
	}

	*l = LogLevel(logLevel)
	return nil
}

type encrypted struct {
	Value string
}

// UnmarshalText automatically decrypts a value from the configuration. On error
// it will return an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (e *encrypted) UnmarshalText(value []byte) error {
	e.Value = string(value)

	if strings.HasPrefix(e.Value, "encrypted:") {
		var err error
		if e.Value, err = passwordDecrypt(strings.TrimPrefix(e.Value, "encrypted:")); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

type aesKey struct {
	encrypted
}

// UnmarshalText automatically decrypts a value from the configuration. On error
// it will return an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (a *aesKey) UnmarshalText(value []byte) error {
	if err := a.encrypted.UnmarshalText(value); err != nil {
		return errors.WithStack(err)
	}

	// The key argument should be the AES key, either 16, 24, or 32 bytes to
	// select AES-128, AES-192, or AES-256. We will always force AES-256.
	if a.Value != "" {
		if len(a.Value) < 32 {
			a.Value = a.Value + strings.Repeat("0", 32-len(a.Value))
		} else if len(a.Value) > 32 {
			a.Value = a.Value[:32]
		}
	}

	return nil
}

const (
	// EmailFormatPlain ascii only content for e-mail clients that accept only
	// simple text.
	EmailFormatPlain EmailFormat = "plain"

	// EmailFormatHTML better structured content that requires HTML support by the
	// e-mail client.
	EmailFormatHTML EmailFormat = "html"
)

var emailFormatValid = map[string]bool{
	string(EmailFormatPlain): true,
	string(EmailFormatHTML):  true,
}

// EmailFormat defines the desired content format to be used in report e-mails.
// By default "html" is used.
type EmailFormat string

// UnmarshalText ensure that the email format defined in the configuration is
// valid.
func (e *EmailFormat) UnmarshalText(value []byte) error {
	emailFormat := string(value)
	emailFormat = strings.TrimSpace(emailFormat)
	emailFormat = strings.ToLower(emailFormat)

	if ok := emailFormatValid[emailFormat]; !ok {
		return newError("", ErrorCodeEmailFormat, nil)
	}

	*e = EmailFormat(emailFormat)
	return nil
}

// Percentage stores a valid percentage value.
type Percentage float64

// UnmarshalText verifies if a percentage is a valid number.
func (p *Percentage) UnmarshalText(value []byte) error {
	percentage := string(value)
	percentage = strings.TrimSpace(percentage)
	percentage = strings.ToLower(percentage)
	percentage = strings.TrimSuffix(percentage, "%")

	number, err := strconv.ParseFloat(percentage, 64)
	if err != nil {
		return newError("", ErrorCodePercentageFormat, err)
	}

	if number < 0 || number > 100 {
		return newError("", ErrorCodePercentageRange, err)
	}

	*p = Percentage(number)
	return nil
}

// Pattern stores a valid regular expression.
type Pattern struct {
	Value *regexp.Regexp
}

// UnmarshalText compile the pattern checking for expression errors.
func (p *Pattern) UnmarshalText(value []byte) error {
	pattern := string(value)
	pattern = strings.TrimSpace(pattern)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return newError("", ErrorCodePattern, err)
	}

	p.Value = re
	return nil
}

// Scheduler stores the periodicity of an action.
type Scheduler struct {
	Value cron.Schedule
}

// UnmarshalText verifies the cron format of the scheduler entry. For details
// about the expected format please check
// http://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format
func (s *Scheduler) UnmarshalText(value []byte) error {
	scheduler := string(value)
	scheduler = strings.TrimSpace(scheduler)

	schedulerParts := strings.Split(scheduler, " ")
	if len(schedulerParts) != 6 {
		return newError("", ErrorCodeSchedulerFormat, nil)
	}

	var err error
	s.Value, err = cron.Parse(scheduler)
	if err != nil {
		return newError("", ErrorCodeSchedulerValue, err)
	}

	return nil
}
