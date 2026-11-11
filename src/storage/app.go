package storage

import (
	"SignTools/src/util"
	"bytes"
	"github.com/google/uuid"
	"github.com/natefinch/atomic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"os"
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
	AppIcon         = FSName("icon")
	TweaksDir       = FSName("tweaks")
)

type App interface {
	GetId() string
	SaveIcon() error
	IsSigned() (bool, error)
	GetModTime() (time.Time, error)
	ResetModTime() error
	delete() error
	FileSystem
}

func loadApp(id string) App {
	a := newApp(id)
	a.watch()
	return a
}

func createApp(unsignedFile io.Reader, name string, profile Profile, signArgs string, userBundleId string, builderId string, tweakMap map[string]io.Reader) (*app, error) {
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
	if err := app.SaveIcon(); err != nil {
		log.Warn().Err(err).Str("app_id", app.id).Msg("extract icon")
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
	id string
	FileSystemBase
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
	now := time.Now()
	err := os.Chtimes(a.resolvePath(AppRoot), now, now)
	a.mu.Unlock()
	return a.changed(err)
}

// SaveIcon extracts the icon from the unsigned app. On failure it saves an empty file, so the
// extraction isn't retried. It doesn't announce a change, since nothing else on the card depends on it.
func (a *app) SaveIcon() error {
	icon, extractErr := a.extractIcon()
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := atomic.WriteFile(a.resolvePath(AppIcon), bytes.NewReader(icon)); err != nil {
		return errors.WithMessagef(err, "set %s", AppIcon)
	}
	return extractErr
}

func (a *app) extractIcon() ([]byte, error) {
	file, err := a.GetFile(AppUnsignedFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return util.ExtractIcon(file, stat.Size())
}
