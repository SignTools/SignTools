package storage

import (
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
		return &AppError{"read apps dir", ".", err}
	}
	for _, idDir := range idDirs {
		id := idDir.Name()
		r.idToAppMap[id] = loadAppFromId(id)
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

func (r *appResolver) New(unsignedFile io.ReadSeeker, name string, profile Profile, signArgs string) (App, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	app, err := newApp(unsignedFile, name, profile, signArgs)
	if err != nil {
		return nil, &AppError{"new app", ".", err}
	}

	r.idToAppMap[app.GetId()] = app
	return app, nil
}

func (r *appResolver) Delete(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	app, ok := r.idToAppMap[id]
	if !ok {
		return nil
	}
	if err := app._prune(); err != nil {
		return &AppError{"prune app", ".", err}
	}
	delete(r.idToAppMap, app.GetId())
	return nil
}
