package storage

import (
	"os"
)

type profileResolver struct {
	idToProfileMap map[string]*Profile
	scannedDir     bool
}

func (r *profileResolver) Refresh() error {
	idDirs, err := os.ReadDir(profilesPath)
	if err != nil {
		return &AppError{"read profiles dir", ".", err}
	}
	for _, idDir := range idDirs {
		id := idDir.Name()
		r.idToProfileMap[id] = newProfile(id)
	}
	return nil
}

func (r *profileResolver) GetAll() ([]*Profile, error) {
	if !r.scannedDir {
		if err := r.Refresh(); err != nil {
			return nil, &AppError{"refresh profiles", ".", err}
		}
		r.scannedDir = true
	}
	var profiles []*Profile
	for _, profile := range r.idToProfileMap {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (r *profileResolver) Get(id string) (*Profile, bool) {
	profile, ok := r.idToProfileMap[id]
	if !ok {
		return nil, false
	}
	return profile, true
}
