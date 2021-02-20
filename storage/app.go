package storage

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

type App interface {
	GetId() string
	GetSigned() (io.ReadSeekCloser, error)
	SetSigned(io.ReadSeeker) error
	IsSigned() (bool, error)
	GetUnsigned() (io.ReadSeekCloser, error)
	GetName() (string, error)
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

func newApp(id string) App {
	return &app{id: id}
}

type app struct {
	mu sync.RWMutex
	id string
}

func (a *app) GetProfileId() (string, error) {
	a.mu.RLock()
	a.mu.RUnlock()
	b, err := ioutil.ReadFile(appProfileIdPath(a.id))
	if err != nil {
		return "", &AppError{"read profile id file", a.id, err}
	}
	return string(b), nil
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

func (a *app) GetSigned() (io.ReadSeekCloser, error) {
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

func (a *app) GetUnsigned() (io.ReadSeekCloser, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return os.Open(appUnsignedPath(a.id))
}

func (a *app) GetName() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	b, err := ioutil.ReadFile(appNamePath(a.id))
	if err != nil {
		return "", &AppError{"read name file", a.id, err}
	}
	return string(b), nil
}

func (a *app) GetWorkflowUrl() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	b, err := ioutil.ReadFile(appWorkflowUrlPath(a.id))
	if err != nil {
		return "", &AppError{"read workflow url file", a.id, err}
	}
	return string(b), nil
}

func (a *app) SetWorkflowUrl(url string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ioutil.WriteFile(appWorkflowUrlPath(a.id), []byte(url), 0666); err != nil {
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
