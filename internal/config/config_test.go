package config_test

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"syscall"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/kr/pretty"
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
				t.Errorf("config don't match.\n%s", pretty.Diff(scenario.expected, c))
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	scenarios := []struct {
		description   string
		filename      string
		expected      *config.Config
		expectedError error
	}{
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
			expectedError: &os.PathError{
				Op:   "open",
				Path: "toglacier-idontexist.tmp",
				Err:  syscall.Errno(2),
			},
		},
		{
			description: "it should detect an invalid YAML configuration file",
			filename: func() string {
				f, err := ioutil.TempFile("", "toglacier-")
				if err != nil {
					t.Fatalf("error creating a temporary file. details %s", err)
				}
				defer f.Close()

				f.WriteString(`
- /usr/local/important-files-1
- /usr/local/important-files-2
`)

				return f.Name()
			}(),
			expectedError: &yaml.TypeError{
				Errors: []string{
					"line 2: cannot unmarshal !!seq into config.Config",
				},
			},
		},
		{
			description: "it should detect invalid encrypted values",
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
aws:
  account id: encrypted:invalid
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

				return f.Name()
			}(),
			expectedError: fmt.Errorf("error decrypting value. details: %s", base64.CorruptInputError(4)),
		},
		{
			description: "it should detect an invalid backup secret",
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
backup secret: encrypted:invalid
aws:
  account id: encrypted:DueEGILYe8OoEp49Qt7Gymms2sPuk5weSPiG6w==
  access key id: encrypted:XesW4TPKzT3Cgw1SCXeMB9Pb2TssRPCdM4mrPwlf4zWpzSZQ
  secret access key: encrypted:hHHZXW+Uuj+efOA7NR4QDAZh6tzLqoHFaUHkg/Yw1GE/3sJBi+4cn81LhR8OSVhNwv1rI6BR4fA=
  region: us-east-1
  vault name: backup
`)

				return f.Name()
			}(),
			expectedError: fmt.Errorf("error decrypting value. details: %s", base64.CorruptInputError(4)),
		},
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
				t.Errorf("config don't match.\n%s", pretty.Diff(scenario.expected, c))
			}

			if !reflect.DeepEqual(scenario.expectedError, err) {
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:M5rNhMpetktcTEOSuF25mYNn97TN1w==",
			},
			expectedError: &envconfig.ParseError{
				KeyName:   "TOGLACIER_AWS_ACCOUNT_ID",
				FieldName: "AccountID",
				TypeName:  "config.encrypted",
				Value:     "encrypted:invalid",
				Err:       fmt.Errorf("error decrypting value. details: %s", base64.CorruptInputError(4)),
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
				"TOGLACIER_PATHS":                 "/usr/local/important-files-1,/usr/local/important-files-2",
				"TOGLACIER_AUDIT":                 "/var/log/toglacier/audit.log",
				"TOGLACIER_KEEP_BACKUPS":          "10",
				"TOGLACIER_BACKUP_SECRET":         "encrypted:invalid",
			},
			expectedError: &envconfig.ParseError{
				KeyName:   "TOGLACIER_BACKUP_SECRET",
				FieldName: "BackupSecret",
				TypeName:  "config.aesKey",
				Value:     "encrypted:invalid",
				Err:       fmt.Errorf("error decrypting value. details: %s", base64.CorruptInputError(4)),
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
				t.Errorf("config don't match.\n%s", pretty.Diff(scenario.expected, c))
			}

			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}
