package storage

import (
	"archive/tar"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
	"io"
	"time"
)

// A signing job waiting to be picked up by a builder.
type signJob struct {
	ts        time.Time
	appId     string
	profileId string
}

// When a signJob has been picked up by a builder, it's replaced
// with a return job waiting for the builder to submit its results.
type ReturnJob struct {
	Id            string
	Ts            time.Time
	AppId         string
	TwoFactorCode atomic.String
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
	id := uuid.NewString()
	files, err := profile.GetFiles()
	if err != nil {
		return "", errors.WithMessage(err, "get profile files")
	}
	files = append(files, []fileGetter{
		{name: "unsigned.ipa", f1: app.GetUnsigned},
		{name: "id.txt", f2: func() (string, error) { return id, nil }},
		{name: "args.txt", f2: app.GetSignArgs},
		{name: "user_bundle_id.txt", f2: app.GetUserBundleId},
	}...)
	for _, file := range files {
		if err := tarPackage(w, &file); err != nil {
			return "", errors.WithMessage(err, "tar package")
		}
	}
	return id, nil
}

func tarPackage(w *tar.Writer, fileGen *fileGetter) error {
	if fileGen.f1 != nil {
		file, err := fileGen.f1()
		if err != nil {
			return errors.WithMessage(err, "read "+fileGen.name)
		}
		defer file.Close()
		if err := tarWriteFile(w, fileGen.name, file); err != nil {
			return errors.WithMessage(err, "write file")
		}
	} else if fileGen.f2 != nil {
		data, err := fileGen.f2()
		if err != nil {
			return errors.WithMessage(err, "read "+fileGen.name)
		}
		if err := tarWriteString(w, fileGen.name, data); err != nil {
			return errors.WithMessage(err, "write bytes")
		}
	} else {
		log.Fatal().Msg("badly initialized fileGetter")
	}
	return nil
}

func tarWriteFile(w *tar.Writer, name string, file ReadonlyFile) error {
	stat, err := file.Stat()
	if err != nil {
		return errors.WithMessage(err, "stat "+name)
	}
	if err := w.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0600,
		Size: stat.Size(),
	}); err != nil {
		return errors.WithMessage(err, "write header "+name)
	}
	if _, err := io.Copy(w, file); err != nil {
		return errors.WithMessage(err, "write "+name)
	}
	return nil
}

func tarWriteString(w *tar.Writer, name string, data string) error {
	if err := w.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0600,
		Size: int64(len(data)),
	}); err != nil {
		return errors.WithMessage(err, "write header "+name)
	}
	if _, err := w.Write([]byte(data)); err != nil {
		return errors.WithMessage(err, "write "+name)
	}
	return nil
}
