package storage

import (
	"github.com/rs/zerolog/log"
	"io"
	"ios-signer-service/src/config"
	"os"
	"path/filepath"
)

var (
	appsPath     string
	profilesPath string
)

type ReadonlyFile interface {
	io.ReadSeekCloser
	io.ReaderAt
	Stat() (os.FileInfo, error)
}

var Apps = newAppResolver()
var Profiles = newProfileResolver()
var Jobs = newJobResolver()

func Load() {
	appsPath = filepath.Join(config.Current.SaveDir, "apps")
	profilesPath = filepath.Join(config.Current.SaveDir, "profiles")
	requiredPaths := []string{appsPath, profilesPath}
	for _, path := range requiredPaths {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			log.Fatal().Err(err).Msg("mkdir required path")
		}
	}
	if err := Apps.refresh(); err != nil {
		log.Fatal().Err(err).Msg("refresh apps")
	}
	if err := Profiles.refresh(); err != nil {
		log.Fatal().Err(err).Msg("refresh profiles")
	}
}

type fileGetter struct {
	name string
	f1   func() (ReadonlyFile, error)
	f2   func() (string, error)
	f3   func() ([]byte, error)
}
