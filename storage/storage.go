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
	saveAppsPath        = filepath.Join(config.Current.SaveDir, "apps")
	saveAppPath         = resolveLocationWithId(saveAppsPath, "")
	saveSignedPath      = resolveLocationWithId(saveAppsPath, "signed")
	saveUnsignedPath    = resolveLocationWithId(saveAppsPath, "unsigned")
	saveWorkflowUrlPath = resolveLocationWithId(saveAppsPath, "workflow_url")
	saveNamePath        = resolveLocationWithId(saveAppsPath, "name")

	profilesPath     = filepath.Join(config.Current.SaveDir, "profiles")
	profilesCertPath = resolveLocationWithId(profilesPath, "cert.p12")
	profilesPassPath = resolveLocationWithId(profilesPath, "pass.txt")
	profilesProvPath = resolveLocationWithId(profilesPath, "prov.mobileprovision")
	profilesNamePath = resolveLocationWithId(profilesPath, "name.txt")
)

var Apps = appResolver{idToAppMap: map[string]App{}}
var Profiles = profileResolver{idToProfileMap: map[string]Profile{}}

func init() {
	requiredPaths := []string{saveAppsPath, profilesPath}
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
