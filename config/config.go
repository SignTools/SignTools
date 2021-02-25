package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"ios-signer-service/util"
	"log"
)

type Config struct {
	GitHubToken         string `yaml:"github_token"`
	RepoOwner           string `yaml:"repo_owner"`
	RepoName            string `yaml:"repo_name"`
	WorkflowFileName    string `yaml:"workflow_file_name"`
	WorkflowRef         string `yaml:"workflow_ref"`
	ServerURL           string `yaml:"server_url"`
	SaveDir             string `yaml:"save_dir"`
	CertPass            string `yaml:"cert_pass"`
	CleanupMins         uint64 `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64 `yaml:"cleanup_interval_mins"`
}

func createDefaultConfig() *Config {
	return &Config{
		GitHubToken:         "MY_GITHUB_TOKEN",
		RepoOwner:           "foo",
		RepoName:            "bar",
		WorkflowFileName:    "sign.yml",
		WorkflowRef:         "master",
		ServerURL:           "http://localhost:8080",
		SaveDir:             "data",
		CleanupMins:         60 * 24 * 7,
		CleanupIntervalMins: 30,
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
