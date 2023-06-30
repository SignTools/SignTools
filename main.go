package main

import (
	"SignTools/src/assets"
	"SignTools/src/builders"
	"SignTools/src/config"
	"SignTools/src/storage"
	"SignTools/src/tunnel"
	"SignTools/src/util"
	"archive/tar"
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
	"github.com/tus/tusd/pkg/filestore"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/ziflex/lecho/v2"
	htmlTemplate "html/template"
	"io"
	log3 "log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	textTemplate "text/template"
	"time"
)

var formNames = assets.FormNames{
	FormFileId:          "file_id",
	FormFileUrl:         "file_url",
	FormTweakIds:        "tweak_ids",
	FormProfileId:       "profile_id",
	FormBuilderId:       "builder_id",
	FormAppDebug:        "app_debug",
	FormAllDevices:      "all_devices",
	FormMac:             "mac",
	FormFileShare:       "file_share",
	FormToken:           "token",
	FormId:              "id",
	FormIdOriginal:      "id_original",
	FormIdProv:          "id_prov",
	FormIdCustom:        "id_custom",
	FormIdCustomText:    "id_custom_text",
	FormIdEncode:        "id_encode",
	FormIdPatch:         "id_patch",
	FormIdForceOriginal: "id_force_original",
	FormBundleName:      "bundle_name",
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
	if err := os.MkdirAll(config.Current.SaveDir, 0700); err != nil {
		log.Fatal().Err(err).Send()
	}

	interval := time.Duration(config.Current.CleanupIntervalMins) * time.Minute
	timeout := time.Duration(config.Current.SignTimeoutMins) * time.Minute
	go func() {
		for range time.Tick(interval) {
			storage.Jobs.Cleanup(timeout)
			storage.Uploads.Cleanup(timeout)
		}
	}()

	log.Info().Msg("setting builder secrets")
	for _, builder := range config.Current.Builder {
		if err := setBuilderSecrets(builder); err != nil {
			log.Fatal().Err(err).Send()
		}
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
	getAndHead(e, "/apps/:id/signed", appResolver(getSignedApp), appResolver(getSignedApp))
	getAndHead(e, "/apps/:id/tweaks", appResolver(getTweaks), appResolver(getEmpty200App))
	getAndHead(e, "/apps/:id/unsigned", appResolver(getUnsignedApp), appResolver(getUnsignedApp))
	e.GET("/apps/:id/install", appResolver(renderInstall))
	e.GET("/apps/:id/manifest", appResolver(getManifest))
	e.GET("/apps/:id/resign", appResolver(resignApp), basicAuth)
	e.GET("/apps/:id/delete", appResolver(deleteApp), basicAuth)
	e.GET("/apps/:id/rename", appResolver(renderRenameApp), basicAuth)
	e.POST("/apps/:id/rename", appResolver(renameApp), basicAuth)
	e.GET("/apps/:id/2fa", appResolver(render2FAPage), basicAuth)
	e.POST("/apps/:id/2fa", appResolver(set2FA), basicAuth)
	getAndHead(e, "/jobs", getLastJob, getEmpty200, workflowKeyAuth)
	e.GET("/jobs/:id/2fa", jobResolver(get2FA), workflowKeyAuth)
	e.POST("/jobs/:id/signed", jobResolver(uploadSignedApp), workflowKeyAuth)
	getAndHead(e, "/jobs/:id/unsigned", jobResolver(getUnsignedAppJob), jobResolver(getUnsignedAppJob), workflowKeyAuth)
	e.GET("/jobs/:id/fail", jobResolver(failJob), workflowKeyAuth)

	if err := addTusHandlers(e, map[string]echo.MiddlewareFunc{
		"/tus/":          basicAuth,
		"/jobs/:id/tus/": workflowKeyAuth,
	}); err != nil {
		log.Fatal().Err(err).Send()
	}

	log.Fatal().Err(e.Start(fmt.Sprintf("%s:%d", host, port))).Send()
}

func getAndHead(e *echo.Echo, path string, getHandler func(c echo.Context) error, headHandler func(c echo.Context) error, m ...echo.MiddlewareFunc) {
	e.GET(path, getHandler, m...)
	e.HEAD(path, headHandler, m...)
}

func renderInstall(c echo.Context, app storage.App) error {
	usingManifestProxy := false
	baseUrl := getBaseUrl(c)
	manifestUrl := ""
	var err error
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
		name, err := app.GetString(storage.AppName)
		if err != nil {
			logErrApp(err, app).Msg("get name")
		}
		bundleId, _ := app.GetString(storage.AppBundleId)

		query := url.Values{
			"ipa":   []string{downloadFullUrl},
			"title": []string{name},
			"id":    []string{bundleId},
		}
		proxyUrl.RawQuery = query.Encode()
		manifestUrl = proxyUrl.String()
	}
	if usingManifestProxy {
		log.Warn().Str("base_url", getBaseUrl(c)).Msg("using OTA manifest proxy, installation may not work")
	}
	appName, err := app.GetString(storage.AppName)
	if err != nil {
		return err
	}
	data := assets.InstallData{
		ManifestUrl: manifestUrl,
		AppName:     appName,
	}
	t, err := htmlTemplate.New("").Parse(assets.InstallHtml)
	if err != nil {
		return err
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.HTMLBlob(200, result.Bytes())
}

type LogToZeroLog struct {
	Context string
}

func (l *LogToZeroLog) Write(p []byte) (n int, err error) {
	log.Info().Str("data", strings.TrimSpace(string(p))).Msg(l.Context)
	return len(p), nil
}

func addTusHandlers(e *echo.Echo, uploadEndpoints map[string]echo.MiddlewareFunc) error {
	store := filestore.FileStore{
		Path: storage.GetUploadsPath(),
	}
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	logToZeroLog := LogToZeroLog{Context: "tusd"}
	logger := log3.New(&logToZeroLog, "", 0)
	handler, err := tusd.NewUnroutedHandler(tusd.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
		UseRelativeUrls:       true,
		Logger:                logger,
	})
	//TODO: Apply tus middleware
	go func() {
		for {
			event := <-handler.CompleteUploads
			storage.Uploads.Add(event.Upload.ID)
		}
	}()
	if err != nil {
		return err
	}
	for prefix, middlewareFunc := range uploadEndpoints {
		e.POST(prefix, func(c echo.Context) error {
			handler.PostFile(c.Response().Writer, c.Request())
			return nil
		}, middlewareFunc)
	}
	e.HEAD("/files/:file_id", func(c echo.Context) error {
		handler.HeadFile(c.Response().Writer, c.Request())
		return nil
	})
	e.PATCH("/files/:file_id", func(c echo.Context) error {
		handler.PatchFile(c.Response().Writer, c.Request())
		return nil
	})
	return nil
}

