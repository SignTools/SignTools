package storage

import (
	"SignTools/src/util"
	"bytes"
	"github.com/natefinch/atomic"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FSName string

type FileSystem interface {
	GetString(FSName) (string, error)
	GetFile(FSName) (ReadonlyFile, error)
	SetString(FSName, string) error
	SetFile(FSName, io.Reader) error
	RemoveFile(FSName) error
	Stat(name FSName) (os.FileInfo, error)
	MkDir(name FSName) error
	ReadDir(name FSName) ([]os.DirEntry, error)
}

type FileSystemBase struct {
	mu          sync.RWMutex
	resolvePath func(FSName) string
	onChange    func()
}

// watch is called once a resolver tracks the entity, so creating one doesn't announce a
// half-written state.
func (a *FileSystemBase) watch() {
	a.onChange = Changes.notify
}

func (a *FileSystemBase) GetModTime() (time.Time, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	stat, err := os.Stat(a.resolvePath(""))
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

func (a *FileSystemBase) delete() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return os.RemoveAll(a.resolvePath(""))
}

func (a *FileSystemBase) changed(err error) error {
	if err == nil && a.onChange != nil {
		a.onChange()
	}
	return err
}

func (a *FileSystemBase) GetString(name FSName) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, err := ioutil.ReadFile(a.resolvePath(name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (a *FileSystemBase) SetString(name FSName, value string) error {
	a.mu.Lock()
	reader := bytes.NewReader([]byte(strings.TrimSpace(value)))
	err := atomic.WriteFile(a.resolvePath(name), reader)
	a.mu.Unlock()
	return a.changed(err)
}

func (a *FileSystemBase) GetFile(name FSName) (ReadonlyFile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return os.Open(a.resolvePath(name))
}

func (a *FileSystemBase) SetFile(name FSName, value io.Reader) error {
	dir, file := filepath.Split(a.resolvePath(name))
	if dir == "" {
		dir = "."
	}
	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		return errors.WithMessage(err, "create temp file")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := io.Copy(f, value); err != nil {
		return errors.WithMessage(err, "save file")
	}
	if err := f.Sync(); err != nil {
		return errors.WithMessage(err, "sync changes")
	}
	if err := f.Close(); err != nil {
		return errors.WithMessage(err, "close file")
	}
	a.mu.Lock()
	err = atomic.ReplaceFile(f.Name(), a.resolvePath(name))
	a.mu.Unlock()
	if err != nil {
		return errors.WithMessage(err, "replace file")
	}
	return a.changed(nil)
}

func (a *FileSystemBase) RemoveFile(name FSName) error {
	a.mu.Lock()
	err := os.Remove(a.resolvePath(name))
	a.mu.Unlock()
	return a.changed(err)
}

func (a *FileSystemBase) Stat(name FSName) (os.FileInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return os.Stat(a.resolvePath(name))
}

func (a *FileSystemBase) MkDir(name FSName) error {
	a.mu.Lock()
	err := os.MkdirAll(a.resolvePath(name), 0700)
	a.mu.Unlock()
	return a.changed(err)
}

func (a *FileSystemBase) ReadDir(name FSName) ([]os.DirEntry, error) {
	dirs, err := os.ReadDir(a.resolvePath(name))
	if err != nil {
		return nil, err
	}
	return util.RemoveHiddenDirs(dirs), nil
}
