package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"ios-signer-service/util"
	"log"
	"strings"
)

type Config struct {
	WorkflowTriggerUrl    string `yaml:"workflow_trigger_url"`
	WorkflowStatusUrl     string `yaml:"workflow_status_url"`
	WorkflowData          string `yaml:"workflow_data"`
	WorkflowAuthorization string `yaml:"workflow_authorization"`
	WorkflowKey           string `yaml:"workflow_key"`
	ServerUrl             string `yaml:"server_url"`
	SaveDir               string `yaml:"save_dir"`
	CleanupMins           uint64 `yaml:"cleanup_mins"`
	CleanupIntervalMins   uint64 `yaml:"cleanup_interval_mins"`
}

func createDefaultConfig() *Config {
	return &Config{
		WorkflowTriggerUrl:    "https://api.github.com/repos/foo/bar/actions/workflows/sign.yml/dispatches",
		WorkflowData:          `{"ref":"master"}`,
		WorkflowAuthorization: "Token 65eaa9c8ef52460d22a93307fe0aee76289dc675",
		WorkflowStatusUrl:     "https://github.com/foo/bar/actions/workflows/sign.yml",
		ServerUrl:             "http://localhost:8080",
		SaveDir:               "data",
		WorkflowKey:           "MY_SUPER_LONG_SECRET_KEY",
		CleanupMins:           60 * 24 * 7,
		CleanupIntervalMins:   30,
	}
}

var Current *Config

func init() {
	viper.SetConfigName("signer-cfg")
	// toml bug: https://github.com/spf13/viper/issues/488
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	cfg, err := getConfig()
	if err != nil {
		log.Fatalln(errors.WithMessage(err, "get config"))
	}

	if len(strings.TrimSpace(cfg.WorkflowKey)) < 16 {
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