func getTweaks(c echo.Context, app storage.App) error {
	tweaks, err := app.ReadDir(storage.TweaksDir)
	if os.IsNotExist(err) {
		return c.NoContent(404)
	} else if err != nil {
		return err
	}
	if len(tweaks) < 1 {
		return c.NoContent(404)
	}
	appName, err := app.GetString(storage.AppName)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type", mime.TypeByExtension(".tar"))
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-tweaks.tar"`, appName))
	writer := tar.NewWriter(c.Response().Writer)
	defer writer.Close()
	for _, tweak := range tweaks {
		tweakPath := storage.FSName(filepath.Join(string(storage.TweaksDir), tweak.Name()))
		file, err := app.GetFile(tweakPath)
		if err != nil {
			return err
		}
		defer file.Close()
		stat, err := app.Stat(tweakPath)
		if err != nil {
			return err
		}
		if err := writer.WriteHeader(&tar.Header{
			Name: tweak.Name(),
			Mode: 0600,
			Size: stat.Size(),
		}); err != nil {
			return err
		}
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
	}
	return nil
}

func renderRenameApp(c echo.Context, app storage.App) error {
	appName, err := app.GetString(storage.AppName)
	if err != nil {
		return err
	}
	data := assets.RenameData{AppName: appName}
	t, err := htmlTemplate.New("").Parse(assets.RenameHtml)
	if err != nil {
		return err
	}
	var result bytes.Buffer
	if err := t.Execute(&result, data); err != nil {
		return err
	}
	return c.HTMLBlob(200, result.Bytes())
}

