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
	SaveAppsPath        = filepath.Join(config.Current.SaveDir, "apps")
	SaveAppPath         = resolveLocationWithId(SaveAppsPath, "")
	SaveSignedPath      = resolveLocationWithId(SaveAppsPath, "signed")
	SaveUnsignedPath    = resolveLocationWithId(SaveAppsPath, "unsigned")
	SaveWorkflowUrlPath = resolveLocationWithId(SaveAppsPath, "workflow_url")
	SaveNamePath        = resolveLocationWithId(SaveAppsPath, "name")
)

func init() {
	if err := os.MkdirAll(SaveAppsPath, 0666); err != nil {
		log.Fatalln(errors.WithMessage(err, "mkdir SaveAppsPath"))
	}
}

var resolveLocationWithId = func(parent string, path string) func(id string) string {
	return func(id string) string {
		return util.SafeJoin(parent, id, path)
	}
}

var Apps = appResolver{idToAppMap: map[string]App{}}
