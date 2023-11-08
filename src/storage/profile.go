package storage

import (
	"SignTools/src/assets"
	"SignTools/src/util"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path"
	"software.sslmate.com/src/go-pkcs12"
)

var ProfilePaths = []FSName{ProfileCert, ProfileCertPass, ProfileProv, ProfileName, ProfileAccountName, ProfileAccountPass}

const (
	ProfileRoot        = FSName("")
	ProfileCert        = FSName("cert.p12")
	ProfileCertPass    = FSName("cert_pass.txt")
	ProfileProv        = FSName("prov.mobileprovision")
	ProfileName        = FSName("name.txt")
	ProfileAccountName = FSName("account_name.txt")
	ProfileAccountPass = FSName("account_pass.txt")
)

type Profile interface {
	GetId() string
	GetFiles() ([]fileGetter, error)
	IsAccount() (bool, error)
	FileSystem
}

func newProfile(id string) *profile {
	return &profile{id: id, FileSystemBase: FileSystemBase{resolvePath: func(name FSName) string {
		return util.SafeJoinFilePaths(profilesPath, id, string(name))
	}}}
}

func loadProfile(id string) (*profile, error) {
	p := newProfile(id)
	isAccount, err := p.IsAccount()
	if err != nil {
		return nil, errors.WithMessage(err, "is account")
	}
	for _, file := range ProfilePaths {
		if !isAccount && (file == ProfileAccountName || file == ProfileAccountPass) {
			continue
		}
		if isAccount && file == ProfileProv {
			continue
		}
		if _, err := p.Stat(file); err != nil {
			return nil, errors.WithMessagef(err, "check required file %s", file)
		}
	}
	pass, err := p.GetString(ProfileCertPass)
	if err != nil {
		return nil, errors.WithMessagef(err, "get %s", ProfileCertPass)
	}
	origCertFile, err := p.GetFile(ProfileCert)
	if err != nil {
		return nil, errors.WithMessagef(err, "get %s", ProfileCert)
	}
	defer origCertFile.Close()
	origCertBytes, err := ioutil.ReadAll(origCertFile)
	if err != nil {
		return nil, errors.WithMessage(err, "read cert file")
	}
	fixedCert, teamId, err := processP12(origCertBytes, pass)
	if err != nil {
		return nil, errors.WithMessage(err, "validate certificate")
	}
	p.fixedCert = fixedCert
	p.teamId = teamId
	return p, nil
}

type PublicKeyComparator interface {
	Equal(x crypto.PublicKey) bool
}

// Validates the input P12 file, adds any missing standard CAs, and returns the new P12 along with the team ID.
func processP12(originalP12 []byte, pass string) ([]byte, string, error) {
	blocks, err := pkcs12.ToPEM(originalP12, pass)
	if err != nil {
		return nil, "", errors.WithMessage(err, "p12 to pem")
	}
	appleCerts, err := assets.AppleCerts.ReadDir("certs")
	if err != nil {
		return nil, "", errors.WithMessage(err, "read certs dir")
	}
	for _, cert := range appleCerts {
		certBytes, err := assets.AppleCerts.ReadFile(path.Join("certs", cert.Name()))
		if err != nil {
			return nil, "", errors.WithMessagef(err, "read cert %s", cert.Name())
		}
		block, _ := pem.Decode(certBytes)
		blocks = append(blocks, block)
	}
	keyMap := map[any]PublicKeyComparator{}
	var authorities []*x509.Certificate
	var certificates []*x509.Certificate
	serialNumbers := map[string]bool{}
	for _, block := range blocks {
		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, "", errors.WithMessage(err, "parse certificate")
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
			var key any
			if key, err = x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				keyMap[key] = key.(*rsa.PrivateKey).Public().(*rsa.PublicKey)
			} else if key, err = x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				switch v := key.(type) {
				case *rsa.PrivateKey:
					keyMap[key] = v.Public().(*rsa.PublicKey)
				case *ecdsa.PrivateKey:
					keyMap[key] = v.Public().(*ecdsa.PublicKey)
				case *ed25519.PrivateKey:
					keyMap[key] = v.Public().(*ed25519.PublicKey)
				default:
					return nil, "", errors.New("unknown private key type")
				}
			} else if key, err = x509.ParseECPrivateKey(block.Bytes); err == nil {
				keyMap[key] = key.(*ecdsa.PrivateKey).Public().(*ecdsa.PublicKey)
			} else {
				return nil, "", errors.New("unknown private key type")
			}
		}
	}
	if len(keyMap) < 1 {
		return nil, "", errors.Errorf("no private keys found")
	}
	if len(certificates) < 1 {
		return nil, "", errors.Errorf("no signing certificates found")
	}
	if len(authorities) < 1 {
		return nil, "", errors.New("no certificate authorities found")
	}
	for _, cert := range certificates {
		if len(cert.Subject.OrganizationalUnit) != 1 {
			return nil, "", errors.Errorf("certificate %s has invalid organization unit, bad item count", cert.SerialNumber.String())
		}
		valid := false
		for _, publicKey := range keyMap {
			if publicKey.Equal(cert.PublicKey) {
				valid = true
				break
			}
		}
		if !valid {
			return nil, "", errors.Errorf("certificate %s has no matching private key", cert.SerialNumber.String())
		}
	}
	orgUnit := certificates[0].Subject.OrganizationalUnit[0]
	for _, cert := range certificates {
		if cert.Subject.OrganizationalUnit[0] != orgUnit {
			return nil, "", errors.Errorf("certificate %s has invalid organization unit, not the same as the others", cert.SerialNumber.String())
		}
	}
	var keys []any
	for key := range keyMap {
		keys = append(keys, key)
	}
	fixedP12, err := pkcs12.LegacyDES.Encode(keys, certificates, authorities, pass)
	if err != nil {
		return nil, "", errors.WithMessage(err, "encode final p12")
	}
	return fixedP12, orgUnit, nil
}

type profile struct {
	id        string
	teamId    string
	fixedCert []byte
	FileSystemBase
}

func (p *profile) GetId() string {
	return p.id
}

func (p *profile) IsAccount() (bool, error) {
	if _, err := os.Stat(p.resolvePath(ProfileAccountName)); os.IsNotExist(err) {
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
		{name: "cert_pass.txt", f2: func() (string, error) { return p.GetString(ProfileCertPass) }},
		{name: "team_id.txt", f2: p.getTeamId},
	}
	if isAccount {
		files = append(files, []fileGetter{
			{name: "account_name.txt", f2: func() (string, error) { return p.GetString(ProfileAccountName) }},
			{name: "account_pass.txt", f2: func() (string, error) { return p.GetString(ProfileAccountPass) }},
		}...)
	} else {
		files = append(files, []fileGetter{
			{name: "prov.mobileprovision", f1: func() (ReadonlyFile, error) { return p.GetFile(ProfileProv) }},
		}...)
	}
	return files, nil
}

func (p *profile) getFixedCert() ([]byte, error) {
	return p.fixedCert, nil
}

func (p *profile) getTeamId() (string, error) {
	return p.teamId, nil
}
