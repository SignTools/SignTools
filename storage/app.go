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
	GetSigned(io.Writer) (int64, error)
	SetSigned(io.Reader) (int64, error)
	IsSigned() (bool, error)
	GetUnsigned(io.Writer) (int64, error)
	SetUnsigned(io.Reader) (int64, error)
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

func (a *app) GetSigned(writer io.Writer) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	i, err := a.readFile(writer, SaveSignedPath(a.id))
	if err != nil {
		return 0, &AppError{"read signed file", a.id, err}
	}
	return i, nil
}

func (a *app) SetSigned(reader io.Reader) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	i, err := a.writeFile(reader, SaveSignedPath(a.id))
	if err != nil {
		return 0, &AppError{"write signed file", a.id, err}
	}
	return i, nil
}

func (a *app) GetUnsigned(writer io.Writer) (int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	i, err := a.readFile(writer, SaveUnsignedPath(a.id))
	if err != nil {
		return 0, &AppError{"read unsigned file", a.id, err}
	}
	return i, nil
}

func (a *app) SetUnsigned(reader io.Reader) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	i, err := a.writeFile(reader, SaveUnsignedPath(a.id))
	if err != nil {
		return 0, &AppError{"set unsigned file", a.id, err}
	}
	return i, nil
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

func (a *app) prune() error {
	if err := os.RemoveAll(SaveAppPath(a.id)); err != nil {
		return &AppError{"remove app dir", a.id, err}
	}
	return nil
}

func (a *app) readFile(writer io.Writer, path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if n, err := io.Copy(writer, file); err != nil {
		return 0, err
	} else {
		return n, nil
	}
}

func (a *app) writeFile(reader io.Reader, path string) (int64, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if n, err := io.Copy(file, reader); err != nil {
		return 0, err
	} else {
		return n, nil
	}
}
