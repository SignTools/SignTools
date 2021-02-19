package storage

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

type App interface {
	GetId() string
	ReadSigned(func(io.ReadSeeker) error) error
	WriteSigned(func(io.WriteSeeker) error) error
	IsSigned() (bool, error)
	ReadUnsigned(func(io.ReadSeeker) error) error
	WriteUnsigned(func(io.WriteSeeker) error) error
	GetName() (string, error)
	SetName(string) error
	GetWorkflowUrl() (string, error)
	SetWorkflowUrl(string) error
	GetModTime() (time.Time, error)
	prune() error
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

func (a *app) GetModTime() (time.Time, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	appDir, err := os.Stat(SaveAppPath(a.id))
	if err != nil {
		return time.Time{}, &AppError{"stat app dir", a.id, err}
	}
	return appDir.ModTime(), nil
}

func (a *app) IsSigned() (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if _, err := os.Stat(SaveSignedPath(a.id)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, &AppError{"stat signed file", a.id, err}
	}
	return true, nil
}

func (a *app) GetId() string {
	return a.id
}

func (a *app) ReadSigned(f func(io.ReadSeeker) error) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if err := a.readFile(f, SaveSignedPath(a.id)); err != nil {
		return err
	}
	return nil
}

func (a *app) WriteSigned(f func(io.WriteSeeker) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.writeFile(f, SaveSignedPath(a.id)); err != nil {
		return err
	}
	return nil
}

func (a *app) ReadUnsigned(f func(io.ReadSeeker) error) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if err := a.readFile(f, SaveUnsignedPath(a.id)); err != nil {
		return err
	}
	return nil
}

func (a *app) WriteUnsigned(f func(io.WriteSeeker) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.writeFile(f, SaveUnsignedPath(a.id)); err != nil {
		return err
	}
	return nil
}

func (a *app) GetName() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	b, err := ioutil.ReadFile(SaveNamePath(a.id))
	if err != nil {
		return "", &AppError{"read name file", a.id, err}
	}
	return string(b), nil
}

func (a *app) SetName(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ioutil.WriteFile(SaveNamePath(a.id), []byte(name), 0666); err != nil {
		return &AppError{"write name file", a.id, err}
	}
	return nil
}

func (a *app) GetWorkflowUrl() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	b, err := ioutil.ReadFile(SaveWorkflowUrlPath(a.id))
	if err != nil {
		return "", &AppError{"read workflow url file", a.id, err}
	}
	return string(b), nil
}

func (a *app) SetWorkflowUrl(url string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ioutil.WriteFile(SaveWorkflowUrlPath(a.id), []byte(url), 0666); err != nil {
		return &AppError{"set workflow url file", a.id, err}
	}
	return nil
}

// used by appResolver.Delete, must be synchronized
func (a *app) prune() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.RemoveAll(SaveAppPath(a.id)); err != nil {
		return &AppError{"remove app dir", a.id, err}
	}
	return nil
}

func (a *app) readFile(f func(io.ReadSeeker) error, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return &AppError{"open file", a.id, err}
	}
	defer file.Close()
	if err := f(file); err != nil {
		return &AppError{"user read function", a.id, err}
	}
	return nil
}

func (a *app) writeFile(f func(io.WriteSeeker) error, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return &AppError{"create file", a.id, err}
	}
	defer file.Close()
	if err := f(file); err != nil {
		return &AppError{"user write function", a.id, err}
	}
	return nil
}
