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
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/galecore/xslog/xzerolog"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log2 "github.com/labstack/gommon/log"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"github.com/ziflex/lecho/v2"
	log3 "golang.org/x/exp/slog"
	htmlTemplate "html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	FormSavedTweakIds:   "saved_tweak_ids",
}

func main() {
	host := flag.String("host", "", "Listen host, empty for all")
	port := flag.Uint64("port", 8080, "Listen port")
	configFile := flag.String("config", "signer-cfg.yml", "Configuration file")
	ngrokHost := flag.String("ngrok-host", "", "Ngrok web interface host. "+
		"Used to automatically parse the server_url")
	cloudflaredHost := flag.String("cloudflared-host", "", "cloudflared metrics host. "+
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
	case *ngrokHost != "":
		config.Current.ServerUrl = getPublicUrlFatal(&tunnel.Ngrok{Host: *ngrokHost, Proto: "https"})
	case *cloudflaredHost != "":
		config.Current.ServerUrl = getPublicUrlFatal(&tunnel.Cloudflare{Host: *cloudflaredHost})
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
	e.GET("/events", streamEvents, basicAuth)
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
	e.GET("/tweaks", func(c echo.Context) error { return c.Redirect(302, "/#tweaks") }, basicAuth)
	e.POST("/tweaks", uploadTweaks, basicAuth)
	e.GET("/tweaks/:id/download", tweakResolver(downloadTweak), basicAuth)
	e.GET("/tweaks/:id/delete", tweakResolver(deleteTweak), basicAuth)
	e.GET("/tweaks/:id/rename", tweakResolver(renderRenameTweak), basicAuth)
	e.POST("/tweaks/:id/rename", tweakResolver(renameTweak), basicAuth)
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

// actionDone answers the page's fetch requests with 204, since live updates show the result.
func actionDone(c echo.Context, back string) error {
	if c.Request().Header.Get("X-Requested-With") != "" {
		return c.NoContent(http.StatusNoContent)
	}
	return c.Redirect(http.StatusFound, back)
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

// https://tus.github.io/tusd/advanced-topics/usage-package/
func addTusHandlers(e *echo.Echo, uploadEndpoints map[string]echo.MiddlewareFunc) error {
	uploadsPath := storage.GetUploadsPath()
	store := filestore.New(uploadsPath)
	locker := filelocker.New(uploadsPath)
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)
	logger := xzerolog.NewHandler(&log.Logger)
	handler, err := tusd.NewUnroutedHandler(tusd.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
		UseRelativeUrls:       true,
		Logger:                log3.New(logger),
	})
	go func() {
		for {
			event := <-handler.CompleteUploads
			storage.Uploads.Add(event.Upload.ID)
		}
	}()
	if err != nil {
		return err
	}
	tusMiddleware := echo.WrapMiddleware(handler.Middleware)
	stripMiddleware := echo.WrapMiddleware(func(h http.Handler) http.Handler {
		return http.StripPrefix("/files/", h)
	})
	for prefix, authMiddleware := range uploadEndpoints {
		// middlewares run in order, make sure auth goes first!
		e.POST(prefix, func(c echo.Context) error {
			handler.PostFile(c.Response().Writer, c.Request())
			return nil
		}, authMiddleware, tusMiddleware)
	}
	e.HEAD("/files/:file_id", func(c echo.Context) error {
		handler.HeadFile(c.Response().Writer, c.Request())
		return nil
	}, stripMiddleware, tusMiddleware)
	e.PATCH("/files/:file_id", func(c echo.Context) error {
		handler.PatchFile(c.Response().Writer, c.Request())
		return nil
	}, stripMiddleware, tusMiddleware)
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
	data := assets.RenameData{Kind: "App", Name: appName, BackUrl: "/"}
	return renderRename(c, data)
}

func renderRename(c echo.Context, data assets.RenameData) error {
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
	return actionDone(c, "/")
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

func tweakResolver(handler func(echo.Context, storage.Tweak) error) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		tweak, ok := storage.Tweaks.Get(id)
		if !ok {
			return c.NoContent(404)
		}
		return handler(c, tweak)
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
	return actionDone(c, "/")
}

func deleteApp(c echo.Context, app storage.App) error {
	if err := storage.Apps.Delete(app.GetId()); err != nil {
		return err
	}
	return actionDone(c, "/")
}

func getManifest(c echo.Context, app storage.App) error {
	manifestBytes, err := makeManifest(getBaseUrl(c), app)
	if err != nil {
		return err
	}
	return c.Blob(200, "text/plain", manifestBytes)
}

func getBaseUrl(c echo.Context) string {
	var host = ""
	if value := c.Request().Header.Get("X-Forwarded-Host"); value != "" {
		host = value
	} else {
		host = c.Request().Host
	}
	serverUrl := url.URL{
		Scheme: c.Scheme(),
		Host:   host,
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
	if err := writeFileResponse(c, file, app, storage.AppName); err != nil {
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
	if err := writeFileResponse(c, file, app, storage.AppName); err != nil {
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
	if err := writeFileResponse(c, file, app, storage.AppName); err != nil {
		return err
	}
	return nil
}

type fileOwner interface {
	GetString(storage.FSName) (string, error)
	GetModTime() (time.Time, error)
}

func writeFileResponse(c echo.Context, file io.ReadSeeker, owner fileOwner, nameField storage.FSName) error {
	name, err := owner.GetString(nameField)
	if err != nil {
		return err
	}
	//TODO: Should use the file's mod time, otherwise may tell client to use cached file even though it has changed
	modTime, err := owner.GetModTime()
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
		if err != nil {
			return c.String(400, "Failed to download app from url: "+err.Error())
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			resp.Body.Close()
			return c.String(400, "Failed to download app from url: "+resp.Status)
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
	formParams, err := c.FormParams()
	if err != nil {
		return err
	}
	for _, savedId := range formParams[formNames.FormSavedTweakIds] {
		tweak, ok := storage.Tweaks.Get(savedId)
		if !ok {
			return errors.New("no saved tweak with id " + savedId)
		}
		name, err := tweak.GetString(storage.TweakName)
		if err != nil {
			return err
		}
		readonlyFile, err := tweak.GetFile(storage.TweakFile)
		if err != nil {
			return err
		}
		defer readonlyFile.Close()
		tweakMap[name] = readonlyFile
	}
	tweakIds := c.FormValue(formNames.FormTweakIds)
	if tweakIds != "" {
		for tweakId := range strings.SplitSeq(tweakIds, ",") {
			tweak, ok := storage.Uploads.Get(tweakId)
			if !ok {
				return errors.New("no tweak upload file with id " + tweakId)
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
	return actionDone(c, "/")
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
	return actionDone(c, "/")
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

// indexTemplate's "apps", "tweaks" and "savedTweaks" blocks are also rendered alone by streamEvents.
var indexTemplate = htmlTemplate.Must(htmlTemplate.New("index").Parse(assets.IndexHtml))

func buildAppViews() ([]assets.App, error) {
	apps, err := storage.Apps.GetAll()
	if err != nil {
		return nil, err
	}
	var views []assets.App
	for _, app := range apps {
		isSigned, err := app.IsSigned()
		if err != nil {
			return nil, errors.WithMessage(err, "get is signed")
		}
		modTime, err := app.GetModTime()
		if err != nil {
			return nil, errors.WithMessage(err, "get mod time")
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
			return nil, err
		}

		views = append(views, assets.App{
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
	return views, nil
}

func buildLiveData() (assets.IndexData, error) {
	data := assets.IndexData{FormNames: formNames}
	var err error
	if data.Apps, err = buildAppViews(); err != nil {
		return data, err
	}
	if data.Tweaks, err = getTweakViews(); err != nil {
		return data, err
	}
	return data, nil
}

func renderIndex(c echo.Context) error {
	data, err := buildLiveData()
	if err != nil {
		return err
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
	var result bytes.Buffer
	if err := indexTemplate.Execute(&result, data); err != nil {
		return err
	}
	return c.HTMLBlob(200, result.Bytes())
}

type liveState struct {
	Apps        string `json:"apps"`
	Tweaks      string `json:"tweaks"`
	SavedTweaks string `json:"savedTweaks"`
}

// liveCache shares one render per change across all connected clients.
var liveCache struct {
	sync.Mutex
	version uint64
	payload []byte
}

func renderLiveState() ([]byte, error) {
	version := storage.Changes.Version()
	liveCache.Lock()
	defer liveCache.Unlock()
	if liveCache.payload != nil && liveCache.version == version {
		return liveCache.payload, nil
	}
	data, err := buildLiveData()
	if err != nil {
		return nil, err
	}
	render := func(name string) (string, error) {
		var buf bytes.Buffer
		err := indexTemplate.ExecuteTemplate(&buf, name, data)
		return buf.String(), errors.WithMessagef(err, "render %s", name)
	}
	var state liveState
	if state.Apps, err = render("apps"); err != nil {
		return nil, err
	}
	if state.Tweaks, err = render("tweaks"); err != nil {
		return nil, err
	}
	if state.SavedTweaks, err = render("savedTweaks"); err != nil {
		return nil, err
	}
	// the payload is mostly HTML, don't inflate it with \u003c escapes
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(state); err != nil {
		return nil, err
	}
	liveCache.version = version
	liveCache.payload = bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return liveCache.payload, nil
}

const sseHeartbeat = 25 * time.Second

func streamEvents(c echo.Context) error {
	// subscribe before the first render so no change can slip in between
	changes, cancel := storage.Changes.Subscribe()
	defer cancel()

	res := c.Response()
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)

	var lastSent []byte
	send := func() error {
		payload, err := renderLiveState()
		if err != nil {
			log.Err(err).Msg("render live state")
			return nil
		}
		if bytes.Equal(payload, lastSent) {
			return nil
		}
		lastSent = payload
		if _, err := fmt.Fprintf(res, "event: state\ndata: %s\n\n", payload); err != nil {
			return err
		}
		res.Flush()
		return nil
	}

	if _, err := fmt.Fprint(res, "retry: 3000\n\n"); err != nil {
		return nil
	}
	if err := send(); err != nil {
		return nil
	}
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()
	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-changes:
			if err := send(); err != nil {
				return nil
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(res, ": ping\n\n"); err != nil {
				return nil
			}
			res.Flush()
		}
	}
}

// saveUploadAsTweak moves a completed tus upload into the tweak library.
func saveUploadAsTweak(uploadId string) (storage.Tweak, error) {
	upload, ok := storage.Uploads.Get(uploadId)
	if !ok {
		return nil, errors.New("no tweak upload file with id " + uploadId)
	}
	defer storage.Uploads.Delete(uploadId)
	info, err := upload.GetInfo()
	if err != nil {
		return nil, err
	}
	data, err := upload.GetData()
	if err != nil {
		return nil, err
	}
	defer data.Close()
	return storage.Tweaks.New(data, info.MetaData["filename"])
}

func uploadTweaks(c echo.Context) error {
	tweakIds := c.FormValue(formNames.FormTweakIds)
	if tweakIds != "" {
		for tweakId := range strings.SplitSeq(tweakIds, ",") {
			if _, err := saveUploadAsTweak(tweakId); err != nil {
				return err
			}
		}
	}
	return actionDone(c, "/#tweaks")
}

func downloadTweak(c echo.Context, tweak storage.Tweak) error {
	file, err := tweak.GetFile(storage.TweakFile)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeFileResponse(c, file, tweak, storage.TweakName)
}

func renderRenameTweak(c echo.Context, tweak storage.Tweak) error {
	name, err := tweak.GetString(storage.TweakName)
	if err != nil {
		return err
	}
	data := assets.RenameData{Kind: "Tweak", Name: name, BackUrl: "/#tweaks"}
	if ext := filepath.Ext(name); ext != "" {
		data.Hint = fmt.Sprintf("The extension (%s) is kept so the tweak is still injected correctly.", ext)
	}
	return renderRename(c, data)
}

// renameTweak renames a library tweak, keeping its original file extension since
// the signer uses it to decide how to inject the tweak.
func renameTweak(c echo.Context, tweak storage.Tweak) error {
	oldName, err := tweak.GetString(storage.TweakName)
	if err != nil {
		return err
	}
	newName, err := storage.SanitizeTweakName(c.FormValue("name"))
	if err != nil {
		return c.String(400, err.Error())
	}
	if ext := filepath.Ext(oldName); !strings.EqualFold(filepath.Ext(newName), ext) {
		newName += ext
	}
	if err := tweak.SetString(storage.TweakName, newName); err != nil {
		return err
	}
	return actionDone(c, "/#tweaks")
}

func deleteTweak(c echo.Context, tweak storage.Tweak) error {
	if err := storage.Tweaks.Delete(tweak.GetId()); err != nil {
		return err
	}
	return actionDone(c, "/#tweaks")
}

func getTweakViews() ([]assets.Tweak, error) {
	tweaks, err := storage.Tweaks.GetAll()
	if err != nil {
		return nil, err
	}
	var views []assets.Tweak
	for _, tweak := range tweaks {
		name, err := tweak.GetString(storage.TweakName)
		if err != nil {
			log.Err(err).Str("tweak_id", tweak.GetId()).Msg("get tweak name")
			continue
		}
		size := ""
		if stat, err := tweak.Stat(storage.TweakFile); err == nil {
			size = formatSize(stat.Size())
		}
		modTime := ""
		if t, err := tweak.GetModTime(); err == nil {
			modTime = t.Format(time.RFC822)
		}
		views = append(views, assets.Tweak{
			Id:          tweak.GetId(),
			Name:        name,
			Size:        size,
			ModTime:     modTime,
			DownloadUrl: path.Join("/tweaks", tweak.GetId(), "download"),
			DeleteUrl:   path.Join("/tweaks", tweak.GetId(), "delete"),
			RenameUrl:   path.Join("/tweaks", tweak.GetId(), "rename"),
		})
	}
	return views, nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
