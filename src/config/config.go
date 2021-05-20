package config

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/knadh/koanf"
	kyaml "github.com/knadh/koanf/parsers/yaml"
	kfile "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"ios-signer-service/src/builders"
	"os"
	"path/filepath"
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

func (b *Builder) MakeFirstEnabled() builders.Builder {
	if b.GitHub.Enable {
		return builders.MakeGitHub(&b.GitHub)
	}
	if b.Semaphore.Enable {
		return builders.MakeSemaphore(&b.Semaphore)
	}
	if b.SelfHosted.Enable {
		return builders.MakeSelfHosted(&b.SelfHosted)
	}
	return nil
}

type File struct {
	Builder             Builder   `yaml:"builder"`
	ServerUrl           string    `yaml:"server_url"`
	SaveDir             string    `yaml:"save_dir"`
	CleanupMins         uint64    `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64    `yaml:"cleanup_interval_mins"`
	SignTimeoutMins     uint64    `yaml:"sign_timeout_mins"`
	BasicAuth           BasicAuth `yaml:"basic_auth"`
}

func createDefaultFile() *File {
	return &File{
		Builder: Builder{
			GitHub: builders.GitHubData{
				Enable:           false,
				RepoName:         "ios-signer-ci",
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
		SaveDir:             "data",
		CleanupMins:         60 * 24 * 7,
		CleanupIntervalMins: 30,
		SignTimeoutMins:     15,
		BasicAuth: BasicAuth{
			Enable:   false,
			Username: "admin",
			Password: "admin",
		},
	}
}

type Config struct {
	Builder    builders.Builder
	BuilderKey string
	PublicUrl  string
	*File
}

var Current Config

func Load(fileName string) {
	allowedExts := []string{".yml", ".yaml"}
	if !isAllowedExt(allowedExts, fileName) {
		log.Fatal().Msgf("config extension not allowed: %v\n", allowedExts)
	}
	fileConfig, err := getFile(fileName)
	if err != nil {
		log.Fatal().Err(err).Msg("get config")
	}
	builder := fileConfig.Builder.MakeFirstEnabled()
	if builder == nil {
		log.Fatal().Msg("init: no builder defined")
	}
	builderKey := make([]byte, 32)
	if _, err := rand.Read(builderKey); err != nil {
		log.Fatal().Err(err).Msg("init: error generating builder key")
	}
	Current = Config{
		Builder:    builder,
		BuilderKey: hex.EncodeToString(builderKey),
		File:       fileConfig,
	}
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

func getFile(fileName string) (*File, error) {
	mapDelim := '.'
	k := koanf.New(string(mapDelim))
	if err := k.Load(structs.Provider(createDefaultFile(), "yaml"), nil); err != nil {
		return nil, errors.WithMessage(err, "load default")
	}
	if err := k.Load(kfile.Provider(fileName), kyaml.Parser()); os.IsNotExist(err) {
		log.Info().Str("name", fileName).Msg("config file created")
	} else if err != nil {
		return nil, errors.WithMessage(err, "load existing")
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
