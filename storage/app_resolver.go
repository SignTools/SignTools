package storage

import (
	"github.com/google/uuid"
	"os"
	"sync"
)

type appResolver struct {
	idToAppMap map[string]App
	mutex      sync.Mutex
	scannedDir bool
}

func (r *appResolver) Refresh() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	idDirs, err := os.ReadDir(SaveAppsPath)
	if err != nil {
		return &AppError{"read apps dir", ".", err}
	}
	for _, idDir := range idDirs {
		id := idDir.Name()
		r.idToAppMap[id] = newApp(id)
	}
	return nil
}

func (r *appResolver) GetAll() ([]App, error) {
	if !r.scannedDir {
		if err := r.Refresh(); err != nil {
			return nil, &AppError{"refresh apps", ".", err}
		}
		r.scannedDir = true
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	var apps []App
	for _, app := range r.idToAppMap {
		apps = append(apps, app)
	}
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

func (r *appResolver) New() (App, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	id := uuid.NewString()
	if err := os.MkdirAll(SaveAppPath(id), 0666); err != nil {
		return nil, &AppError{"make app dir", id, err}
	}
	app := newApp(id)
	r.idToAppMap[id] = app
	return app, nil
}

func (r *appResolver) Delete(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	app, ok := r.idToAppMap[id]
	if !ok {
		return nil
	}
	if err := app.prune(); err != nil {
		return &AppError{"prune app", ".", err}
	}
	delete(r.idToAppMap, app.GetId())
	return nil
}
