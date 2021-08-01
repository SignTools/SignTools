package storage

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"ios-signer-service/src/config"
	"ios-signer-service/src/util"
	"os"
	"sort"
)

func newProfileResolver() *profileResolver {
	return &profileResolver{
		idToProfileMap: map[string]Profile{},
	}
}

type profileResolver struct {
	idToProfileMap map[string]Profile
}

func (r *profileResolver) refresh() error {
	idDirs, err := util.ReadDirNonHidden(profilesPath)
	if err != nil {
		return errors.WithMessage(err, "read profiles dir")
	}
	envProfile, err := newEnvProfile(config.Current.EnvProfile)
	if err == nil {
		r.idToProfileMap[envProfile.GetId()] = envProfile
	} else if !os.IsNotExist(err) {
		return errors.WithMessage(err, "import profile from envvars")
	}
	for _, idDir := range idDirs {
		id := idDir.Name()
		profile, err := newProfile(id)
		if err != nil {
			log.Err(err).Str("id", id).Msg("load profile from files")
			continue
		}
		r.idToProfileMap[id] = profile
	}
	return nil
}

func (r *profileResolver) GetAll() ([]Profile, error) {
	var profiles []Profile
	for _, profile := range r.idToProfileMap {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		name1, _ := profiles[i].GetName()
		name2, _ := profiles[j].GetName()
		return name1 < name2
	})
	return profiles, nil
}

func (r *profileResolver) GetById(id string) (Profile, bool) {
	profile, ok := r.idToProfileMap[id]
	if !ok {
		return nil, false
	}
	return profile, true
}
