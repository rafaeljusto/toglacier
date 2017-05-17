package config

import (
	"io/ioutil"
	"path"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// prefix is used as a namespace identifier for environment variables.
const prefix = "toglacier"

var config unsafe.Pointer

// Config stores all the necessary information to send backups to the cloud and
// keep track in the local storage.
type Config struct {
	Paths        []string `yaml:"paths" envconfig:"paths"`
	KeepBackups  int      `yaml:"keep backups" envconfig:"keep_backups"`
	BackupSecret aesKey   `yaml:"backup secret" envconfig:"backup_secret"`

	Database struct {
		Type DatabaseType `yaml:"type" envconfig:"type"`
		File string       `yaml:"file" envconfig:"file"`
	} `yaml:"database" envconfig:"db"`

	Log struct {
		File  string   `yaml:"file" envconfig:"file"`
		Level LogLevel `yaml:"level" envconfig:"level"`
	} `yaml:"log" envconfig:"log"`

	Email struct {
		Server   string      `yaml:"server" envconfig:"server"`
		Port     int         `yaml:"port" envconfig:"port"`
		Username string      `yaml:"username" envconfig:"username"`
		Password encrypted   `yaml:"password" envconfig:"password"`
		From     string      `yaml:"from" envconfig:"from"`
		To       []string    `yaml:"to" envconfig:"to"`
		Format   EmailFormat `yaml:"format" envconfig:"format"`
	} `yaml:"email" envconfig:"email"`

	AWS struct {
		AccountID       encrypted `yaml:"account id" envconfig:"account_id"`
		AccessKeyID     encrypted `yaml:"access key id" envconfig:"access_key_id"`
		SecretAccessKey encrypted `yaml:"secret access key" envconfig:"secret_access_key"`
		Region          string    `yaml:"region" envconfig:"region"`
		VaultName       string    `yaml:"vault name" envconfig:"vault_name"`
	} `yaml:"aws" envconfig:"aws"`
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
