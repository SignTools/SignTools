package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/v33/github"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	htmlTemplate "html/template"
	"ios-signer-service/assets"
	"ios-signer-service/config"
	"ios-signer-service/storage"
	"ios-signer-service/util"
	"log"
	"os"
	"path"
	"sort"
	textTemplate "text/template"
	"time"
)

var (
	cfg          = config.Current
	formFileName = "file"
)

func cleanupApps() error {
	apps, err := storage.Apps.GetAll()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, app := range apps {
		modTime, err := app.GetModTime()
		if err != nil {
			return err
		}
		if modTime.Add(time.Duration(cfg.CleanupMins) * time.Minute).Before(now) {
			if err := storage.Apps.Delete(app.GetId()); err != nil {
				return err
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
			if err := cleanupApps(); err != nil {
				log.Println(errors.WithMessage(err, "cleanup apps"))
			}
			time.Sleep(time.Duration(cfg.CleanupIntervalMins) * time.Minute)
		}
	}()

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())

	e.GET("/", index)
	e.POST("/app", uploadUnsignedApp)
	e.GET("/app/:id/unsigned", appResolver(getUnsignedApp))
	e.GET("/app/:id/signed", appResolver(getSignedApp))
	e.GET("/app/:id/manifest", appResolver(getManifest))
	e.GET("/app/:id/delete", deleteApp)

	e.GET("/cert/:file", getCertFile, authMiddleware)
	e.POST("/app/:id/signed", appResolver(uploadSignedApp), authMiddleware)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", *port)))
}

func appResolver(handler func(echo.Context, storage.App) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		app, ok := storage.Apps.Get(c.Param("id"))
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, app)
	}
}

func deleteApp(c echo.Context) error {
	if err := storage.Apps.Delete(c.Param("id")); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func getManifest(c echo.Context, app storage.App) error {
	t, err := textTemplate.New("").Parse(assets.ManifestPlist)
	if err != nil {
		return err
	}
	appName, err := app.GetName()
	if err != nil {
		return err
	}
	data := assets.ManifestData{
		DownloadUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", c.Param("id"), "signed"),
		BundleId:    "com.foo.bar",
		Title:       appName,
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.Blob(200, "text/plain", result.Bytes())
}

func getCertFile(c echo.Context) error {
	writeFileNameHeader(c, c.Param("file"))
	return c.File(util.SafeJoin(cfg.CertDir, c.Param("file")))
}

func uploadSignedApp(c echo.Context, app storage.App) error {
	header, err := c.FormFile(formFileName)
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := app.SetSigned(file); err != nil {
		return err
	}
	return c.NoContent(200)
}

func getSignedApp(c echo.Context, app storage.App) error {
	if _, err := app.GetSigned(c.Response().Writer); err != nil {
		return err
	}
	return writeAppResponse(c, app)
}

func getUnsignedApp(c echo.Context, app storage.App) error {
	if _, err := app.GetUnsigned(c.Response().Writer); err != nil {
		return err
	}
	return writeAppResponse(c, app)
}

func writeAppResponse(c echo.Context, app storage.App) error {
	name, err := app.GetName()
	if err != nil {
		return err
	}
	writeFileNameHeader(c, name)
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().WriteHeader(200)
	return nil
}

func writeFileNameHeader(c echo.Context, name string) {
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
}

func uploadUnsignedApp(c echo.Context) error {
	header, err := c.FormFile(formFileName)
	if err != nil {
		return err
	}
	app, err := storage.Apps.New()
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := app.SetUnsigned(file); err != nil {
		return err
	}
	workflowUrl, err := triggerWorkflow(app.GetId())
	if err != nil {
		return err
	}
	if err := app.SetWorkflowUrl(workflowUrl); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func index(c echo.Context) error {
	apps, err := storage.Apps.GetAll()
	if err != nil {
		return err
	}
	data := assets.IndexData{}
	for _, app := range apps {
		isSigned, err := app.IsSigned()
		if err != nil {
			return err
		}
		modTime, err := app.GetModTime()
		if err != nil {
			return err
		}
		name, err := app.GetName()
		if err != nil {
			log.Printf("read name for %s: %v\n", app.GetId(), err)
		}
		workflowUrl, err := app.GetWorkflowUrl()
		if err != nil {
			log.Printf("read workflow url for %s (%s): %v\n", app.GetId(), name, err)
		}
		data.Apps = append(data.Apps, assets.App{
			Id:          app.GetId(),
			IsSigned:    isSigned,
			Name:        name,
			ModTime:     modTime,
			WorkflowUrl: workflowUrl,
			ManifestUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "manifest"),
			DownloadUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "signed"),
			DeleteUrl:   util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "delete"),
		})
	}
	// reverse sort
	sort.Slice(data.Apps, func(i, j int) bool {
		return data.Apps[i].ModTime.After(data.Apps[j].ModTime)
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
