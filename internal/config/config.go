package config

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

// prefix is used as a namespace identifier for environment variables.
const prefix = "toglacier"

var config unsafe.Pointer

// Config stores all the necessary information to send backups to the cloud and
// keep track in the local storage.
type Config struct {
	Paths        []string `yaml:"paths" envconfig:"paths"`
	AuditFile    string   `yaml:"audit file" envconfig:"audit"`
	KeepBackups  int      `yaml:"keep backups" envconfig:"keep_backups"`
	BackupSecret aesKey   `yaml:"backup secret" envconfig:"backup_secret"`

	Email struct {
		Server   string    `yaml:"server" envconfig:"server"`
		Port     int       `yaml:"port" envconfig:"port"`
		Username string    `yaml:"username" envconfig:"username"`
		Password encrypted `yaml:"password" envconfig:"password"`
		From     string    `yaml:"from" envconfig:"from"`
		To       []string  `yaml:"to" envconfig:"to"`
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

	c.AuditFile = path.Join("var", "log", "toglacier", "audit.log")
	c.KeepBackups = 10

	Update(c)
}

// LoadFromFile parse an YAML file and fill the system configuration parameters.
func LoadFromFile(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	c := Current()
	if c == nil {
		c = new(Config)
	}

	if err = yaml.Unmarshal(content, c); err != nil {
		return err
	}

	Update(c)
	return nil
}

// LoadFromEnvironment analysis all project environment variables.
func LoadFromEnvironment() error {
	c := Current()
	if c == nil {
		c = new(Config)
	}

	if err := envconfig.Process(prefix, c); err != nil {
		return err
	}

	Update(c)
	return nil
}

type encrypted struct {
	Value string
}

func (e *encrypted) UnmarshalText(value []byte) error {
	e.Value = string(value)

	if strings.HasPrefix(e.Value, "encrypted:") {
		var err error
		if e.Value, err = passwordDecrypt(strings.TrimPrefix(e.Value, "encrypted:")); err != nil {
			return fmt.Errorf("error decrypting value. details: %s", err)
		}
	}

	return nil
}

type aesKey struct {
	encrypted
}

func (a *aesKey) UnmarshalText(value []byte) error {
	if err := a.encrypted.UnmarshalText(value); err != nil {
		return err
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
