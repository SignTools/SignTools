package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"ios-signer-service/util"
	"log"
	"path/filepath"
	"strings"
)

type Trigger struct {
	Url     string            `yaml:"url"`
	Body    string            `yaml:"body"`
	Headers map[string]string `yaml:"headers"`
}

type Workflow struct {
	Trigger   Trigger
	StatusUrl string `yaml:"status_url"`
	Key       string `yaml:"key"`
}

type Config struct {
	Workflow            Workflow `yaml:"workflow"`
	ServerUrl           string   `yaml:"server_url"`
	SaveDir             string   `yaml:"save_dir"`
	CleanupMins         uint64   `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64   `yaml:"cleanup_interval_mins"`
}

func createDefaultConfig() *Config {
	return &Config{
		Workflow: Workflow{
			Trigger: Trigger{
				Url:  "https://api.github.com/repos/foo/bar/actions/workflows/sign.yml/dispatches",
				Body: `{"ref":"master"}`,
				Headers: map[string]string{
					"Authorization": "Token 65eaa9c8ef52460d22a93307fe0aee76289dc675",
				},
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
	viper.SetConfigName(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	// toml bug: https://github.com/spf13/viper/issues/488
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	cfg, err := getConfig()
	if err != nil {
		log.Fatalln(errors.WithMessage(err, "get config"))
	}

	if len(strings.TrimSpace(cfg.Workflow.Key)) < 16 {
		log.Fatalln("init: bad workflow key, must be at least 16 characters long")
	}

	Current = cfg
}

func getConfig() (*Config, error) {
	cfg := createDefaultConfig()
	if err := viper.ReadInConfig(); err != nil {
		return nil, handleNoConfigFile(cfg)
	}
	// don't use viper.Unmarshal because it doesn't support nested structs: https://github.com/spf13/viper/issues/488
	if err := util.Restructure(viper.AllSettings(), cfg); err != nil {
		return nil, errors.WithMessage(err, "config restructure")
	}
	return cfg, nil
}

func handleNoConfigFile(config *Config) error {
	defaultMap := make(map[string]interface{})
	if err := util.Restructure(config, &defaultMap); err != nil {
		return errors.WithMessage(err, "restructure")
	}
	if err := viper.MergeConfigMap(defaultMap); err != nil {
		return errors.WithMessage(err, "merge default config")
	}
	if err := viper.SafeWriteConfig(); err != nil {
		return errors.WithMessage(err, "save config")
	}
	return errors.New("file not present, template generated")
}
