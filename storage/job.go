package storage

import (
	"archive/tar"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"time"
)

type signJob struct {
	ts        time.Time
	appId     string
	profileId string
}

type returnJob struct {
	ts    time.Time
	appId string
}

func (j *signJob) writeArchive(writer io.Writer) (string, error) {
	app, ok := Apps.Get(j.appId)
	if !ok {
		return "", errors.New("invalid app id")
	}
	profile, ok := Profiles.GetById(j.profileId)
	if !ok {
		return "", errors.New("invalid profile id")
	}
	w := tar.NewWriter(writer)
	defer w.Close()
	files := []fileGenerator{
		{name: "unsigned.ipa", f: app.GetUnsigned},
		{name: "prov.mobileprovision", f: profile.GetProv},
		{name: "cert.p12", f: profile.GetCert},
		{name: "pass.txt", f: profile.GetPassword},
	}
	for _, file := range files {
		if err := tarPackage(w, &file); err != nil {
			return "", errors.WithMessage(err, "tar package")
		}
	}
	id := uuid.NewString()
	if err := tarWrite(w, "id.txt", []byte(id)); err != nil {
		return "", errors.WithMessage(err, "tar write id")
	}
	args, err := app.GetSignArgs()
	if err != nil {
		return "", errors.WithMessage(err, "get app args")
	}
	if err := tarWrite(w, "args.txt", []byte(args)); err != nil {
		return "", errors.WithMessage(err, "tar write args")
	}
	return id, nil
}

type fileGenerator struct {
	name string
	f    func() (ReadonlyFile, error)
}

func tarPackage(w *tar.Writer, fileGen *fileGenerator) error {
	file, err := fileGen.f()
	if err != nil {
		return errors.WithMessage(err, "read "+fileGen.name)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return errors.WithMessage(err, "stat "+fileGen.name)
	}
	if err := w.WriteHeader(&tar.Header{
		Name: fileGen.name,
		Mode: 0600,
		Size: stat.Size(),
	}); err != nil {
		return errors.WithMessage(err, "write header "+fileGen.name)
	}
	if _, err := io.Copy(w, file); err != nil {
		return errors.WithMessage(err, "write "+fileGen.name)
	}
	return nil
}

func tarWrite(w *tar.Writer, name string, data []byte) error {
	if err := w.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}); err != nil {
		return errors.WithMessage(err, "write header "+name)
	}
	if _, err := w.Write(data); err != nil {
		return errors.WithMessage(err, "write "+name)
	}
	return nil
}
