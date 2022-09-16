package storage

import (
	"SignTools/src/config"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"os"
	"reflect"
)

type MissingData struct {
	string
}

func (e *MissingData) Error() string {
	return "missing " + e.string
}

// Attempts to parse a Profile from environment variables.
// Returns os.ErrNotExist if variables are missing, otherwise any other error.
func newEnvProfile(cfg *config.EnvProfile) (*envProfile, error) {
	p, err := parseEnvProfile(cfg)
	if err != nil {
		return nil, err
	}
	fixedCert, teamId, err := processP12(p.originalCert, p.certPass)
	if err != nil {
		return nil, errors.WithMessage(err, "validate certificate")
	}
	p.fixedCert = fixedCert
	p.teamId = teamId
	return p, nil
}

func parseEnvProfile(cfg *config.EnvProfile) (*envProfile, error) {
	// cfg is empty, no envvars were set
	if reflect.DeepEqual(cfg, &config.EnvProfile{}) {
		return nil, os.ErrNotExist
	}
	requiredMap := map[string]string{
		cfg.Name:       "name",
		cfg.CertBase64: "certificate",
		cfg.CertPass:   "certificate password"}
	for k, v := range requiredMap {
		if k == "" {
			return nil, &MissingData{v}
		}
	}
	if cfg.ProvBase64 != "" {
		log.Info().Msg("importing prov profile from envvars")
		certBytes, err := decodeVar(cfg.CertBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode cert base64")
		}
		provBytes, err := decodeVar(cfg.ProvBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode prov base64")
		}
		return newEnvProfileProv(uuid.NewString(), cfg, certBytes, provBytes), nil
	} else if cfg.AccountName != "" && cfg.AccountPass != "" {
		log.Info().Msg("importing account profile from envvars")
		certBytes, err := decodeVar(cfg.CertBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode cert base64")
		}
		return newEnvProfileAccount(uuid.NewString(), cfg, certBytes), nil
	} else {
		return nil, &MissingData{"provisioning profile or account name and password"}
	}
}

func decodeVar(dataStr string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, err
	}
	zlibReader, err := zlib.NewReader(bytes.NewBuffer(data))
	if errors.Is(err, zlib.ErrHeader) {
		return data, nil
	} else if err != nil {
		return nil, err
	}
	dataRaw, err := ioutil.ReadAll(zlibReader)
	if err != nil {
		return nil, err
	}
	return dataRaw, nil
}

type envProfile struct {
	id           string
	name         string
	prov         []byte
	certPass     string
	originalCert []byte
	fixedCert    []byte
	accountName  string
	accountPass  string
	teamId       string
}

func (p *envProfile) MkDir(name FSName) error {
	return errors.New("unsupported operation")
}

func (p *envProfile) ReadDir(name FSName) ([]os.DirEntry, error) {
	return nil, errors.New("unsupported operation")
}

func (p *envProfile) Stat(name FSName) (os.FileInfo, error) {
	return nil, errors.New("unsupported operation")
}

func (p *envProfile) GetString(name FSName) (string, error) {
	switch name {
	case ProfileName:
		return p.name, nil
	default:
		return "", errors.New("unknown file name")
	}
}

func (p *envProfile) GetFile(name FSName) (ReadonlyFile, error) {
	return nil, errors.New("unsupported operation")
}

func (p *envProfile) SetString(name FSName, s string) error {
	return errors.New("unsupported operation")
}

func (p *envProfile) SetFile(name FSName, seeker io.Reader) error {
	return errors.New("unsupported operation")
}

func (p *envProfile) RemoveFile(name FSName) error {
	return errors.New("unsupported operation")
}

func newEnvProfileProv(id string, cfg *config.EnvProfile, certBytes []byte, provBytes []byte) *envProfile {
	return &envProfile{
		id:           id,
		name:         cfg.Name,
		prov:         provBytes,
		certPass:     cfg.CertPass,
		originalCert: certBytes,
	}
}

func newEnvProfileAccount(id string, cfg *config.EnvProfile, certBytes []byte) *envProfile {
	return &envProfile{
		id:           id,
		name:         cfg.Name,
		certPass:     cfg.CertPass,
		originalCert: certBytes,
		accountName:  cfg.AccountName,
		accountPass:  cfg.AccountPass,
	}
}

func (p *envProfile) GetId() string {
	return p.id
}

func (p *envProfile) GetFiles() ([]fileGetter, error) {
	isAccount, err := p.IsAccount()
	if err != nil {
		return nil, errors.WithMessage(err, "is account")
	}
	fromString := func(str string) func() (string, error) {
		return func() (string, error) {
			return str, nil
		}
	}
	fromBytes := func(str []byte) func() ([]byte, error) {
		return func() ([]byte, error) {
			return str, nil
		}
	}
	var files = []fileGetter{
		{name: "cert.p12", f3: fromBytes(p.fixedCert)},
		{name: "cert_pass.txt", f2: fromString(p.certPass)},
		{name: "team_id.txt", f2: fromString(p.teamId)},
	}
	if isAccount {
		files = append(files, []fileGetter{
			{name: "account_name.txt", f2: fromString(p.accountName)},
			{name: "account_pass.txt", f2: fromString(p.accountPass)},
		}...)
	} else {
		files = append(files, []fileGetter{
			{name: "prov.mobileprovision", f2: fromString(string(p.prov))},
		}...)
	}
	return files, nil
}

func (p *envProfile) IsAccount() (bool, error) {
	return p.accountName != "", nil
}
