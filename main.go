package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	htmlTemplate "html/template"
	"io"
	"ios-signer-service/src/assets"
	"ios-signer-service/src/config"
	"ios-signer-service/src/ngrok"
	"ios-signer-service/src/storage"
	"ios-signer-service/src/util"
	"log"
	"mime"
	"net/http"
	"os"
	textTemplate "text/template"
	"time"
)

var formNames = assets.FormNames{
	FormFile:         "file",
	FormProfileId:    "profile_id",
	FormAppDebug:     "all_devices",
	FormAllDevices:   "app_debug",
	FormFileShare:    "file_share",
	FormToken:        "align_app_id",
	FormId:           "id",
	FormIdOriginal:   "id_original",
	FormIdProv:       "id_prov",
	FormIdCustom:     "id_custom",
	FormIdCustomText: "id_custom_text",
	FormBundleId:     "bundle_id",
}

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
		if modTime.Add(time.Duration(config.Current.CleanupMins) * time.Minute).Before(now) {
			if err := storage.Apps.Delete(app.GetId()); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	host := flag.String("host", "", "Listen host, empty for all")
	port := flag.Uint64("port", 8080, "Listen port")
	configFile := flag.String("config", "signer-cfg.yml", "Configuration file")
	ngrokPort := flag.Uint64("ngrok-port", 0, "Ngrok web interface port. "+
		"Used to automatically parse the server_url")
	flag.Parse()

	config.Load(*configFile)
	storage.Load()
	if *ngrokPort != 0 {
		publicUrl, err := ngrok.GetPublicUrl(*ngrokPort, "https", 10*time.Second)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("ngrok public URL: " + publicUrl)
		config.Current.ServerUrl = publicUrl
	}

	serve(*host, *port)
}

func serve(host string, port uint64) {
	if err := os.MkdirAll(config.Current.SaveDir, 0777); err != nil {
		log.Fatalln(err)
	}

	if config.Current.CleanupIntervalMins > 0 && config.Current.CleanupMins > 0 {
		go func() {
			for {
				if err := cleanupApps(); err != nil {
					log.Println(errors.WithMessage(err, "cleanup apps"))
				}
				time.Sleep(time.Duration(config.Current.CleanupIntervalMins) * time.Minute)
			}
		}()
	}

	if err := config.Current.Builder.SetSecrets(map[string]string{
		"SECRET_KEY": config.Current.BuilderKey,
		"SECRET_URL": config.Current.ServerUrl,
	}); err != nil {
		log.Fatalln(err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())

	forcedBasicAuth := middleware.BasicAuth(func(username string, password string, c echo.Context) (bool, error) {
		return username == config.Current.BasicAuth.Username && password == config.Current.BasicAuth.Password, nil
	})
	basicAuth := func(f echo.HandlerFunc) echo.HandlerFunc {
		if config.Current.BasicAuth.Enable {
			return forcedBasicAuth(f)
		} else {
			return f
		}
	}
	workflowKeyAuth := middleware.KeyAuth(func(s string, c echo.Context) (bool, error) {
		return s == config.Current.BuilderKey, nil
	})

	e.GET("/", renderIndex, basicAuth)
	e.GET("/favicon.png", getFavIcon, basicAuth)
	e.POST("/apps", uploadUnsignedApp, basicAuth)
	e.GET("/apps/:id/signed", appResolver(getSignedApp))
	e.GET("/apps/:id/manifest", appResolver(getManifest))
	e.GET("/apps/:id/delete", appResolver(deleteApp), basicAuth)
	e.GET("/apps/:id/2fa", appResolver(render2FAPage), basicAuth)
	e.POST("/apps/:id/2fa", appResolver(set2FA), basicAuth)
	e.GET("/jobs", getLastJob, workflowKeyAuth)
	e.GET("/jobs/:id/2fa", jobResolver(get2FA), workflowKeyAuth)
	e.POST("/jobs/:id/signed", jobResolver(uploadSignedApp), workflowKeyAuth)

	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", host, port)))
}

func getFavIcon(c echo.Context) error {
	http.ServeContent(c.Response(), c.Request(), assets.FavIconStat.Name(), assets.FavIconStat.ModTime(), bytes.NewReader(assets.FavIconBytes))
	return nil
}

func uploadSignedApp(c echo.Context, job *storage.ReturnJob) error {
	if !storage.Jobs.DeleteById(job.Id) {
		return errors.New("unable to delete return job " + job.Id)
	}
	app, ok := storage.Apps.Get(job.AppId)
	if !ok {
		return errors.New(fmt.Sprintf("return job %s appid %s not resolved", job.Id, job.AppId))
	}
	header, err := c.FormFile(formNames.FormFile)
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	if err := app.SetSigned(file, c.FormValue(formNames.FormBundleId)); err != nil {
		return err
	}
	return c.NoContent(200)
}

func get2FA(c echo.Context, job *storage.ReturnJob) error {
	code := job.TwoFactorCode.Load()
	if code == "" {
		return c.NoContent(404)
	} else {
		return c.String(200, code)
	}
}

func appResolver(handler func(echo.Context, storage.App) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		app, ok := storage.Apps.Get(id)
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, app)
	}
}

