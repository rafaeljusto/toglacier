package config_test

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/aryann/difflib"
	"github.com/davecgh/go-spew/spew"
	"github.com/kelseyhightower/envconfig"
	"github.com/rafaeljusto/toglacier/internal/config"
	"gopkg.in/yaml.v2"
)

func TestDefault(t *testing.T) {
	scenarios := []struct {
		description string
		expected    *config.Config
	}{
		{
			description: "it should set the default configuration values",
			expected: func() *config.Config {
				c := new(config.Config)
				c.Database.Type = config.DatabaseTypeBoltDB
				c.Database.File = path.Join("var", "log", "toglacier", "toglacier.db")
				c.KeepBackups = 10
				c.Log.Level = config.LogLevelError
				c.Email.Format = config.EmailFormatHTML
				return c
			}(),
		},
	}

	originalConfig := config.Current()
	defer func() {
		config.Update(originalConfig)
	}()

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			config.Default()

			if c := config.Current(); !reflect.DeepEqual(scenario.expected, c) {
				t.Errorf("config don't match.\n%s", Diff(scenario.expected, c))
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	type scenario struct {
		description   string
		filename      string
		expected      *config.Config
		expectedError error
	}

	scenarios := []scenario{
		{
			description: "it should load a YAML configuration file correctly",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-")
				if err != nil {
					t.Fatalf("error creating a temporary file. details %s", err)
				}
				defer f.Close()

				f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level:   DEBUG
keep backups: 10
backup secret: encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

				return f.Name()
			}(),
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "abc12300000000000000000000000000"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		{
			description: "it should detect when the file doesn't exist",
			filename:    "toglacier-idontexist.tmp",
			expectedError: &config.Error{
				Filename: "toglacier-idontexist.tmp",
				Code:     config.ErrorCodeReadingFile,
				Err: &os.PathError{
					Op:   "open",
					Path: "toglacier-idontexist.tmp",
					Err:  syscall.Errno(2),
				},
			},
		},
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: idontexist
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: error
keep backups: 10
backup secret: encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var s scenario
			s.description = "it should detect when the database type is unknown"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeDatabaseType,
				},
			}

			return s
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: idontexist
keep backups: 10
backup secret: encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var s scenario
			s.description = "it should detect when the log level is unknown"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeLogLevel,
				},
			}

			return s
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
- /usr/local/important-files-1
- /usr/local/important-files-2
`)

			var s scenario
			s.description = "it should detect an invalid YAML configuration file"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &yaml.TypeError{
					Errors: []string{
						"line 2: cannot unmarshal !!seq into config.Config",
					},
				},
			}

			return s
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: debug
keep backups: 10
backup secret: encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:invalid
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var s scenario
			s.description = "it should detect invalid encrypted values"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeDecodeBase64,
					Err:  base64.CorruptInputError(4),
				},
			}

			return s
		}(),
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: debug
keep backups: 10
backup secret: encrypted:invalid
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var s scenario
			s.description = "it should detect an invalid backup secret"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeDecodeBase64,
					Err:  base64.CorruptInputError(4),
				},
			}

			return s
		}(),
		{
			description: "it should fill the backup secret when is less than 32 bytes",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-")
				if err != nil {
					t.Fatalf("error creating a temporary file. details %s", err)
				}
				defer f.Close()

				f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: debug
keep backups: 10
backup secret: a123456789012345678901234567890
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

				return f.Name()
			}(),
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "a1234567890123456789012345678900"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		{
			description: "it should truncate the backup secret when is more than 32 bytes",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-")
				if err != nil {
					t.Fatalf("error creating a temporary file. details %s", err)
				}
				defer f.Close()

				f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level: debug
keep backups: 10
backup secret: a12345678901234567890123456789012
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: html
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

				return f.Name()
			}(),
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "a1234567890123456789012345678901"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		func() scenario {
			f, err := ioutil.TempFile("", "toglacier-")
			if err != nil {
				t.Fatalf("error creating a temporary file. details %s", err)
			}
			defer f.Close()

			f.WriteString(`
