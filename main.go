package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log2 "github.com/labstack/gommon/log"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ziflex/lecho/v2"
	htmlTemplate "html/template"
	"io"
	"ios-signer-service/src/assets"
	"ios-signer-service/src/config"
	"ios-signer-service/src/storage"
	"ios-signer-service/src/tunnel"
	"ios-signer-service/src/util"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
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

func cleanupOld() error {
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
				log.Err(err).Str("id", app.GetId()).Msg("cleanup app")
			}
		}
	}
	for _, job := range storage.Jobs.GetAll() {
		if job.Ts.Add(time.Duration(config.Current.SignTimeoutMins) * time.Minute).Before(now) {
			if !storage.Jobs.DeleteById(job.Id) {
				log.Error().Str("id", job.Id).Msg("cleanup job")
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
	cloudflaredPort := flag.Uint64("cloudflared-port", 0, "cloudflared metrics port. "+
		"Used to automatically parse the server_url")
	logJson := flag.Bool("log-json", false, "If enabled, outputs logs in JSON instead of pretty printing them.")
	logLevel := flag.Uint("log-level", uint(zerolog.InfoLevel), "Logging level, 0 (debug) - 5 (panic).")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.Level(*logLevel))
	if !*logJson {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	config.Load(*configFile)
	storage.Load()
	switch {
	case *ngrokPort != 0:
		config.Current.ServerUrl = getPublicUrlFatal(&tunnel.Ngrok{Port: *ngrokPort, Proto: "https"})
	case *cloudflaredPort != 0:
		config.Current.ServerUrl = getPublicUrlFatal(&tunnel.Cloudflare{Port: *cloudflaredPort})
	}

	log.Info().Str("url", config.Current.ServerUrl).Msg("using server url")
	serve(*host, *port)
}

func getPublicUrlFatal(provider tunnel.Provider) string {
	log.Info().Msg("obtaining server url")
	serverUrl, err := tunnel.GetPublicUrl(provider, 15*time.Second)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	return serverUrl
}

func serve(host string, port uint64) {
	if err := os.MkdirAll(config.Current.SaveDir, 0777); err != nil {
		log.Fatal().Err(err).Send()
	}

	if config.Current.CleanupIntervalMins > 0 && config.Current.CleanupMins > 0 {
		go func() {
			for {
				if err := cleanupOld(); err != nil {
					log.Err(err).Msg("cleanup old")
				}
				time.Sleep(time.Duration(config.Current.CleanupIntervalMins) * time.Minute)
			}
		}()
	}

	log.Info().Msg("setting builder secrets")
	if err := setBuilderSecrets(); err != nil {
		log.Fatal().Err(err).Send()
	}

	e := echo.New()
	e.HideBanner = true
	logger := lecho.From(log.Logger, lecho.WithLevel(log2.INFO))
	e.Logger = logger
	e.Use(lecho.Middleware(lecho.Config{Logger: logger}))

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

	if config.Current.RedirectHttps {
		e.Pre(middleware.HTTPSRedirectWithConfig(middleware.RedirectConfig{
			Code: 302,
		}))
	}

	e.GET("/", renderIndex, basicAuth)
	e.GET("/favicon.png", getFavIcon, basicAuth)
	e.POST("/apps", uploadUnsignedApp, basicAuth)
	e.GET("/apps/:id/signed", appResolver(getSignedApp))
	e.GET("/apps/:id/manifest", appResolver(getManifest))
	e.GET("/apps/:id/restart", appResolver(restartSign), basicAuth)
	e.GET("/apps/:id/delete", appResolver(deleteApp), basicAuth)
	e.GET("/apps/:id/2fa", appResolver(render2FAPage), basicAuth)
	e.POST("/apps/:id/2fa", appResolver(set2FA), basicAuth)
	e.GET("/jobs", getLastJob, workflowKeyAuth)
	e.GET("/jobs/:id/2fa", jobResolver(get2FA), workflowKeyAuth)
	e.POST("/jobs/:id/signed", jobResolver(uploadSignedApp), workflowKeyAuth)
	e.GET("/jobs/:id/fail", jobResolver(failJob), workflowKeyAuth)

	log.Fatal().Err(e.Start(fmt.Sprintf("%s:%d", host, port))).Send()
}

func failJob(c echo.Context, job *storage.ReturnJob) error {
	if !storage.Jobs.DeleteById(job.Id) {
		return errors.New("unable to delete return job " + job.Id)
	}
	return c.NoContent(200)
}

func setBuilderSecrets() error {
	return config.Current.Builder.SetSecrets(map[string]string{
		"SECRET_KEY": config.Current.BuilderKey,
		"SECRET_URL": config.Current.ServerUrl,
	})
}

func getFavIcon(c echo.Context) error {
	http.ServeContent(c.Response(), c.Request(), assets.FavIconStat.Name(), assets.FavIconStat.ModTime(), bytes.NewReader(assets.FavIconBytes))
	return nil
}

func uploadSignedApp(c echo.Context, job *storage.ReturnJob) error {
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
	if !storage.Jobs.DeleteById(job.Id) {
		return errors.New("unable to delete return job " + job.Id)
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
	manifestBytes, err := makeManifest(getBaseUrl(c), app)
	if err != nil {
		return err
	}
	return c.Blob(200, "text/plain", manifestBytes)
}

func getBaseUrl(c echo.Context) string {
	serverUrl := url.URL{
		Scheme: c.Scheme(),
		Host:   c.Request().Host,
	}
	return serverUrl.String()
}

func makeManifest(baseUrl string, app storage.App) ([]byte, error) {
	t, err := textTemplate.New("").Funcs(
		textTemplate.FuncMap{"escape": func(text string) (string, error) {
			return escapeXML(text)
		}},
	).Parse(assets.ManifestPlist)
	if err != nil {
		return nil, err
	}
	appName, err := app.GetName()
	if err != nil {
		return nil, err
	}
	bundleId, err := app.GetBundleId()
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.JoinUrls(baseUrl, "/apps", app.GetId(), "signed")
	if err != nil {
		return nil, err
	}
	data := assets.ManifestData{
		DownloadUrl: downloadUrl,
		BundleId:    bundleId,
		Title:       appName,
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func escapeXML(str string) (string, error) {
	buff := bytes.NewBuffer(nil)
	if err := xml.EscapeText(buff, []byte(str)); err != nil {
		return "", err
	}
	return buff.String(), nil
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
	app, err := storage.Apps.New(file, header.Filename, profile, signArgs, userBundleId)
	if err != nil {
		return err
	}
	if err := startSign(app); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func restartSign(c echo.Context, app storage.App) error {
	if err := startSign(app); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func startSign(app storage.App) error {
	if err := app.ResetModTime(); err != nil {
		return err
	}
	profileId, err := app.GetProfileId()
	if err != nil {
		return err
	}
	storage.Jobs.MakeSignJob(app.GetId(), profileId)
	if err := setBuilderSecrets(); err != nil {
		return err
	}
	if err := config.Current.Builder.Trigger(); err != nil {
		return err
	}
	statusUrl, err := config.Current.Builder.GetStatusUrl()
	if err != nil {
		return err
	}
	if err := app.SetWorkflowUrl(statusUrl); err != nil {
		return err
	}
	return nil
}

func logErrApp(err error, app storage.App) *zerolog.Event {
	return log.Err(err).Str("app_id", app.GetId())
}

func renderIndex(c echo.Context) error {
	apps, err := storage.Apps.GetAll()
	if err != nil {
		return err
	}
	data := assets.IndexData{
		FormNames: formNames,
	}
	usingManifestProxy := false
	for _, app := range apps {
		isSigned, err := app.IsSigned()
		if err != nil {
			return errors.WithMessage(err, "get is signed")
		}
		modTime, err := app.GetModTime()
		if err != nil {
			return errors.WithMessage(err, "get mod time")
		}
		name, err := app.GetName()
		if err != nil {
			logErrApp(err, app).Msg("get name")
		}
		workflowUrl, err := app.GetWorkflowUrl()
		if err != nil {
			logErrApp(err, app).Msg("get workflow url")
		}
		bundleId, _ := app.GetBundleId()
		profileId, err := app.GetProfileId()
		if err != nil {
			logErrApp(err, app).Msg("get profile id")
		}
		var profileName string
		if profile, ok := storage.Profiles.GetById(profileId); ok {
			profileName, err = profile.GetName()
			if err != nil {
				logErrApp(err, app).Msg("get profile name")
			}
		} else {
			logErrApp(err, app).Msg("get profile")
			profileName = "unknown"
		}
		jobPending, jobExists := storage.Jobs.GetStatusByAppId(app.GetId())
		var status int
		if isSigned {
			status = assets.AppStatusSigned
		} else if jobPending {
			status = assets.AppStatusWaiting
		} else if jobExists {
			status = assets.AppStatusProcessing
		} else {
			status = assets.AppStatusFailed
		}
		baseUrl := getBaseUrl(c)
		manifestUrl := ""
		if strings.HasPrefix(baseUrl, "https") {
			// must be a full URL
			manifestUrl, err = util.JoinUrls(baseUrl, "/apps", app.GetId(), "manifest")
			if err != nil {
				return errors.WithMessage(err, "build manifest url")
			}
		} else {
			usingManifestProxy = true
			downloadFullUrl, err := util.JoinUrls(baseUrl, "/apps", app.GetId(), "signed")
			if err != nil {
				return errors.WithMessage(err, "build download url")
			}
			proxyUrl := url.URL{
				Scheme: "https",
				Host:   "ota.signtools.workers.dev",
				Path:   "/v1",
			}
			query := url.Values{
				"ipa":   []string{downloadFullUrl},
				"title": []string{name},
				"id":    []string{bundleId},
			}
			proxyUrl.RawQuery = query.Encode()
			manifestUrl = proxyUrl.String()
		}

		data.Apps = append(data.Apps, assets.App{
			Id:           app.GetId(),
			Status:       status,
			Name:         name,
			ModTime:      modTime.Format(time.RFC822),
			WorkflowUrl:  workflowUrl,
			ProfileName:  profileName,
			BundleId:     bundleId,
			ManifestUrl:  manifestUrl,
			DownloadUrl:  path.Join("/apps", app.GetId(), "signed"),
			TwoFactorUrl: path.Join("/apps", app.GetId(), "2fa"),
			RestartUrl:   path.Join("/apps", app.GetId(), "restart"),
			DeleteUrl:    path.Join("/apps", app.GetId(), "delete"),
		})
	}
	if usingManifestProxy {
		log.Warn().Str("base_url", getBaseUrl(c)).Msg("using OTA manifest proxy, installation may not work")
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
