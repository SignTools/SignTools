package config

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"ios-signer-service/builders"
	"log"
	"os"
	"path/filepath"
)

type BasicAuth struct {
	Enable   bool   `yaml:"enable"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Builder struct {
	GitHub    builders.GitHubData    `yaml:"github"`
	Semaphore builders.SemaphoreData `yaml:"semaphore"`
	Generic   builders.GenericData   `yaml:"generic"`
}

func (b *Builder) MakeFirstEnabled() builders.Builder {
	if b.GitHub.Enabled {
		return builders.MakeGitHub(&b.GitHub)
	}
	if b.Semaphore.Enabled {
		return builders.MakeSemaphore(&b.Semaphore)
	}
	if b.Generic.Enabled {
		return builders.MakeGeneric(&b.Generic)
	}
	return nil
}

type File struct {
	Builder             Builder   `yaml:"builder"`
	ServerUrl           string    `yaml:"server_url"`
	SaveDir             string    `yaml:"save_dir"`
	CleanupMins         uint64    `yaml:"cleanup_mins"`
	CleanupIntervalMins uint64    `yaml:"cleanup_interval_mins"`
	BasicAuth           BasicAuth `yaml:"basic_auth"`
}

func createDefaultFile() *File {
	return &File{
		Builder: Builder{
			GitHub: builders.GitHubData{
				Enabled:          false,
				RepoName:         "ios-signer-ci",
				OrgName:          "YOUR_PROFILE_NAME",
				WorkflowFileName: "sign.yml",
				Token:            "YOUR_TOKEN",
				Ref:              "master",
			},
			Semaphore: builders.SemaphoreData{
				Enabled:    false,
				ProjectId:  "YOUR_PROJECT_ID",
				OrgName:    "YOUR_ORG_NAME",
				Token:      "YOUR_TOKEN",
				Ref:        "refs/heads/master",
				SecretName: "ios-signer",
			},
			Generic: builders.GenericData{
				Enabled:     false,
				StatusUrl:   "http://localhost:1234/status",
				TriggerUrl:  "http://localhost:1234/trigger",
				SecretsUrl:  "http://localhost:1234/secrets",
				TriggerBody: "hello",
				Headers: map[string]string{
					"Authroziation": "Token YOUR_TOKEN",
				},
				AttemptHTTP2: true,
			},
		},
		ServerUrl:           "http://localhost:8080",
		SaveDir:             "data",
		CleanupMins:         60 * 24 * 7,
		CleanupIntervalMins: 30,
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
	*File
}

var Current Config

func Load(fileName string) {
	allowedExts := []string{".yml", ".yaml"}
	if !isAllowedExt(allowedExts, fileName) {
		log.Fatalf("config extension not allowed: %v\n", allowedExts)
	}
	fileConfig, err := getFile(fileName)
	if err != nil {
		log.Fatalln(errors.WithMessage(err, "get config"))
	}
	builder := fileConfig.Builder.MakeFirstEnabled()
	if builder == nil {
		log.Fatalln("init: no builder defined")
	}
	builderKey := make([]byte, 32)
	if _, err := rand.Read(builderKey); err != nil {
		log.Fatalln("init: error generating builder key: " + err.Error())
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
	fileConfig := createDefaultFile()
	exists, err := readExistingFile(fileName, fileConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "read existing")
	}
	file, err := os.Create(fileName)
	if err != nil {
		return nil, errors.WithMessage(err, "create")
	}
	defer file.Close()
	if err := yaml.NewEncoder(file).Encode(&fileConfig); err != nil {
		return nil, errors.WithMessage(err, "write")
	}
	if !exists {
		return nil, errors.New("no file found, generating one")
	}
	return fileConfig, nil
}

func readExistingFile(fileName string, fileConfig *File) (bool, error) {
	file, err := os.Open(fileName)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return true, errors.WithMessage(err, "open")
	}
	defer file.Close()
	if err := yaml.NewDecoder(file).Decode(fileConfig); err != nil {
		return true, errors.WithMessage(err, "decode")
	}
	return true, nil
}
