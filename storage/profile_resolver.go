package storage

import (
	"os"
)

func newProfileResolver() *profileResolver {
	return &profileResolver{
		idToProfileMap:   map[string]Profile{},
		nameToProfileMap: map[string]Profile{},
	}
}

type profileResolver struct {
	idToProfileMap   map[string]Profile
	nameToProfileMap map[string]Profile
}

func (r *profileResolver) refresh() error {
	idDirs, err := os.ReadDir(profilesPath)
	if err != nil {
		return &AppError{"read profiles dir", ".", err}
	}
	for _, idDir := range idDirs {
		id := idDir.Name()
		profile := newProfile(id)
		name, err := profile.GetName()
		if err != nil {
			return &AppError{"get profile name", id, err}
		}
		r.idToProfileMap[id] = profile
		r.nameToProfileMap[name] = profile
	}
	return nil
}

func (r *profileResolver) GetAll() ([]Profile, error) {
	var profiles []Profile
	for _, profile := range r.idToProfileMap {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (r *profileResolver) GetById(id string) (Profile, bool) {
	profile, ok := r.idToProfileMap[id]
	if !ok {
		return nil, false
	}
	return profile, true
}

func (r *profileResolver) GetByName(name string) (Profile, bool) {
	profile, ok := r.nameToProfileMap[name]
	if !ok {
		return nil, false
	}
	return profile, true
}
