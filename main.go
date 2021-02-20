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
	"io"
	"ios-signer-service/assets"
	"ios-signer-service/config"
	"ios-signer-service/storage"
	"ios-signer-service/util"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	textTemplate "text/template"
	"time"
)

var (
	cfg             = config.Current
	formFileName    = "file"
	formProfileName = "profile_name"
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
	e.POST("/app/:id/signed", appResolver(uploadSignedApp))
	e.GET("/app/:id/manifest", appResolver(getManifest))
	e.GET("/app/:id/delete", appResolver(deleteApp))
	e.GET("/profile/:id/:file", profileResolver(getProfileFile))

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", *port)))
}

func appResolver(handler func(echo.Context, storage.App) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		if newId, ok := storage.OneTime.ResolveId(id); ok {
			id = newId
		}
		app, ok := storage.Apps.Get(id)
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, app)
	}
}

func profileResolver(handler func(echo.Context, storage.Profile) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		if newId, ok := storage.OneTime.ResolveId(id); ok {
			id = newId
		} else {
			// deny access to profiles via private id
			return c.NoContent(404)
		}
		profile, ok := storage.Profiles.GetById(id)
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, profile)
	}
}

func deleteApp(c echo.Context, app storage.App) error {
	if err := storage.Apps.Delete(app.GetId()); err != nil {
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

func getProfileFile(c echo.Context, profile storage.Profile) error {
	var file io.ReadSeekCloser
	var err error
	switch c.Param("file") {
	case "cert":
		file, err = profile.GetCert()
	case "prov":
		file, err = profile.GetProv()
	case "pass":
		file, err = profile.GetPassword()
	default:
		return c.NoContent(404)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	name, err := profile.GetName()
	if err != nil {
		return err
	}
	http.ServeContent(c.Response(), c.Request(), name, time.Now(), file)
	return nil
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
	if err := app.SetSigned(file); err != nil {
		return err
	}
	return c.NoContent(200)
}

func getSignedApp(c echo.Context, app storage.App) error {
	file, err := app.GetSigned()
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeFileResponse(c, file, app); err != nil {
		return err
	}
	return nil
}

func getUnsignedApp(c echo.Context, app storage.App) error {
	file, err := app.GetUnsigned()
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeFileResponse(c, file, app); err != nil {
		return err
	}
	return nil
}

func writeFileResponse(c echo.Context, file io.ReadSeeker, app storage.App) error {
	name, err := app.GetName()
	if err != nil {
		return err
	}
	//TODO: Should use the file's mod time, otherwise may tell client to use cached file even though it has changed
	modTime, err := app.GetModTime()
	if err != nil {
		return err
	}
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeContent(c.Response(), c.Request(), name, modTime, file)
	return nil
}

func uploadUnsignedApp(c echo.Context) error {
	profileName := c.FormValue(formProfileName)
	profile, ok := storage.Profiles.GetByName(profileName)
	if !ok {
		return errors.New("no profile with id " + profileName)
	}
	header, err := c.FormFile(formFileName)
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	app, err := storage.Apps.New(file, header.Filename, profile)
	if err != nil {
		return err
	}
	workflowUrl, err := triggerWorkflow(app, profile)
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
	data := assets.IndexData{
		FormFileName:    formFileName,
		FormProfileName: formProfileName,
	}
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
			log.Println(errors.WithMessage(err, "get name"))
		}
		workflowUrl, err := app.GetWorkflowUrl()
		if err != nil {
			log.Println(errors.WithMessage(err, "get workflow url"))
		}
		profileId, err := app.GetProfileId()
		if err != nil {
			log.Println(errors.WithMessage(err, "get profile id"))
		}
		var profileName string
		if profile, ok := storage.Profiles.GetById(profileId); ok {
			profileName, err = profile.GetName()
			if err != nil {
				log.Println(errors.WithMessage(err, "get profile name"))
			}
		} else {
			log.Println(errors.WithMessage(err, "get profile"))
			profileName = "unknown"
		}

		data.Apps = append(data.Apps, assets.App{
			Id:          app.GetId(),
			IsSigned:    isSigned,
			Name:        name,
			ModTime:     modTime,
			WorkflowUrl: workflowUrl,
			ProfileName: profileName,
			ManifestUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "manifest"),
			DownloadUrl: util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "signed"),
			DeleteUrl:   util.JoinUrlsPanic(config.Current.ServerURL, "app", app.GetId(), "delete"),
		})
	}
	// reverse sort
	sort.Slice(data.Apps, func(i, j int) bool {
		return data.Apps[i].ModTime.After(data.Apps[j].ModTime)
	})
	profiles, err := storage.Profiles.GetAll()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		name, err := profile.GetName()
		if err != nil {
			return err
		}
		data.Profiles = append(data.Profiles, assets.Profile{
			Id:   profile.GetId(),
			Name: name,
		})
	}
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

func triggerWorkflow(app storage.App, profile storage.Profile) (string, error) {
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
				"download_suffix": path.Join("app", storage.OneTime.MakeId(app.GetId()), "unsigned"),
				"upload_suffix":   path.Join("app", storage.OneTime.MakeId(app.GetId()), "signed"),
				"cert_suffix":     path.Join("profile", storage.OneTime.MakeId(profile.GetId()), "cert"),
				"prov_suffix":     path.Join("profile", storage.OneTime.MakeId(profile.GetId()), "prov"),
				"pass_suffix":     path.Join("profile", storage.OneTime.MakeId(profile.GetId()), "pass"),
			},
		}); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/%s/%s/actions", config.Current.RepoOwner, config.Current.RepoName), nil
}
