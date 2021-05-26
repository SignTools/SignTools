package storage

import (
	"crypto/x509"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/pkcs12"
	"io/ioutil"
	"os"
)

type Profile interface {
	GetId() string
	GetFiles() ([]fileGetter, error)
	GetName() (string, error)
	IsAccount() (bool, error)
}

func newProfile(id string) (*profile, error) {
	p := profile{id: id}
	pass, err := p.getCertPass()
	if err != nil {
		return nil, err
	}
	p12, err := p.getCert()
	if err != nil {
		return nil, err
	}
	p12Bytes, err := ioutil.ReadAll(p12)
	if err != nil {
		return nil, err
	}
	teamId, err := validateCertAndGetTeamId(p12Bytes, pass)
	if err != nil {
		return nil, errors.WithMessage(err, "validate certificate")
	}
	p.teamId = teamId
	return &p, nil
}

func validateCertAndGetTeamId(p12Bytes []byte, pass string) (string, error) {
	blocks, err := pkcs12.ToPEM(p12Bytes, pass)
	if err != nil {
		return "", err
	}
	keys := 0
	appleCerts := 0
	teamId := ""
	for _, block := range blocks {
		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return "", err
			}
			if cert.IsCA {
				appleCerts++
			} else {
				if len(cert.Subject.OrganizationalUnit) != 1 {
					log.Error().Str("serial number", cert.SerialNumber.String()).
						Msg("certificate has invalid organization unit")
					continue
				}
				teamId = cert.Subject.OrganizationalUnit[0]
			}
		case "PRIVATE KEY":
			keys++
		}
	}
	if keys < 1 {
		return "", errors.New("no private keys found in p12 file")
	}
	if teamId == "" {
		return "", errors.New("no signing certificates found in p12 file")
	}
	if appleCerts < 1 {
		return "", errors.New("no Apple intermediary certificates found in p12 file")
	}
	return teamId, nil
}

type profile struct {
	id     string
	teamId string
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
		{name: "team_id.txt", f2: p.getTeamId},
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

func (p *profile) getTeamId() (string, error) {
	return p.teamId, nil
}

func (p *profile) GetName() (string, error) {
	data, err := readTrimSpace(profileNamePath(p.id))
	if err != nil {
		return "", &ProfileError{"read file ProfilesNamePath", p.id, err}
	}
	return data, nil
}