func renameApp(c echo.Context, app storage.App) error {
	if err := app.SetString(storage.AppName, c.FormValue("name")); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func failJob(c echo.Context, job *storage.ReturnJob) error {
	if !storage.Jobs.DeleteById(job.Id) {
		return errors.New("unable to delete return job " + job.Id)
	}
	return c.NoContent(200)
}

func setBuilderSecrets(builder builders.Builder) error {
	return builder.SetSecrets(map[string]string{
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
	fileId := c.FormValue(formNames.FormFileId)
	upload, ok := storage.Uploads.Get(fileId)
	if !ok {
		return errors.New("no app upload file with id " + fileId)
	}
	defer storage.Uploads.Delete(fileId)
	file, err := upload.GetData()
	if err != nil {
		return err
	}
	defer file.Close()
	if err := app.SetFile(storage.AppSignedFile, file); err != nil {
		return err
	}
	if err := app.SetString(storage.AppBundleId, c.FormValue("bundle_id")); err != nil {
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
	appName, err := app.GetString(storage.AppName)
	if err != nil {
		return nil, err
	}
	bundleId, err := app.GetString(storage.AppBundleId)
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
	file, err := app.GetFile(storage.AppSignedFile)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeFileResponse(c, file, app); err != nil {
		return err
	}
	return nil
}

func getEmpty200(c echo.Context) error {
	return c.NoContent(200)
}

func getEmpty200App(c echo.Context, app storage.App) error {
	return c.NoContent(200)
}

func getUnsignedAppJob(c echo.Context, job *storage.ReturnJob) error {
	app, ok := storage.Apps.Get(job.AppId)
	if !ok {
		return c.NoContent(404)
	}
	file, err := app.GetFile(storage.AppUnsignedFile)
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
	file, err := app.GetFile(storage.AppUnsignedFile)
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
	name, err := app.GetString(storage.AppName)
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
	builderId := c.FormValue(formNames.FormBuilderId)
	builder, ok := config.Current.Builder[builderId]
	if !ok {
		return errors.New("no builder with id " + builderId)
	}

	var file io.ReadCloser
	var fileName string
	fileId := c.FormValue(formNames.FormFileId)
	fileUrl := c.FormValue(formNames.FormFileUrl)
	if fileUrl != "" {
		resp, err := http.Get(fileUrl)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
			return c.String(400, "Failed to download app from url: "+err.Error())
		}
		file = resp.Body
		defer file.Close()
		fileName = filepath.Base(fileUrl)
	} else if app, ok := storage.Apps.Get(fileId); ok {
		readonlyFile, err := app.GetFile(storage.AppUnsignedFile)
		if err != nil {
			return err
		}
		file = readonlyFile
		defer file.Close()
		fileName, err = app.GetString(storage.AppName)
		if err != nil {
			return err
		}
	} else if upload, ok := storage.Uploads.Get(fileId); ok {
		defer storage.Uploads.Delete(fileId)
		readonlyFile, err := upload.GetData()
		if err != nil {
			return err
		}
		file = readonlyFile
		defer file.Close()
		info, err := upload.GetInfo()
		if err != nil {
			return err
		}
		fileName = info.MetaData["filename"]
	} else {
		return errors.New("no app upload file with id " + fileId)
	}

	signArgs := ""
	if c.FormValue(formNames.FormAllDevices) != "" {
		signArgs += " -a"
	}
	if c.FormValue(formNames.FormMac) != "" {
		signArgs += " -m"
	}
	if c.FormValue(formNames.FormAppDebug) != "" {
		signArgs += " -d"
	}
	if c.FormValue(formNames.FormFileShare) != "" {
		signArgs += " -s"
	}
	if c.FormValue(formNames.FormIdEncode) != "" {
		signArgs += " -e"
	}
	if c.FormValue(formNames.FormIdForceOriginal) != "" {
		signArgs += " -o"
	}
	if c.FormValue(formNames.FormIdPatch) != "" {
		signArgs += " -p"
	}
	idType := c.FormValue(formNames.FormId)
	userBundleId := c.FormValue(formNames.FormIdCustomText)
	if idType == formNames.FormIdProv {
		signArgs += " -n"
	} else if idType == formNames.FormIdCustom {
		signArgs += " -b " + userBundleId
	}
	bundleName := c.FormValue(formNames.FormBundleName)
	if bundleName != "" {
		fileName = fmt.Sprintf("%s (%s)%s",
			strings.TrimSuffix(fileName, filepath.Ext(fileName)), bundleName, filepath.Ext(fileName))
	}
	tweakMap := map[string]io.Reader{}
	tweakIds := c.FormValue(formNames.FormTweakIds)
	if tweakIds != "" {
		for _, tweakId := range strings.Split(tweakIds, ",") {
			tweak, ok := storage.Uploads.Get(tweakId)
			if !ok {
				return errors.New("no tweak upload file with id " + fileId)
			}
			defer storage.Uploads.Delete(tweakId)
			readonlyFile, err := tweak.GetData()
			if err != nil {
				return err
			}
			defer readonlyFile.Close()
			info, err := tweak.GetInfo()
			if err != nil {
				return err
			}
			tweakMap[info.MetaData["filename"]] = readonlyFile
		}
	}
	app, err := storage.Apps.New(file, fileName, profile, signArgs, userBundleId, builderId, tweakMap)
	if err != nil {
		return err
	}
	if bundleName != "" {
		if err := app.SetString(storage.AppBundleName, bundleName); err != nil {
			return err
		}
	}
	if err := startSign(app, builder); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func resignApp(c echo.Context, app storage.App) error {
	builderId, err := app.GetString(storage.AppBuilderId)
	if err != nil {
		return err
	}
	builder, ok := config.Current.Builder[builderId]
	if !ok {
		return errors.New("no builder with id " + builderId)
	}
	if err := app.RemoveFile(storage.AppSignedFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := app.ResetModTime(); err != nil {
		return err
	}
	if err := startSign(app, builder); err != nil {
		return err
	}
	return c.Redirect(302, "/")
}

func startSign(app storage.App, builder builders.Builder) error {
	profileId, err := app.GetString(storage.AppProfileId)
	if err != nil {
		return err
	}
	storage.Jobs.MakeSignJob(app.GetId(), profileId)
	if err := setBuilderSecrets(builder); err != nil {
		return err
	}
	if err := builder.Trigger(); err != nil {
		return err
	}
	statusUrl, err := builder.GetStatusUrl()
	if err != nil {
		return err
	}
	if err := app.SetString(storage.AppWorkflowUrl, statusUrl); err != nil {
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
	for _, app := range apps {
		isSigned, err := app.IsSigned()
		if err != nil {
			return errors.WithMessage(err, "get is signed")
		}
		modTime, err := app.GetModTime()
		if err != nil {
			return errors.WithMessage(err, "get mod time")
		}
		name, err := app.GetString(storage.AppName)
		if err != nil {
			logErrApp(err, app).Msg("get name")
		}
		workflowUrl, err := app.GetString(storage.AppWorkflowUrl)
		if err != nil {
			logErrApp(err, app).Msg("get workflow url")
		}
		bundleId, _ := app.GetString(storage.AppBundleId)
		profileId, err := app.GetString(storage.AppProfileId)
		if err != nil {
			logErrApp(err, app).Msg("get profile id")
		}
		var profileName string
		if profile, ok := storage.Profiles.GetById(profileId); ok {
			profileName, err = profile.GetString(storage.ProfileName)
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

		tweakCount := 0
		if tweaks, err := app.ReadDir(storage.TweaksDir); err == nil {
			tweakCount = len(tweaks)
		} else if !os.IsNotExist(err) {
			return err
		}

		data.Apps = append(data.Apps, assets.App{
			Id:                  app.GetId(),
			Status:              status,
			Name:                name,
			ModTime:             modTime.Format(time.RFC822),
			WorkflowUrl:         workflowUrl,
			ProfileName:         profileName,
			BundleId:            bundleId,
			InstallUrl:          path.Join("/apps", app.GetId(), "install"),
			DownloadSignedUrl:   path.Join("/apps", app.GetId(), "signed"),
			DownloadUnsignedUrl: path.Join("/apps", app.GetId(), "unsigned"),
			DownloadTweaksUrl:   path.Join("/apps", app.GetId(), "tweaks"),
			TwoFactorUrl:        path.Join("/apps", app.GetId(), "2fa"),
			ResignUrl:           path.Join("/apps", app.GetId(), "resign"),
			DeleteUrl:           path.Join("/apps", app.GetId(), "delete"),
			RenameUrl:           path.Join("/apps", app.GetId(), "rename"),
			TweakCount:          tweakCount,
		})
	}
	profiles, err := storage.Profiles.GetAll()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		name, err := profile.GetString(storage.ProfileName)
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
	for builderId := range config.Current.Builder {
		data.Builders = append(data.Builders, assets.Builder{
			Id:   builderId,
			Name: builderId,
		})
	}
	sort.Slice(data.Builders, func(i, j int) bool {
		name1 := data.Builders[i].Name
		name2 := data.Builders[j].Name
		return name1 < name2
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
