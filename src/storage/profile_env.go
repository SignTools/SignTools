package storage

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"ios-signer-service/src/config"
	"os"
)

// Attempts to parse a Profile from environment variables.
// Returns os.ErrNotExist if variables are missing, otherwise any other error.
func newEnvProfile(cfg *config.EnvProfile) (*envProfile, error) {
	if cfg.Name == "" || cfg.CertBase64 == "" || cfg.CertPass == "" {
		return nil, os.ErrNotExist
	}
	if cfg.ProvBase64 != "" {
		log.Info().Msg("importing cert profile from envvars")
		certBytes, err := decodeVar(cfg.CertBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode cert base64")
		}
		provBytes, err := decodeVar(cfg.ProvBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode prov base64")
		}
		return &envProfile{
			id:       "imported",
			name:     cfg.Name,
			prov:     provBytes,
			certPass: cfg.CertPass,
			cert:     certBytes,
		}, nil
	} else if cfg.AccountName != "" && cfg.AccountPass != "" {
		log.Info().Msg("importing account profile from envvars")
		certBytes, err := decodeVar(cfg.CertBase64)
		if err != nil {
			return nil, errors.WithMessage(err, "decode cert base64")
		}
		return &envProfile{
			id:          "imported",
			name:        cfg.Name,
			certPass:    cfg.CertPass,
			cert:        certBytes,
			accountName: cfg.AccountName,
			accountPass: cfg.AccountPass,
		}, nil
	}
	return nil, os.ErrNotExist
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
	id          string
	name        string
	prov        []byte
	certPass    string
	cert        []byte
	accountName string
	accountPass string
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
	var files = []fileGetter{
		{name: "cert.p12", f2: fromString(string(p.cert))},
		{name: "cert_pass.txt", f2: fromString(p.certPass)},
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

func (p *envProfile) GetName() (string, error) {
	return p.name, nil
}

func (p *envProfile) IsAccount() (bool, error) {
	return p.accountName != "", nil
}
