package storage

import (
	"SignTools/src/util"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	TweakRoot = FSName("")
	TweakName = FSName("name")
	TweakFile = FSName("file")
)

// Tweak is a reusable tweak file stored in the tweak library.
type Tweak interface {
	GetId() string
	GetModTime() (time.Time, error)
	delete() error
	FileSystem
}

func loadTweak(id string) Tweak {
	return newTweak(id)
}

func createTweak(file io.Reader, name string) (Tweak, error) {
	name, err := SanitizeTweakName(name)
	if err != nil {
		return nil, err
	}
	t := newTweak(uuid.NewString())
	if err := os.MkdirAll(t.resolvePath(TweakRoot), os.ModePerm); err != nil {
		return nil, errors.WithMessage(err, "make tweak dir")
	}
	if err := t.SetFile(TweakFile, file); err != nil {
		_ = t.delete()
		return nil, errors.WithMessagef(err, "set %s", TweakFile)
	}
	if err := t.SetString(TweakName, name); err != nil {
		_ = t.delete()
		return nil, errors.WithMessagef(err, "set %s", TweakName)
	}
	return t, nil
}

// SanitizeTweakName strips any directory components so the name is a plain filename.
func SanitizeTweakName(name string) (string, error) {
	name = filepath.Base(filepath.Clean("/" + strings.TrimSpace(name)))
	if name == "" || name == "/" || name == "." {
		return "", errors.New("invalid tweak name")
	}
	return name, nil
}

func newTweak(id string) *tweak {
	return &tweak{id: id, FileSystemBase: FileSystemBase{resolvePath: func(name FSName) string {
		return util.SafeJoinFilePaths(tweaksPath, id, string(name))
	}}}
}

type tweak struct {
	mu sync.RWMutex
	id string
	FileSystemBase
}

func (t *tweak) GetId() string {
	return t.id
}

func (t *tweak) GetModTime() (time.Time, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stat, err := os.Stat(t.resolvePath(TweakRoot))
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}

func (t *tweak) delete() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return os.RemoveAll(t.resolvePath(TweakRoot))
}
