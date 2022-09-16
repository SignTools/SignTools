package storage

import (
	"SignTools/src/util"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"os"
	"sync"
	"time"
)

const (
	AppRoot         = FSName("")
	AppSignArgs     = FSName("sign_args")
	AppBundleId     = FSName("bundle_id")
	AppSignedFile   = FSName("signed")
	AppUnsignedFile = FSName("unsigned")
	AppName         = FSName("name")
	AppUserBundleId = FSName("user_bundle_id")
	AppWorkflowUrl  = FSName("workflow_url")
	AppProfileId    = FSName("profile_id")
	AppBuilderId    = FSName("builder_id")
	AppBundleName   = FSName("bundle_name")
	TweaksDir       = FSName("tweaks")
)

type App interface {
	GetId() string
	IsSigned() (bool, error)
	GetModTime() (time.Time, error)
	ResetModTime() error
	delete() error
	FileSystem
}

func loadApp(id string) App {
	return newApp(id)
}

func createApp(unsignedFile io.Reader, name string, profile Profile, signArgs string, userBundleId string, builderId string, tweakMap map[string]io.Reader) (App, error) {
	app := newApp(uuid.NewString())
	if err := os.MkdirAll(app.resolvePath(AppRoot), os.ModePerm); err != nil {
		return nil, errors.New("make app dir")
	}
	pairs := map[FSName]string{
		AppName:         name,
		AppSignArgs:     signArgs,
		AppUserBundleId: userBundleId,
		AppBuilderId:    builderId,
		AppProfileId:    profile.GetId(),
	}
	for fileType, value := range pairs {
		if err := app.SetString(fileType, value); err != nil {
			return nil, errors.WithMessagef(err, "set %s", fileType)
		}
	}
	if err := app.SetFile(AppUnsignedFile, unsignedFile); err != nil {
		return nil, errors.WithMessagef(err, "set %s", AppUnsignedFile)
	}
	if len(tweakMap) > 0 {
		if err := app.MkDir(TweaksDir); err != nil {
			return nil, err
		}
	}
	for name, tweak := range tweakMap {
		tweakPath := FSName(util.SafeJoinFilePaths(string(TweaksDir), name))
		if err := app.SetFile(tweakPath, tweak); err != nil {
			return nil, errors.WithMessagef(err, "set %s", tweakPath)
		}
	}
	return app, nil
}

func newApp(id string) *app {
	return &app{id: id, FileSystemBase: FileSystemBase{resolvePath: func(name FSName) string {
		return util.SafeJoinFilePaths(appsPath, id, string(name))
	}}}
}

type app struct {
	mu sync.RWMutex
	id string
	FileSystemBase
}

func (a *app) delete() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return os.RemoveAll(a.resolvePath(AppRoot))
}

func (a *app) GetModTime() (time.Time, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	appDir, err := os.Stat(a.resolvePath(AppRoot))
	if err != nil {
		return time.Time{}, err
	}
	return appDir.ModTime(), nil
}

func (a *app) IsSigned() (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if _, err := os.Stat(a.resolvePath(AppSignedFile)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (a *app) GetId() string {
	return a.id
}

func (a *app) ResetModTime() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if err := os.Chtimes(a.resolvePath(AppRoot), now, now); err != nil {
		return err
	}
	return nil
}
