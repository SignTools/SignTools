package storage

import (
	"fmt"
	"os"
)

type Profile interface {
	GetId() string
	GetFiles() ([]fileGetter, error)
	GetName() (string, error)
	IsAccount() (bool, error)
}

func newProfile(id string) *profile {
	return &profile{id}
}

type profile struct {
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

func (p *profile) GetId() string {
	return p.id
}

func (p *profile) IsAccount() (bool, error) {
	if _, err := os.Stat(profileAccountNamePath(p.id)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (p *profile) GetFiles() ([]fileGetter, error) {
	isAccount, err := p.IsAccount()
	if err != nil {
		return nil, &ProfileError{"is account", p.id, err}
	}
	var files = []fileGetter{
		{name: "cert.p12", f1: p.getCert},
		{name: "cert_pass.txt", f2: p.getCertPass},
	}
	if isAccount {
		files = append(files, []fileGetter{
			{name: "account_name.txt", f2: p.getAccountName},
			{name: "account_pass.txt", f2: p.getAccountPass},
		}...)
	} else {
		files = append(files, []fileGetter{
			{name: "prov.mobileprovision", f1: p.getProv},
		}...)
	}
	return files, nil
}

func (p *profile) getCert() (ReadonlyFile, error) {
	file, err := os.Open(profileCertPath(p.id))
	if err != nil {
		return nil, &ProfileError{"open ProfilesCertPath", p.id, err}
	}
	return file, nil
}

func (p *profile) getProv() (ReadonlyFile, error) {
	file, err := os.Open(profileProvPath(p.id))
	if err != nil {
		return nil, &ProfileError{"open ProfilesProvPath", p.id, err}
	}
	return file, nil
}

func (p *profile) getAccountName() (string, error) {
	data, err := readTrimSpace(profileAccountNamePath(p.id))
	if err != nil {
		return "", &ProfileError{"read file profileAccountNamePath", p.id, err}
	}
	return data, nil
}

func (p *profile) getAccountPass() (string, error) {
	data, err := readTrimSpace(profileAccountPassPath(p.id))
	if err != nil {
		return "", &ProfileError{"read file profileAccountPassPath", p.id, err}
	}
	return data, nil
}

func (p *profile) getCertPass() (string, error) {
	data, err := readTrimSpace(profileCertPassPath(p.id))
	if err != nil {
		return "", &ProfileError{"read file profileCertPassPath", p.id, err}
	}
	return data, nil
}

func (p *profile) GetName() (string, error) {
	data, err := readTrimSpace(profileNamePath(p.id))
	if err != nil {
		return "", &ProfileError{"read file ProfilesNamePath", p.id, err}
	}
	return data, nil
}
