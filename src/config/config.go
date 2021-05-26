package config

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/knadh/koanf"
	kyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	kfile "github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"ios-signer-service/src/builders"
	"os"
	"path/filepath"
	"reflect"
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
	RedirectHttps       bool      `yaml:"redirect_https"`
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
		RedirectHttps:       false,
		SaveDir:             "data",
		CleanupMins:         60 * 24 * 7,
		CleanupIntervalMins: 5,
		SignTimeoutMins:     15,
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
	Builder    builders.Builder
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
	builder := fileConfig.Builder.MakeFirstEnabled()
	if builder == nil {
		log.Fatal().Msg("init: no builder defined")
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
		Builder:    builder,
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
	if err := k.Load(env.Provider("", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	if err := inferDelimiters(k, k.All(), mapDelim); err != nil {
		return nil, errors.WithMessage(err, "infer delimiters")
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
	if err := k.Load(env.Provider("", "_", func(s string) string {
		return strings.ToLower(s)
	}), nil); err != nil {
		return nil, errors.WithMessage(err, "load envvars")
	}
	if err := inferDelimiters(k, k.All(), mapDelim); err != nil {
		return nil, errors.WithMessage(err, "infer delimiters")
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

// By convention, environment variables use an underscore '_' as delimiter. YAML entries also use the same delimiter.
// This creates an ambiguity problem. For an example, take the following structure:
//   builder:
//     github:
//       repo_name:
// The corresponding environment variable would ideally be:
//    BUILDER_GITHUB_REPO_NAME
// However, a parser cannot know whether REPO and NAME are two nested entries or one entry with the name 'REPO_NAME'.
//
// inferDelimiters will take a configuration map with both properly separated entries (builder.github.repo_name) and
// greedily separated entries (builder.github.repo.name), and given the map separation delimiter, will transfer the
// values of all greedily separated entries onto the properly separated entries. This process effectively "fixes" the
// ambiguity problem and allows the configuration map to be used as usual.
//
// The input map will be modified.
func inferDelimiters(k *koanf.Koanf, data map[string]interface{}, mapDelim rune) error {
	for name, val := range data {
		for name2, val2 := range data {
			if name == name2 {
				continue
			}
			splitFunc := func(r rune) bool {
				return r == '_' || r == mapDelim
			}
			split := strings.FieldsFunc(name, splitFunc)
			split2 := strings.FieldsFunc(name2, splitFunc)
			if reflect.DeepEqual(split, split2) {
				var srcName string
				var dstVal interface{}
				if strings.Count(name, string(mapDelim)) > strings.Count(name2, string(mapDelim)) {
					srcName = name2
					dstVal = val
				} else {
					srcName = name
					dstVal = val2
				}
				if err := k.Load(confmap.Provider(map[string]interface{}{srcName: dstVal}, string(mapDelim)), nil); err != nil {
					return errors.WithMessage(err, "load map with new value")
				}
				delete(data, name)
				delete(data, name2)
			}
		}
	}
	return nil
}
