package storage

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	"io/ioutil"
	"ios-signer-service/src/assets"
	"os"
	"path"
	"software.sslmate.com/src/go-pkcs12"
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
	originalCertFile, err := p.getOriginalCert()
	if err != nil {
		return nil, err
	}
	originalCertBytes, err := ioutil.ReadAll(originalCertFile)
	if err != nil {
		return nil, err
	}
	fixedCert, teamId, err := processP12(originalCertBytes, pass)
	if err != nil {
		return nil, errors.WithMessage(err, "validate certificate")
	}
	p.fixedCert = fixedCert
	p.teamId = teamId
	return &p, nil
}

// Validates the input P12 file, adds any missing standard CAs, and returns the new P12 along with the team ID.
func processP12(originalP12 []byte, pass string) ([]byte, string, error) {
	blocks, err := pkcs12.ToPEM(originalP12, pass)
	if err != nil {
		return nil, "", err
	}
	appleCerts, err := assets.AppleCerts.ReadDir("certs")
	if err != nil {
		return nil, "", err
	}
	for _, cert := range appleCerts {
		certBytes, err := assets.AppleCerts.ReadFile(path.Join("certs", cert.Name()))
		if err != nil {
			return nil, "", err
		}
		block, _ := pem.Decode(certBytes)
		blocks = append(blocks, block)
	}
	var keys []interface{}
	var authorities []*x509.Certificate
	var certificates []*x509.Certificate
	serialNumbers := map[string]bool{}
	for _, block := range blocks {
		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, "", err
			}
			serialNumber := cert.SerialNumber.String()
			if _, ok := serialNumbers[serialNumber]; ok {
				continue
			}
			serialNumbers[serialNumber] = true
			if cert.IsCA {
				authorities = append(authorities, cert)
			} else {
				certificates = append(certificates, cert)
			}
		case "PRIVATE KEY":
			var key interface{}
			if key, err = x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			} else if key, err = x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
			} else if key, err = x509.ParseECPrivateKey(block.Bytes); err == nil {
			} else {
				return nil, "", errors.New("unknown private key type")
			}
			keys = append(keys, key)
		}
	}
	if len(keys) != 1 {
		return nil, "", errors.Errorf("expected 1 private key, got %d", keys)
	}
	if len(certificates) != 1 {
		return nil, "", errors.Errorf("expected 1 signing certificate, got %d", len(certificates))
	}
	if len(authorities) < 1 {
		return nil, "", errors.New("no certificate authorities found")
	}
	cert := certificates[0]
	if len(cert.Subject.OrganizationalUnit) != 1 {
		return nil, "", errors.Errorf("certificate %s has invalid organization unit", cert.SerialNumber.String())
	}
	fixedP12, err := pkcs12.Encode(rand.Reader, keys[0], cert, authorities, pass)
	if err != nil {
		return nil, "", err
	}
	return fixedP12, cert.Subject.OrganizationalUnit[0], nil
}

type profile struct {
	id        string
	teamId    string
	fixedCert []byte
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
		return nil, errors.New("is account")
	}
	var files = []fileGetter{
		{name: "cert.p12", f3: p.getFixedCert},
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

func (p *profile) getOriginalCert() (ReadonlyFile, error) {
	file, err := os.Open(profileCertPath(p.id))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (p *profile) getFixedCert() ([]byte, error) {
	return p.fixedCert, nil
}

func (p *profile) getProv() (ReadonlyFile, error) {
	file, err := os.Open(profileProvPath(p.id))
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (p *profile) getAccountName() (string, error) {
	data, err := readTrimSpace(profileAccountNamePath(p.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (p *profile) getAccountPass() (string, error) {
	data, err := readTrimSpace(profileAccountPassPath(p.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (p *profile) getCertPass() (string, error) {
	data, err := readTrimSpace(profileCertPassPath(p.id))
	if err != nil {
		return "", err
	}
	return data, nil
}

func (p *profile) getTeamId() (string, error) {
	return p.teamId, nil
}

func (p *profile) GetName() (string, error) {
	data, err := readTrimSpace(profileNamePath(p.id))
	if err != nil {
		return "", err
	}
	return data, nil
}
