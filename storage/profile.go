package storage

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func newProfile(id string) *Profile {
	return &Profile{id}
}

type Profile struct {
	id string
}

type ProfileError struct {
	Message string
	Id      string
	Err     error
}

func (e *ProfileError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Message, e.Id, e.Err)
}

func (p *Profile) GetId() string {
	return p.id
}

func (p *Profile) GetCert() (io.ReadSeekCloser, error) {
	file, err := os.Open(profilesCertPath(p.id))
	if err != nil {
		return nil, &ProfileError{"open ProfilesCertPath", p.id, err}
	}
	return file, nil
}

func (p *Profile) GetProv() (io.ReadSeekCloser, error) {
	file, err := os.Open(profilesProvPath(p.id))
	if err != nil {
		return nil, &ProfileError{"open ProfilesProvPath", p.id, err}
	}
	return file, nil
}

func (p *Profile) GetPassword() (io.ReadSeekCloser, error) {
	file, err := os.Open(profilesPassPath(p.id))
	if err != nil {
		return nil, &ProfileError{"open ProfilesPassPath", p.id, err}
	}
	return file, nil
}

func (p *Profile) GetName() (string, error) {
	bytes, err := ioutil.ReadFile(profilesNamePath(p.id))
	if err != nil {
		return "", &ProfileError{"read file ProfilesNamePath", p.id, err}
	}
	return string(bytes), nil
}
