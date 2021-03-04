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
	"ios-signer-service/assets"
	"ios-signer-service/config"
	"ios-signer-service/storage"
	"ios-signer-service/util"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	textTemplate "text/template"
	"time"
)

var (
	formFile        = "file"
	formProfileName = "profile_name"
	formAllDevices  = "all_devices"
	formAppDebug    = "app_debug"
	formFileShare   = "file_share"
	formAlignAppId  = "align_app_id"
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
		if modTime.Add(time.Duration(config.Current.CleanupMins) * time.Minute).Before(now) {
			if err := storage.Apps.Delete(app.GetId()); err != nil {
				return err
			}
		}
	}
	return nil
}

var workflowTransport = http.Transport{
	// Cloned http.DefaultTransport.
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}
var workflowClient = http.Client{Transport: &workflowTransport}

func main() {
	port := flag.Uint64("port", 8080, "Listen port")
	configFile := flag.String("config", "signer-cfg.yml", "Configuration file")
	flag.Parse()

	config.Load(*configFile)
	storage.Load()
	serve(*port)
}

func serve(port uint64) {
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

	if !config.Current.Workflow.Trigger.AttemptHTTP2 {
		workflowTransport.ForceAttemptHTTP2 = false
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
		return s == config.Current.Workflow.Key, nil
	})

	e.GET("/", index, basicAuth)
	e.POST("/apps", uploadUnsignedApp, basicAuth)
	e.GET("/apps/:id/signed", appResolver(getSignedApp))
	e.GET("/apps/:id/manifest", appResolver(getManifest))
	e.GET("/apps/:id/delete", appResolver(deleteApp), basicAuth)
	e.GET("/jobs", getLastJob, workflowKeyAuth)
	e.POST("/jobs/:id", uploadJobResult, workflowKeyAuth)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}

func uploadJobResult(c echo.Context) error {
	appId, ok := storage.Jobs.ResolveReturnJob(c.Param("id"))
	if !ok {
		return c.NoContent(404)
	}
	app, ok := storage.Apps.Get(appId)
	if !ok {
		return errors.New(fmt.Sprintf("return job appid %s not resolved", appId))
	}
	header, err := c.FormFile(formFile)
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

func getLastJob(c echo.Context) error {
	if err := storage.Jobs.WriteLastJob(c.Response()); errors.Is(err, storage.ErrNotFound) {
		return c.NoContent(404)
	} else if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type", mime.TypeByExtension(".tar"))
	return c.NoContent(200)
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
	profileName := c.FormValue(formProfileName)
	profile, ok := storage.Profiles.GetByName(profileName)
	if !ok {
		return errors.New("no profile with id " + profileName)
	}
	header, err := c.FormFile(formFile)
	if err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	signArgs := ""
	if c.FormValue(formAllDevices) != "" {
		signArgs += " -a"
	}
	if c.FormValue(formAppDebug) != "" {
		signArgs += " -d"
	}
	if c.FormValue(formFileShare) != "" {
		signArgs += " -s"
	}
	if c.FormValue(formAlignAppId) != "" {
		signArgs += " -n"
	}
	app, err := storage.Apps.New(file, header.Filename, profile, signArgs)
	if err != nil {
		return err
	}
	storage.Jobs.MakeSignJob(app.GetId(), profile.GetId())
	if err := triggerWorkflow(); err != nil {
		return err
	}
	if err := app.SetWorkflowUrl(config.Current.Workflow.StatusUrl); err != nil {
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
		FormFile:        formFile,
		FormProfileName: formProfileName,
		FormAllDevices:  formAllDevices,
		FormAppDebug:    formAppDebug,
		FormFileShare:   formFileShare,
		FormAlignAppId:  formAlignAppId,
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

func triggerWorkflow() error {
	body := bytes.NewReader([]byte(config.Current.Workflow.Trigger.Body))
	request, err := http.NewRequest("POST", config.Current.Workflow.Trigger.Url, body)
	if err != nil {
		return errors.WithMessage(err, "new request")
	}
	for key, val := range config.Current.Workflow.Trigger.Headers {
		request.Header.Set(key, val)
	}
	response, err := workflowClient.Do(request)
	if err != nil {
		return errors.WithMessage(err, "do request")
	}
	if err := util.Check2xxCode(response.StatusCode); err != nil {
		return err
	}
	return nil
}
