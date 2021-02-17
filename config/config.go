package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"ios-signer-service/util"
	"log"
	"strings"
)

type Config struct {
	GitHubToken         string `yaml:"github_token"`
	RepoOwner           string `yaml:"repo_owner"`
	RepoName            string `yaml:"repo_name"`
	WorkflowFileName    string `yaml:"workflow_file_name"`
	WorkflowRef         string `yaml:"workflow_ref"`
	ServerURL           string `yaml:"server_url"`
	SaveDir             string `yaml:"save_dir"`
	Key                 string `yaml:"key"`
	CertDir             string `yaml:"cert_dir"`
	CertFileName        string `yaml:"cert_file_name"`
	ProvFileName        string `yaml:"prov_file_name"`
	CertPass            string `yaml:"cert_pass"`
	CleanupMins         uint64 `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64 `yaml:"cleanup_interval_mins"`
}

var (
	SaveFilePath        = resolveSavedFileWithId("")
	SaveSignedPath      = resolveSavedFileWithId("signed")
	SaveUnsignedPath    = resolveSavedFileWithId("unsigned")
	SaveWorkflowPath    = resolveSavedFileWithId("job")
	SaveDisplayNamePath = resolveSavedFileWithId("name")
	FormFileName        = "file"
)

var resolveSavedFileWithId = func(path string) func(id string) string {
	return func(id string) string {
		return util.SafeJoin(Current.SaveDir, id, path)
	}
}

func createDefaultConfig() *Config {
	return &Config{
		GitHubToken:         "MY_GITHUB_TOKEN",
		RepoOwner:           "foo",
		RepoName:            "bar",
		WorkflowFileName:    "sign.yml",
		WorkflowRef:         "master",
		ServerURL:           "https://website.com",
		SaveDir:             "uploads",
		Key:                 "MY_SUPER_LONG_SECRET_KEY",
		CertDir:             "certs",
		CertFileName:        "cert.p12",
		ProvFileName:        "prov.mobileprovision",
		CertPass:            "123456",
		CleanupMins:         60 * 2,
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
		log.Fatal(err)
	}

	if len(strings.TrimSpace(cfg.Key)) < 16 {
		log.Fatalln("bad secret, must be at least 16 characters long")
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
		return nil, err
	}
	return cfg, nil
}

func handleNoConfigFile(config *Config) error {
	defaultMap := make(map[string]interface{})
	if err := util.Restructure(config, &defaultMap); err != nil {
		return err
	}
	if err := viper.MergeConfigMap(defaultMap); err != nil {
		return err
	}
	if err := viper.SafeWriteConfig(); err != nil {
		return err
	}
	return errors.New("no config, template generated")
}
