package storage

import (
	"SignTools/src/util"
	"github.com/pkg/errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

func newTweakResolver() *tweakResolver {
	return &tweakResolver{idToTweakMap: map[string]Tweak{}}
}

type tweakResolver struct {
	idToTweakMap map[string]Tweak
	mutex        sync.Mutex
}

func (r *tweakResolver) refresh() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	idDirs, err := os.ReadDir(tweaksPath)
	if err != nil {
		return errors.WithMessage(err, "read tweaks dir")
	}
	idDirs = util.RemoveHiddenDirs(idDirs)
	for _, idDir := range idDirs {
		if !idDir.IsDir() {
			continue
		}
		id := idDir.Name()
		r.idToTweakMap[id] = loadTweak(id)
	}
	return nil
}

// GetAll returns all library tweaks sorted by name.
func (r *tweakResolver) GetAll() ([]Tweak, error) {
	r.mutex.Lock()
	var tweaks []Tweak
	for _, t := range r.idToTweakMap {
		tweaks = append(tweaks, t)
	}
	r.mutex.Unlock()
	names := make(map[string]string, len(tweaks))
	for _, t := range tweaks {
		name, _ := t.GetString(TweakName)
		names[t.GetId()] = strings.ToLower(name)
	}
	sort.Slice(tweaks, func(i, j int) bool {
		return names[tweaks[i].GetId()] < names[tweaks[j].GetId()]
	})
	return tweaks, nil
}

func (r *tweakResolver) Get(id string) (Tweak, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	t, ok := r.idToTweakMap[id]
	return t, ok
}

func (r *tweakResolver) New(file io.Reader, name string) (Tweak, error) {
	t, err := createTweak(file, name)
	if err != nil {
		return nil, err
	}
	r.mutex.Lock()
	r.idToTweakMap[t.GetId()] = t
	r.mutex.Unlock()
	return t, nil
}

func (r *tweakResolver) Delete(id string) error {
	r.mutex.Lock()
	t, ok := r.idToTweakMap[id]
	if !ok {
		r.mutex.Unlock()
		return nil
	}
	delete(r.idToTweakMap, id)
	r.mutex.Unlock()
	if err := t.delete(); err != nil {
		return errors.WithMessagef(err, "delete tweak id=%s", id)
	}
	return nil
}
