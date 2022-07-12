package storage

import (
	"archive/tar"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
	"io"
	"os"
	"path"
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

func (j *signJob) writeArchive(returnJobId string, writer io.Writer) error {
	app, ok := Apps.Get(j.appId)
	if !ok {
		return errors.New("invalid app id")
	}
	profile, ok := Profiles.GetById(j.profileId)
	if !ok {
		return errors.New("invalid profile id")
	}
	w := tar.NewWriter(writer)
	defer w.Close()
	files, err := profile.GetFiles()
	if err != nil {
		return errors.WithMessage(err, "get profile files")
	}
	files = append(files, []fileGetter{
		{name: "id.txt", f2: func() (string, error) { return returnJobId, nil }},
		{name: "args.txt", f2: func() (string, error) { return app.GetString(AppSignArgs) }},
		{name: "user_bundle_id.txt", f2: func() (string, error) { return app.GetString(AppUserBundleId) }},
	}...)
	if bundleName, err := app.GetString(AppBundleName); err == nil {
		files = append(files, fileGetter{name: "bundle_name.txt", f2: func() (string, error) { return bundleName, nil }})
	} else if !os.IsNotExist(err) {
		return err
	}
	if tweaks, err := app.ReadDir(TweaksDir); err == nil {
		if err := w.WriteHeader(&tar.Header{
			Name:     string(TweaksDir),
			Typeflag: tar.TypeDir,
			Mode:     0700,
		}); err != nil {
			return err
		}
		for _, tweak := range tweaks {
			tweakPath := FSName(path.Join(string(TweaksDir), tweak.Name()))
			files = append(files, []fileGetter{
				{name: string(tweakPath), f1: func() (ReadonlyFile, error) { return app.GetFile(tweakPath) }},
			}...)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	for _, file := range files {
		if err := tarPackage(w, &file); err != nil {
			// sabotage the archive to nudge the client that something is wrong
			w.WriteHeader(&tar.Header{
				Name: "ERROR",
				Mode: 0600,
				Size: 1024,
			})
			return errors.WithMessage(err, "tar package")
		}
	}
	return nil
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
		if err := tarWriteBytes(w, fileGen.name, []byte(data)); err != nil {
			return errors.WithMessage(err, "write bytes")
		}
	} else if fileGen.f3 != nil {
		data, err := fileGen.f3()
		if err != nil {
			return errors.WithMessage(err, "read "+fileGen.name)
		}
		if err := tarWriteBytes(w, fileGen.name, data); err != nil {
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

func tarWriteBytes(w *tar.Writer, name string, data []byte) error {
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
