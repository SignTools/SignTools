package storage

import (
	"SignTools/src/util"
	"github.com/pkg/errors"
	"io"
	"os"
	"sort"
	"sync"
)

func newAppResolver() *appResolver {
	return &appResolver{
		idToAppMap: map[string]App{},
	}
}

type appResolver struct {
	idToAppMap map[string]App
	mutex      sync.Mutex
}

func (r *appResolver) refresh() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	idDirs, err := os.ReadDir(appsPath)
	if err != nil {
		return errors.WithMessage(err, "read apps dir")
	}
	idDirs = util.RemoveHiddenDirs(idDirs)
	for _, idDir := range idDirs {
		id := idDir.Name()
		r.idToAppMap[id] = loadApp(id)
	}
	return nil
}

func (r *appResolver) GetAll() ([]App, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	var apps []App
	for _, app := range r.idToAppMap {
		apps = append(apps, app)
	}
	// reverse sort
	sort.Slice(apps, func(i, j int) bool {
		time1, _ := apps[i].GetModTime()
		time2, _ := apps[j].GetModTime()
		return time1.After(time2)
	})
	return apps, nil
}

func (r *appResolver) Get(id string) (App, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	app, ok := r.idToAppMap[id]
	if !ok {
		return nil, false
	}
	return app, true
}

func (r *appResolver) New(unsignedFile io.Reader, name string, profile Profile, signArgs string, userBundleId string, builderId string, tweakMap map[string]io.Reader) (App, error) {
	app, err := createApp(unsignedFile, name, profile, signArgs, userBundleId, builderId, tweakMap)
	if err != nil {
		return nil, err
	}
	r.mutex.Lock()
	r.idToAppMap[app.GetId()] = app
	r.mutex.Unlock()
	return app, nil
}

func (r *appResolver) Delete(id string) error {
	r.mutex.Lock()
	app, ok := r.idToAppMap[id]
	if !ok {
		r.mutex.Unlock()
		return nil
	}
	appId := app.GetId()
	delete(r.idToAppMap, appId)
	r.mutex.Unlock()
	if err := app.delete(); err != nil {
		return errors.WithMessagef(err, "delete app id=%s", appId)
	}
	return nil
}
