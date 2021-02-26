package storage

import (
	"github.com/pkg/errors"
	"io"
	"ios-signer-service/config"
	"ios-signer-service/util"
	"log"
	"os"
	"path/filepath"
)

var (
	appsPath           string
	appPath            func(id string) string
	appSignedPath      func(id string) string
	appUnsignedPath    func(id string) string
	appWorkflowUrlPath func(id string) string
	appNamePath        func(id string) string
	appProfileIdPath   func(id string) string
	appSignArgsPath    func(id string) string

	profilesPath    string
	profileCertPath func(id string) string
	profilePassPath func(id string) string
	profileProvPath func(id string) string
	profileNamePath func(id string) string
)

type ReadonlyFile interface {
	io.ReadSeekCloser
	Stat() (os.FileInfo, error)
}

var Apps = newAppResolver()
var Profiles = newProfileResolver()
var Jobs = newJobResolver()

func Load() {
	appsPath = filepath.Join(config.Current.SaveDir, "apps")
	appPath = resolveLocationWithId(appsPath, "")
	appSignedPath = resolveLocationWithId(appsPath, "signed")
	appUnsignedPath = resolveLocationWithId(appsPath, "unsigned")
	appWorkflowUrlPath = resolveLocationWithId(appsPath, "workflow_url")
	appNamePath = resolveLocationWithId(appsPath, "name")
	appProfileIdPath = resolveLocationWithId(appsPath, "profile_id")
	appSignArgsPath = resolveLocationWithId(appsPath, "sign_args")

	profilesPath = filepath.Join(config.Current.SaveDir, "profiles")
	profileCertPath = resolveLocationWithId(profilesPath, "cert.p12")
	profilePassPath = resolveLocationWithId(profilesPath, "pass.txt")
	profileProvPath = resolveLocationWithId(profilesPath, "prov.mobileprovision")
	profileNamePath = resolveLocationWithId(profilesPath, "name.txt")

	requiredPaths := []string{appsPath, profilesPath}
	for _, path := range requiredPaths {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			log.Fatalln(errors.WithMessage(err, "mkdir required path"))
		}
	}
	if err := Apps.refresh(); err != nil {
		log.Fatalln(errors.WithMessage(err, "refresh apps"))
	}
	if err := Profiles.refresh(); err != nil {
		log.Fatalln(errors.WithMessage(err, "refresh profiles"))
	}
}

var resolveLocationWithId = func(parent string, path string) func(id string) string {
	return func(id string) string {
		return util.SafeJoin(parent, id, path)
	}
}
