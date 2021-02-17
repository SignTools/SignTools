package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/v33/github"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	htmlTemplate "html/template"
	"io"
	"io/ioutil"
	"ios-signer-service/assets"
	"ios-signer-service/config"
	"ios-signer-service/util"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	textTemplate "text/template"
	"time"
)

var cfg = config.Current

func cleanupFiles() error {
	idFiles, err := ioutil.ReadDir(cfg.SaveDir)
	if err != nil {
		return errors.WithMessage(err, "could not read save directory")
	}
	now := time.Now()
	for _, idFile := range idFiles {
		if idFile.ModTime().Add(time.Duration(cfg.CleanupMins) * time.Minute).Before(now) {
			name, err := getFileName(idFile.Name())
			if err != nil {
				return errors.WithMessage(err, "could not read file name")
			}
			log.Printf("Removing file: %s/%s", idFile.Name(), name)
			if err := os.RemoveAll(filepath.Join(cfg.SaveDir, idFile.Name())); err != nil {
				return errors.WithMessage(err, "could not remove file")
			}
		}
	}
	return nil
}

var authMiddleware = middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
	return key == cfg.Key, nil
})

func main() {
	port := flag.Uint64("port", 8080, "Listen port")
	flag.Parse()

	if err := os.MkdirAll(cfg.SaveDir, 0777); err != nil {
		log.Fatalln(err)
	}

	go func() {
		for {
			if err := cleanupFiles(); err != nil {
				log.Println(err)
			}
			time.Sleep(time.Duration(cfg.CleanupIntervalMins) * time.Minute)
		}
	}()

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())

	e.GET("/", index)
	e.POST("/app", uploadUnsigned)
	e.GET("/app/:id/unsigned", downloadUnsigned)
	e.GET("/app/:id/signed", downloadSigned)
	e.GET("/app/:id/manifest", downloadManifest)
	e.GET("/app/:id/delete", deleteApp)

	e.GET("/cert/:file", certFile, authMiddleware)
	e.POST("/app/:id/signed", uploadSigned, authMiddleware)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", *port)))
}

func deleteApp(c echo.Context) error {
	filePath := config.SaveFilePath(c.Param("id"))
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.NoContent(404)
	} else if err != nil {
		return err
	}
	if err := os.RemoveAll(filePath); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func downloadManifest(c echo.Context) error {
	t, err := textTemplate.New("").Parse(assets.ManifestPlist)
	if err != nil {
		return err
	}
	fileName, err := getFileName(c.Param("id"))
	if err != nil {
		return err
	}
	data := assets.ManifestData{
		DownloadUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", c.Param("id"), "signed"),
		BundleId:    "com.foo.bar",
		Title:       fileName,
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.Blob(200, "text/plain", result.Bytes())
}

func certFile(c echo.Context) error {
	addFileNameHeader(c, c.Param("file"))
	return c.File(util.SafeJoin(cfg.CertDir, c.Param("file")))
}

func uploadSigned(c echo.Context) error {
	file, err := c.FormFile(config.FormFileName)
	if err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(config.SaveSignedPath(c.Param("id")))
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, src); err != nil {
		return err
	}
	return c.NoContent(200)
}

func downloadSigned(c echo.Context) error {
	name, err := getFileName(c.Param("id"))
	if err != nil {
		return err
	}
	addFileNameHeader(c, name)
	return c.File(config.SaveSignedPath(c.Param("id")))
}

func downloadUnsigned(c echo.Context) error {
	name, err := getFileName(c.Param("id"))
	if err != nil {
		return err
	}
	addFileNameHeader(c, name)
	return c.File(config.SaveUnsignedPath(c.Param("id")))
}

func addFileNameHeader(c echo.Context, name string) {
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
}

func getFileName(id string) (string, error) {
	nameBytes, err := ioutil.ReadFile(config.SaveDisplayNamePath(id))
	if err != nil {
		return "", err
	}
	return string(nameBytes), nil
}

func uploadUnsigned(c echo.Context) error {
	file, err := c.FormFile(config.FormFileName)
	if err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	id := uuid.NewString()
	if err := os.MkdirAll(config.SaveFilePath(id), 0666); err != nil {
		return err
	}
	nameBytes := []byte(filepath.Clean(file.Filename))
	if err := ioutil.WriteFile(config.SaveDisplayNamePath(id), nameBytes, 0600); err != nil {
		return err
	}
	dst, err := os.Create(config.SaveUnsignedPath(id))
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, src); err != nil {
		return err
	}
	workflowUrl, err := triggerWorkflow(id)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(config.SaveWorkflowPath(id), []byte(workflowUrl), 0600); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func index(c echo.Context) error {
	idFiles, err := os.ReadDir(cfg.SaveDir)
	if err != nil {
		return err
	}
	data := assets.IndexData{}
	for _, idFile := range idFiles {
		name, err := getFileName(idFile.Name())
		if err != nil {
			return err
		}
		var isSigned = false
		if _, err := os.Stat(config.SaveSignedPath(idFile.Name())); err == nil {
			isSigned = true
		} else if !os.IsNotExist(err) {
			return err
		}
		info, err := idFile.Info()
		if err != nil {
			return err
		}
		var workflowUrl []byte
		if !isSigned {
			workflowUrl, err = ioutil.ReadFile(config.SaveWorkflowPath(idFile.Name()))
			if err != nil {
				log.Printf("failed to read job url for %s (%s)\n", idFile.Name(), name)
				workflowUrl = []byte("#")
			}
		}
		data.Files = append(data.Files, assets.ServerFile{
			Id:           idFile.Name(),
			IsSigned:     isSigned,
			Name:         name,
			UploadedTime: info.ModTime(),
			JobUrl:       string(workflowUrl),
			ManifestUrl:  util.JoinUrlsPanic(config.Current.ServerURL, "app", idFile.Name(), "manifest"),
			DownloadUrl:  util.JoinUrlsPanic(config.Current.ServerURL, "app", idFile.Name(), "signed"),
			DeleteUrl:    util.JoinUrlsPanic(config.Current.ServerURL, "app", idFile.Name(), "delete"),
		})
	}
	// reverse sort
	sort.Slice(data.Files, func(i, j int) bool {
		return data.Files[i].UploadedTime.After(data.Files[j].UploadedTime)
	})
	t, err := htmlTemplate.New("").Parse(assets.IndexHtml)
	if err != nil {
		return err
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.HTMLBlob(200, result.Bytes())
}

func triggerWorkflow(id string) (string, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	if _, err := client.Actions.CreateWorkflowDispatchEventByFileName(
		ctx,
		cfg.RepoOwner,
		cfg.RepoName,
		cfg.WorkflowFileName,
		github.CreateWorkflowDispatchEventRequest{
			Ref: cfg.WorkflowRef,
			Inputs: map[string]interface{}{
				"download_suffix": path.Join("app", id, "unsigned"),
				"upload_suffix":   path.Join("app", id, "signed"),
				"cert_suffix":     path.Join("cert", config.Current.CertFileName),
				"prov_suffix":     path.Join("cert", config.Current.ProvFileName),
			},
		}); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/%s/%s/actions", config.Current.RepoOwner, config.Current.RepoName), nil
}
