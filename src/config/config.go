package config

import (
	"SignTools/src/builders"
	"crypto/rand"
	"encoding/hex"
	"github.com/ViRb3/koanf-extra/env"
	"github.com/knadh/koanf"
	kyaml "github.com/knadh/koanf/parsers/yaml"
	kfile "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type BasicAuth struct {
	Enable   bool   `yaml:"enable"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Builder struct {
	GitHub     builders.GitHubData     `yaml:"github"`
	Semaphore  builders.SemaphoreData  `yaml:"semaphore"`
	SelfHosted builders.SelfHostedData `yaml:"selfhosted"`
}

func (b *Builder) MakeEnabled() map[string]builders.Builder {
	results := map[string]builders.Builder{}
	if b.GitHub.Enable {
		results["GitHub"] = builders.MakeGitHub(&b.GitHub)
	}
	if b.Semaphore.Enable {
		results["Semaphore"] = builders.MakeSemaphore(&b.Semaphore)
	}
	if b.SelfHosted.Enable {
		results["SelfHosted"] = builders.MakeSelfHosted(&b.SelfHosted)
	}
	return results
}

type File struct {
	Builder             Builder   `yaml:"builder"`
	ServerUrl           string    `yaml:"server_url"`
	RedirectHttps       bool      `yaml:"redirect_https"`
	SaveDir             string    `yaml:"save_dir"`
	CleanupIntervalMins uint64    `yaml:"cleanup_interval_mins"`
	SignTimeoutMins     uint64    `yaml:"sign_timeout_mins"`
	BasicAuth           BasicAuth `yaml:"basic_auth"`
}

func createDefaultFile() *File {
	return &File{
		Builder: Builder{
			GitHub: builders.GitHubData{
				Enable:           false,
				RepoName:         "SignTools-CI",
				OrgName:          "YOUR_PROFILE_NAME",
				WorkflowFileName: "sign.yml",
				Token:            "YOUR_TOKEN",
				Ref:              "master",
			},
			Semaphore: builders.SemaphoreData{
				Enable:      false,
				ProjectName: "YOUR_PROJECT_NAME",
				OrgName:     "YOUR_ORG_NAME",
				Token:       "YOUR_TOKEN",
				Ref:         "refs/heads/master",
				SecretName:  "ios-signer",
			},
			SelfHosted: builders.SelfHostedData{
				Enable: false,
				Url:    "http://192.168.1.133:8090",
				Key:    "SOME_SECRET_KEY",
			},
		},
		ServerUrl:           "http://localhost:8080",
		RedirectHttps:       false,
		SaveDir:             "data",
		SignTimeoutMins:     30,
		CleanupIntervalMins: 1,
		BasicAuth: BasicAuth{
			Enable:   false,
			Username: "admin",
			Password: "admin",
		},
	}
}

type EnvProfile struct {
	Name        string `yaml:"name"`
	ProvBase64  string `yaml:"prov_base64"`
	CertPass    string `yaml:"cert_pass"`
	CertBase64  string `yaml:"cert_base64"`
	AccountName string `yaml:"account_name"`
	AccountPass string `yaml:"account_pass"`
}

type ProfileBox struct {
	EnvProfile `yaml:"profile"`
}

type Config struct {
	Builder    map[string]builders.Builder
	BuilderKey string
	*File
	EnvProfile *EnvProfile
}

var Current Config

func Load(fileName string) {
	allowedExts := []string{".yml", ".yaml"}
	if !isAllowedExt(allowedExts, fileName) {
		log.Fatal().Msgf("config extension not allowed: %v\n", allowedExts)
	}
	mapDelim := '.'
	fileConfig, err := getFile(mapDelim, fileName)
	if err != nil {
		log.Fatal().Err(err).Msg("get config")
	}
	builderMap := fileConfig.Builder.MakeEnabled()
	if len(builderMap) < 1 {
		log.Fatal().Msg("init: no builders defined")
	}
	builderKey := make([]byte, 32)
	if _, err := rand.Read(builderKey); err != nil {
		log.Fatal().Err(err).Msg("init: error generating builder key")
	}
	profile, err := getProfileFromEnv(mapDelim)
	if err != nil {
		log.Fatal().Err(err).Msg("init: error checking for signing profile from envvars")
	}
	Current = Config{
		Builder:    builderMap,
		BuilderKey: hex.EncodeToString(builderKey),
		File:       fileConfig,
		EnvProfile: profile,
	}
}

// Loads a single signing profile entirely from environment variables.
// Intended for use with Heroku without persistent storage.
func getProfileFromEnv(mapDelim rune) (*EnvProfile, error) {
	k := koanf.New(string(mapDelim))
	if err := k.Load(structs.Provider(ProfileBox{}, "yaml"), nil); err != nil {
		return nil, errors.WithMessage(err, "load default")
	}
	if err := k.Load(env.Provider(k, "", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	profile := EnvProfile{}
	if err := k.UnmarshalWithConf("profile", &profile, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, errors.WithMessage(err, "unmarshal")
	}
	return &profile, nil
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

func getFile(mapDelim rune, fileName string) (*File, error) {
	k := koanf.New(string(mapDelim))
	if err := k.Load(structs.Provider(createDefaultFile(), "yaml"), nil); err != nil {
		return nil, errors.WithMessage(err, "load default")
	}
	if err := k.Load(kfile.Provider(fileName), kyaml.Parser()); os.IsNotExist(err) {
		log.Info().Str("name", fileName).Msg("creating config file")
	} else if err != nil {
		return nil, errors.WithMessage(err, "load existing")
	}
	// TODO: Fix entries with same path overwriting each other, e.g. PROFILE_CERT_NAME="bar" and PROFILE_CERT="foo"
	if err := k.Load(env.Provider(k, "", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	fileConfig := File{}
	if err := k.UnmarshalWithConf("", &fileConfig, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, errors.WithMessage(err, "unmarshal")
	}
	file, err := os.Create(fileName)
	if err != nil {
		return nil, errors.WithMessage(err, "create")
	}
	defer file.Close()
	if err := yaml.NewEncoder(file).Encode(&fileConfig); err != nil {
		return nil, errors.WithMessage(err, "write")
	}
	return &fileConfig, nil
}
