package storage

import (
	"fmt"
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
	SetSigned(io.ReadSeeker) error
	IsSigned() (bool, error)
	GetUnsigned() (ReadonlyFile, error)
	GetName() (string, error)
	GetSignArgs() (string, error)
	GetWorkflowUrl() (string, error)
	SetWorkflowUrl(string) error
	GetModTime() (time.Time, error)
	GetProfileId() (string, error)
	_prune() error
}

type AppError struct {
	Message string
	Id      string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Message, e.Id, e.Err)
}

func loadAppFromId(id string) App {
	return &app{id: id}
}

func newApp(unsignedFile io.ReadSeeker, name string, profile Profile, signArgs string) (App, error) {
	id := uuid.NewString()
	app := &app{id: id}

	if err := os.MkdirAll(appPath(id), os.ModePerm); err != nil {
		return nil, &AppError{"make app dir", id, err}
	}
	if err := app.setName(name); err != nil {
		return nil, &AppError{"set name", id, err}
	}
	if err := app.setProfileId(profile); err != nil {
		return nil, &AppError{"set profile id", id, err}
	}
	if err := app.setUnsigned(unsignedFile); err != nil {
		return nil, &AppError{"set unsigned", id, err}
	}
	if err := app.setSignArgs(signArgs); err != nil {
		return nil, &AppError{"set unsigned", id, err}
	}
	return app, nil
}

type app struct {
	mu sync.RWMutex
	id string
}

func (a *app) GetSignArgs() (string, error) {
	a.mu.RLock()
	a.mu.RUnlock()
	data, err := readTrimSpace(appSignArgsPath(a.id))
	if err != nil {
		return "", &AppError{"read sign args file", a.id, err}
	}
	return data, nil
}

func (a *app) GetProfileId() (string, error) {
	a.mu.RLock()
	a.mu.RUnlock()
	data, err := readTrimSpace(appProfileIdPath(a.id))
	if err != nil {
		return "", &AppError{"read profile id file", a.id, err}
	}
	return data, nil
}

func (a *app) GetModTime() (time.Time, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	appDir, err := os.Stat(appPath(a.id))
	if err != nil {
		return time.Time{}, &AppError{"stat app dir", a.id, err}
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

func (a *app) SetSigned(seeker io.ReadSeeker) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	exists, err := a.hasSignedFile()
	if err != nil {
		return &AppError{"already exists", a.id, err}
	}
	if exists {
		return &AppError{"true", a.id, errors.New("already exists")}
	}
	file, err := os.Create(appSignedPath(a.id))
	if err != nil {
		return &AppError{"create", a.id, err}
	}
	defer file.Close()
	if _, err := io.Copy(file, seeker); err != nil {
		return &AppError{"write", a.id, err}
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
		return "", &AppError{"read name file", a.id, err}
	}
	return data, nil
}

func (a *app) GetWorkflowUrl() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := readTrimSpace(appWorkflowUrlPath(a.id))
	if err != nil {
		return "", &AppError{"read workflow url file", a.id, err}
	}
	return data, nil
}

func (a *app) SetWorkflowUrl(url string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := writeTrimSpace(appWorkflowUrlPath(a.id), url); err != nil {
		return &AppError{"set workflow url file", a.id, err}
	}
	return nil
}

// used by appResolver.Delete, must be synchronized
func (a *app) _prune() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.RemoveAll(appPath(a.id)); err != nil {
		return &AppError{"remove app dir", a.id, err}
	}
	return nil
}

func (a *app) hasSignedFile() (bool, error) {
	if _, err := os.Stat(appSignedPath(a.id)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, &AppError{"stat signed file", a.id, err}
	}
	return true, nil
}

func (a *app) setUnsigned(file io.ReadSeeker) error {
	dstFile, err := os.Create(appUnsignedPath(a.id))
	if err != nil {
		return &AppError{"create unsigned", a.id, err}
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return &AppError{"write unsigned", a.id, err}
	}
	return nil
}

func (a *app) setProfileId(profile Profile) error {
	if err := writeTrimSpace(appProfileIdPath(a.id), profile.GetId()); err != nil {
		return &AppError{"write profile id file", a.id, err}
	}
	return nil
}

func (a *app) setName(name string) error {
	if err := writeTrimSpace(appNamePath(a.id), name); err != nil {
		return &AppError{"write name file", a.id, err}
	}
	return nil
}

func (a *app) setSignArgs(args string) error {
	if err := writeTrimSpace(appSignArgsPath(a.id), args); err != nil {
		return &AppError{"write sign args file", a.id, err}
	}
	return nil
}
