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
			expected: &config.Config{
				AuditFile:   path.Join("var", "log", "toglacier", "audit.log"),
				KeepBackups: 10,
			},
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
audit file: /var/log/toglacier/audit.log
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
				c.AuditFile = "/var/log/toglacier/audit.log"
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
- /usr/local/important-files-1
- /usr/local/important-files-2
`)

			var scenario scenario
			scenario.description = "it should detect an invalid YAML configuration file"
			scenario.filename = f.Name()

			scenario.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &yaml.TypeError{
					Errors: []string{
						"line 2: cannot unmarshal !!seq into config.Config",
					},
				},
			}

			return scenario
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
audit file: /var/log/toglacier/audit.log
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
aws:
  account id: encrypted:invalid
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var scenario scenario
			scenario.description = "it should detect invalid encrypted values"
			scenario.filename = f.Name()

			scenario.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeDecodeBase64,
					Err:  base64.CorruptInputError(4),
				},
			}

			return scenario
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
audit file: /var/log/toglacier/audit.log
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
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

			var scenario scenario
			scenario.description = "it should detect an invalid backup secret"
			scenario.filename = f.Name()

			scenario.expectedError = &config.Error{
				Filename: f.Name(),
				Code:     config.ErrorCodeParsingYAML,
				Err: &config.Error{
					Code: config.ErrorCodeDecodeBase64,
					Err:  base64.CorruptInputError(4),
				},
			}

			return scenario
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
audit file: /var/log/toglacier/audit.log
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
				c.AuditFile = "/var/log/toglacier/audit.log"
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
audit file: /var/log/toglacier/audit.log
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
				c.AuditFile = "/var/log/toglacier/audit.log"
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
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.AuditFile = "/var/log/toglacier/audit.log"
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
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
				return c
			}(),
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "a123456789012345678901234567890",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.AuditFile = "/var/log/toglacier/audit.log"
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "a12345678901234567890123456789012",
			},
			expected: func() *config.Config {
				c := new(config.Config)
				c.Paths = []string{
					"/usr/local/important-files-1",
					"/usr/local/important-files-2",
				}
				c.AuditFile = "/var/log/toglacier/audit.log"
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
				c.AWS.AccountID.Value = "000000000000"
				c.AWS.AccessKeyID.Value = "AAAAAAAAAAAAAAAAAAAA"
				c.AWS.SecretAccessKey.Value = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				c.AWS.Region = "us-east-1"
				c.AWS.VaultName = "backup"
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
