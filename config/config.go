package config

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Trigger struct {
	Url          string            `yaml:"url"`
	Body         string            `yaml:"body"`
	Headers      map[string]string `yaml:"headers"`
	AttemptHTTP2 bool              `yaml:"attempt_http2"`
}

type Workflow struct {
	Trigger   Trigger
	StatusUrl string `yaml:"status_url"`
	Key       string `yaml:"key"`
}

type BasicAuth struct {
	Enable   bool   `yaml:"enable"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Config struct {
	Workflow            Workflow  `yaml:"workflow"`
	ServerUrl           string    `yaml:"server_url"`
	SaveDir             string    `yaml:"save_dir"`
	CleanupMins         uint64    `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64    `yaml:"cleanup_interval_mins"`
	BasicAuth           BasicAuth `yaml:"basic_auth"`
}

func createDefaultConfig() *Config {
	return &Config{
		Workflow: Workflow{
			Trigger: Trigger{
				Url:  "https://api.github.com/repos/foo/bar/actions/workflows/sign.yml/dispatches",
				Body: `{"ref":"master"}`,
				Headers: map[string]string{
					"Authorization": "Token MY_TOKEN",
					"Content-Type":  "application/json",
				},
				AttemptHTTP2: true,
			},
			StatusUrl: "https://github.com/foo/bar/actions/workflows/sign.yml",
			Key:       "MY_SUPER_LONG_SECRET_KEY",
		},
		ServerUrl:           "http://localhost:8080",
		SaveDir:             "data",
		CleanupMins:         60 * 24 * 7,
		CleanupIntervalMins: 30,
	}
}

var Current *Config

func Load(fileName string) {
	allowedExts := []string{".yml", ".yaml"}
	if !isAllowedExt(allowedExts, fileName) {
		log.Fatalf("config extension not allowed: %v\n", allowedExts)
	}
	cfg, err := getConfig(fileName)
	if err != nil {
		log.Fatalln(errors.WithMessage(err, "get config"))
	}
	if len(strings.TrimSpace(cfg.Workflow.Key)) < 16 {
		log.Fatalln("init: bad workflow key, must be at least 16 characters long")
	}
	Current = cfg
}

func isAllowedExt(allowedExts []string, fileName string) bool {
	fileExt := filepath.Ext(fileName)
	for _, ext := range allowedExts {
		if fileExt == ext {
			return true
		}
	}
	return false
}

func getConfig(fileName string) (*Config, error) {
	cfg := createDefaultConfig()
	exists, err := readExisting(fileName, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "read existing")
	}
	cfgFile, err := os.Create(fileName)
	if err != nil {
		return nil, errors.WithMessage(err, "create")
	}
	defer cfgFile.Close()
	if err := yaml.NewEncoder(cfgFile).Encode(&cfg); err != nil {
		return nil, errors.WithMessage(err, "write")
	}
	if !exists {
		return nil, errors.New("no file found, generating one")
	}
	return cfg, nil
}

func readExisting(fileName string, cfg *Config) (bool, error) {
	cfgFile, err := os.Open(fileName)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return true, errors.WithMessage(err, "open")
	}
	defer cfgFile.Close()
	if err := yaml.NewDecoder(cfgFile).Decode(cfg); err != nil {
		return true, errors.WithMessage(err, "decode")
	}
	return true, nil
}
