package storage

import (
	"github.com/google/uuid"
	"io"
	"os"
	"sync"
)

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
		r.idToAppMap[id] = newApp(id)
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

func (r *appResolver) New(unsignedFile io.ReadSeeker, name string, profile Profile) (App, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	id := uuid.NewString()
	app := &app{id: id}
	if err := os.MkdirAll(appPath(id), 0666); err != nil {
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
	if err := app._prune(); err != nil {
		return &AppError{"prune app", ".", err}
	}
	delete(r.idToAppMap, app.GetId())
	return nil
}
