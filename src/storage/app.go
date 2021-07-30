package storage

import (
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"os"
	"sync"
	"time"
)

type App interface {
	GetId() string
	GetSigned() (ReadonlyFile, error)
	ClearSigned() error
	SetSigned(reader io.ReadSeeker, bundleId string) error
	IsSigned() (bool, error)
	GetUnsigned() (ReadonlyFile, error)
	GetName() (string, error)
	SetName(name string) error
	GetSignArgs() (string, error)
	GetBundleId() (string, error)
	GetUserBundleId() (string, error)
	GetWorkflowUrl() (string, error)
	SetWorkflowUrl(string) error
	GetModTime() (time.Time, error)
	ResetModTime() error
	GetProfileId() (string, error)
	GetBuilderId() (string, error)
}

func loadAppFromId(id string) App {
	return &app{id: id}
}

func newApp(unsignedFile io.ReadSeeker, name string, profile Profile, signArgs string, userBundleId string, builderId string) (App, error) {
	id := uuid.NewString()
	app := &app{id: id}

	if err := os.MkdirAll(appPath(id), os.ModePerm); err != nil {
		return nil, errors.New("make app dir")
	}
	if err := app.setName(name); err != nil {
		return nil, errors.New("set name")
	}
	if err := app.setProfileId(profile); err != nil {
		return nil, errors.New("set profile id")
	}
	if err := app.setUnsigned(unsignedFile); err != nil {
		return nil, errors.New("set unsigned")
	}
	if err := app.setSignArgs(signArgs); err != nil {
		return nil, errors.New("set sign args")
	}
	if err := app.setUserBundleId(userBundleId); err != nil {
		return nil, errors.New("set user bundle id")
	}
	if err := app.setBuilderId(builderId); err != nil {
		return nil, errors.New("set builder id")
	}
	return app, nil
}

type app struct {
	mu sync.RWMutex
	id string
}

func (a *app) GetSignArgs() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appSignArgsPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) GetBundleId() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appBundleIdPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) GetUserBundleId() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appUserBundleIdPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) GetProfileId() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appProfileIdPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) GetBuilderId() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appBuilderIdPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) GetModTime() (time.Time, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	appDir, err := os.Stat(appPath(a.id))
	if err != nil {
		return time.Time{}, err
	}
	return appDir.ModTime(), nil
}

func (a *app) IsSigned() (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.hasSignedFile()
}

func (a *app) GetId() string {
	return a.id
}

func (a *app) GetSigned() (ReadonlyFile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return os.Open(appSignedPath(a.id))
}

func (a *app) ClearSigned() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return os.Remove(appSignedPath(a.id))
}

func (a *app) SetSigned(reader io.ReadSeeker, bundleId string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	exists, err := a.hasSignedFile()
	if err != nil {
		return errors.WithMessage(err, "check already exists")
	}
	if exists {
		return errors.New("already exists")
	}
	file, err := os.Create(appSignedPath(a.id))
	if err != nil {
		return errors.New("create")
	}
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		return errors.New("write")
	}
	if err := a.setBundleId(bundleId); err != nil {
		return errors.WithMessage(err, "set bundle id")
	}
	return nil
}

func (a *app) GetUnsigned() (ReadonlyFile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return os.Open(appUnsignedPath(a.id))
}

func (a *app) GetName() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appNamePath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) SetName(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.setName(name)
}

func (a *app) GetWorkflowUrl() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appWorkflowUrlPath(a.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (a *app) SetWorkflowUrl(url string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := writeTrimSpace(appWorkflowUrlPath(a.id), url); err != nil {
		return err
	}
	return nil
}

func (a *app) ResetModTime() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if err := os.Chtimes(appPath(a.id), now, now); err != nil {
		return err
	}
	return nil
}

func (a *app) hasSignedFile() (bool, error) {
	if _, err := os.Stat(appSignedPath(a.id)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (a *app) setUnsigned(file io.ReadSeeker) error {
	dstFile, err := os.Create(appUnsignedPath(a.id))
	if err != nil {
		return errors.WithMessage(err, "create file")
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return errors.WithMessage(err, "write file")
	}
	return nil
}

func (a *app) setProfileId(profile Profile) error {
	if err := writeTrimSpace(appProfileIdPath(a.id), profile.GetId()); err != nil {
		return err
	}
	return nil
}

func (a *app) setName(name string) error {
	if err := writeTrimSpace(appNamePath(a.id), name); err != nil {
		return err
	}
	return nil
}

func (a *app) setSignArgs(args string) error {
	if err := writeTrimSpace(appSignArgsPath(a.id), args); err != nil {
		return err
	}
	return nil
}

func (a *app) setBundleId(id string) error {
	if err := writeTrimSpace(appBundleIdPath(a.id), id); err != nil {
		return err
	}
	return nil
}

func (a *app) setUserBundleId(id string) error {
	if err := writeTrimSpace(appUserBundleIdPath(a.id), id); err != nil {
		return err
	}
	return nil
}

func (a *app) setBuilderId(id string) error {
	if err := writeTrimSpace(appBuilderIdPath(a.id), id); err != nil {
		return err
	}
	return nil
}
