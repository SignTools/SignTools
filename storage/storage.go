package storage

import (
	"github.com/pkg/errors"
	"ios-signer-service/config"
	"ios-signer-service/util"
	"log"
	"os"
	"path/filepath"
)

var (
	appsPath           = filepath.Join(config.Current.SaveDir, "apps")
	appPath            = resolveLocationWithId(appsPath, "")
	appSignedPath      = resolveLocationWithId(appsPath, "signed")
	appUnsignedPath    = resolveLocationWithId(appsPath, "unsigned")
	appWorkflowUrlPath = resolveLocationWithId(appsPath, "workflow_url")
	appNamePath        = resolveLocationWithId(appsPath, "name")

	profilesPath    = filepath.Join(config.Current.SaveDir, "profiles")
	profileCertPath = resolveLocationWithId(profilesPath, "cert.p12")
	profilePassPath = resolveLocationWithId(profilesPath, "pass.txt")
	profileProvPath = resolveLocationWithId(profilesPath, "prov.mobileprovision")
	profileNamePath = resolveLocationWithId(profilesPath, "name.txt")
)

var Apps = appResolver{idToAppMap: map[string]App{}}
var Profiles = profileResolver{idToProfileMap: map[string]Profile{}}

func init() {
	requiredPaths := []string{appsPath, profilesPath}
	for _, path := range requiredPaths {
		if err := os.MkdirAll(path, 0666); err != nil {
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