paths:
  - /usr/local/important-files-1
  - /usr/local/important-files-2
database:
  type: audit-file
  file: /var/log/toglacier/audit.log
log:
  file: /var/log/toglacier/toglacier.log
  level:   DEBUG
keep backups: 10
backup secret: encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==
email:
  server: smtp.example.com
  port: 587
  username: user@example.com
  password: encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==
  from: user@example.com
  to:
    - report1@example.com
    - report2@example.com
  format: strange
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var s scenario
			s.description = "it should detect an invalid e-mail format"
			s.filename = f.Name()
			s.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeEmailFormat,
				},
			}

			return s
		}(),
	}

	originalConfig := config.Current()
	defer func() {
		config.Update(originalConfig)
	}()

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			config.Update(originalConfig)
			err := config.LoadFromFile(scenario.filename)

			if c := config.Current(); !reflect.DeepEqual(scenario.expected, c) {
				t.Errorf("config don't match.\n%s", Diff(scenario.expected, c))
			}

			if !config.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	scenarios := []struct {
		description   string
		env           map[string]string
		expected      *config.Config
		expectedError error
	}{
		{
			description: "it should load the configuration from environment variables correctly",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "  DEBUG  ",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "abc12300000000000000000000000000"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		{
			description: "it should detect an invalid database type",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "idontexist",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "error",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expectedError: &config.Error{
				Code: config.ErrorCodeReadingEnvVars,
				Err: &envconfig.ParseError{
					KeyName:   "TOGLACIER_DB_TYPE",
					FieldName: "Type",
					TypeName:  "config.DatabaseType",
					Value:     "idontexist",
					Err: &config.Error{
						Code: config.ErrorCodeDatabaseType,
					},
				},
			},
		},
		{
			description: "it should detect an invalid log level",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "idontexist",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expectedError: &config.Error{
				Code: config.ErrorCodeReadingEnvVars,
				Err: &envconfig.ParseError{
					KeyName:   "TOGLACIER_LOG_LEVEL",
					FieldName: "Level",
					TypeName:  "config.LogLevel",
					Value:     "idontexist",
					Err: &config.Error{
						Code: config.ErrorCodeLogLevel,
					},
				},
			},
		},
		{
			description: "it should detect invalid encrypted values",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:invalid",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "debug",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expectedError: &config.Error{
				Code: config.ErrorCodeReadingEnvVars,
				Err: &envconfig.ParseError{
					KeyName:   "TOGLACIER_AWS_ACCOUNT_ID",
					FieldName: "AccountID",
					TypeName:  "config.encrypted",
					Value:     "encrypted:invalid",
					Err: &config.Error{
						Code: config.ErrorCodeDecodeBase64,
						Err:  base64.CorruptInputError(4),
					},
				},
			},
		},
		{
			description: "it should detect an invalid backup secret",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "debug",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:invalid",
			},
			expectedError: &config.Error{
				Code: config.ErrorCodeReadingEnvVars,
				Err: &envconfig.ParseError{
					KeyName:   "TOGLACIER_BACKUP_SECRET",
					FieldName: "BackupSecret",
					TypeName:  "config.aesKey",
					Value:     "encrypted:invalid",
					Err: &config.Error{
						Code: config.ErrorCodeDecodeBase64,
						Err:  base64.CorruptInputError(4),
					},
				},
			},
		},
		{
			description: "it should fill the backup secret when is less than 32 bytes",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "debug",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "a123456789012345678901234567890",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "a1234567890123456789012345678900"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		{
			description: "it should truncate the backup secret when is more than 32 bytes",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "html",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "debug",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "a12345678901234567890123456789012",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.Database.Type = config.DatabaseTypeAuditFile
				c.Database.File = "/var/log/toglacier/audit.log"
				c.Log.File = "/var/log/toglacier/toglacier.log"
				c.Log.Level = config.LogLevelDebug
				c.KeepBackups = 10
				c.BackupSecret.Value = "a1234567890123456789012345678901"
				c.Email.Server = "smtp.example.com"
				c.Email.Port = 587
				c.Email.Username = "user@example.com"
				c.Email.Password.Value = "abc123"
				c.Email.From = "user@example.com"
				c.Email.To = []string{
					"report1@example.com",
					"report2@example.com",
				}
				c.Email.Format = config.EmailFormatHTML
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
		},
		{
			description: "it should detect an invalid e-mail format",
			env: map[string]string{
				"TOGLACIER_AWS_ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"TOGLACIER_AWS_ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"TOGLACIER_AWS_SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"TOGLACIER_AWS_REGION":            "us-east-1",
				"TOGLACIER_AWS_VAULT_NAME":        "backup",
				"TOGLACIER_EMAIL_SERVER":          "smtp.example.com",
				"TOGLACIER_EMAIL_PORT":            "587",
				"TOGLACIER_EMAIL_USERNAME":        "user@example.com",
				"TOGLACIER_EMAIL_PASSWORD":        "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"TOGLACIER_EMAIL_FROM":            "user@example.com",
				"TOGLACIER_EMAIL_TO":              "report1@example.com,report2@example.com",
				"TOGLACIER_EMAIL_FORMAT":          "strange",
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_DB_TYPE":               "audit-file",
				"TOGLACIER_DB_FILE":               "/var/log/toglacier/audit.log",
				"TOGLACIER_LOG_FILE":              "/var/log/toglacier/toglacier.log",
				"TOGLACIER_LOG_LEVEL":             "  DEBUG  ",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expectedError: &config.Error{
				Code: config.ErrorCodeReadingEnvVars,
				Err: &envconfig.ParseError{
					KeyName:   "TOGLACIER_EMAIL_FORMAT",
					FieldName: "Format",
					TypeName:  "config.EmailFormat",
					Value:     "strange",
					Err: &config.Error{
						Code: config.ErrorCodeEmailFormat,
					},
				},
			},
		},
		{
			description: "it should ignore environment variables without prefix",
			env: map[string]string{
				"ACCOUNT_ID":        "encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==",
				"ACCESS_KEY_ID":     "encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ",
				"SECRET_ACCESS_KEY": "encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=",
				"REGION":            "us-east-1",
				"VAULT_NAME":        "backup",
				"SERVER":            "smtp.example.com",
				"PORT":              "587",
				"USERNAME":          "user@example.com",
				"PASSWORD":          "encrypted:i9dw0HZPOzNiFgtEtrr0tiY0W+YYlA==",
				"FROM":              "user@example.com",
				"TO":                "report1@example.com,report2@example.com",
				"FORMAT":            "html",
				"PATHS":             "/usr/local/important-files-1,/usr/local/important-files-2",
				"TYPE":              "audit-file",
				"FILE":              "/var/log/toglacier/audit.log",
				"LEVEL":             "  DEBUG  ",
				"KEEP_BACKUPS":      "10",
				"BACKUP_SECRET":     "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expected: func() *config.Config {
				return new(config.Config)
			}(),
		},
	}

	originalConfig := config.Current()
	defer func() {
		config.Update(originalConfig)
	}()

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			config.Update(originalConfig)

			os.Clearenv()
			for key, value := range scenario.env {
				os.Setenv(key, value)
			}

			err := config.LoadFromEnvironment()

			if c := config.Current(); !reflect.DeepEqual(scenario.expected, c) {
				t.Errorf("config don't match.\n%s", Diff(scenario.expected, c))
			}

			if !config.ErrorEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

// Diff is useful to see the difference when comparing two complex types.
func Diff(a, b interface{}) []difflib.DiffRecord {
	return difflib.Diff(strings.SplitAfter(spew.Sdump(a), "\n"), strings.SplitAfter(spew.Sdump(b), "\n"))
}