func jobResolver(handler func(echo.Context, *storage.ReturnJob) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		job, ok := storage.Jobs.GetById(id)
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, job)
	}
}

func getLastJob(c echo.Context) error {
	if err := storage.Jobs.TakeLastJob(c.Response()); errors.Is(err, storage.ErrNotFound) {
		return c.NoContent(404)
	} else if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type", mime.TypeByExtension(".tar"))
	return c.NoContent(200)
}

func render2FAPage(c echo.Context, _ storage.App) error {
	return c.HTML(200, assets.TwoFactorHtml)
}

func set2FA(c echo.Context, app storage.App) error {
	job, ok := storage.Jobs.GetByAppId(app.GetId())
	if !ok {
		return errors.New("no job found for app " + app.GetId())
	}
	job.TwoFactorCode.Store(c.FormValue("formToken"))
	return c.Redirect(302, "/")
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
		DownloadUrl: util.JoinUrlsPanic(config.Current.ServerUrl, "apps", c.Param("id"), "signed"),
		BundleId:    "com.foo.bar",
		Title:       appName,
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.Blob(200, "text/plain", result.Bytes())
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
	profileId := c.FormValue(formNames.FormProfileId)
	profile, ok := storage.Profiles.GetById(profileId)
	if !ok {
		return errors.New("no profile with id " + profileId)
	}
	header, err := c.FormFile(formNames.FormFile)
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	signArgs := ""
	if c.FormValue(formNames.FormAllDevices) != "" {
		signArgs += " -a"
	}
	if c.FormValue(formNames.FormAppDebug) != "" {
		signArgs += " -d"
	}
	if c.FormValue(formNames.FormFileShare) != "" {
		signArgs += " -s"
	}
	idType := c.FormValue(formNames.FormId)
	userBundleId := c.FormValue(formNames.FormIdCustomText)
	if idType == formNames.FormIdProv {
		signArgs += " -n"
	} else if idType == formNames.FormIdCustom {
		signArgs += " -b " + userBundleId
	}
	app, err := storage.Apps.New(file, header.Filename, profile, signArgs)
	if err != nil {
		return err
	}
	storage.Jobs.MakeSignJob(app.GetId(), userBundleId, profile.GetId())
	if err := config.Current.Builder.Trigger(); err != nil {
		return err
	}
	if err := app.SetWorkflowUrl(config.Current.Builder.GetStatusUrl()); err != nil {
		return err
	}
	isAccount, err := profile.IsAccount()
	if err != nil {
		return err
	}
	if isAccount {
		return c.Redirect(302, fmt.Sprintf("/apps/%s/2fa", app.GetId()))
	} else {
		return c.Redirect(302, "/")
	}
}

func renderIndex(c echo.Context) error {
	apps, err := storage.Apps.GetAll()
	if err != nil {
		return err
	}
	data := assets.IndexData{
		FormNames: formNames,
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
		bundleId, _ := app.GetBundleId()
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
		appTimeoutTime := modTime.Add(time.Duration(config.Current.SignTimeoutMins) * time.Minute)
		status := assets.AppStatusFailed
		if isSigned {
			status = assets.AppStatusSigned
		} else if time.Now().Before(appTimeoutTime) {
			status = assets.AppStatusProcessing
		}

		data.Apps = append(data.Apps, assets.App{
			Id:          app.GetId(),
			Status:      status,
			Name:        name,
			ModTime:     modTime.Format(time.RFC822),
			WorkflowUrl: workflowUrl,
			ProfileName: profileName,
			BundleId:    bundleId,
			ManifestUrl: util.JoinUrlsPanic(config.Current.ServerUrl, "apps", app.GetId(), "manifest"),
			DownloadUrl: util.JoinUrlsPanic(config.Current.ServerUrl, "apps", app.GetId(), "signed"),
			DeleteUrl:   util.JoinUrlsPanic(config.Current.ServerUrl, "apps", app.GetId(), "delete"),
		})
	}
	profiles, err := storage.Profiles.GetAll()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		name, err := profile.GetName()
		if err != nil {
			return err
		}
		isAccount, err := profile.IsAccount()
		if err != nil {
			return err
		}
		data.Profiles = append(data.Profiles, assets.Profile{
			Id:        profile.GetId(),
			Name:      name,
			IsAccount: isAccount,
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
