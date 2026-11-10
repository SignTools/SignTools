package main

import (
	"SignTools/src/builders"
	"SignTools/src/config"
	"SignTools/src/storage"
	"SignTools/src/util"
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/eventials/go-tus"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/ziflex/lecho/v2"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	workflowKey     = uuid.NewString()
	builderKey      = uuid.NewString()
	saveDir         = ""
	profileId       = uuid.NewString()
	profileCert     []byte
	profileName     = uuid.NewString()
	profileCertPass = "1234"
	profileProv     = uuid.NewString()
	unsignedData    = uuid.NewString()
	signedData      = uuid.NewString()
)

var listenHost = "localhost"
var servePort = uint64(8098)
var serveAddress = fmt.Sprintf("http://%s:%d", listenHost, servePort)
var workflowPort = uint64(8099)
var workflowAddress = fmt.Sprintf("http://%s:%d", listenHost, workflowPort)

func TestMain(m *testing.M) {
	var err error
	saveDir, err = os.MkdirTemp(".", "data-test")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	defer func() {
		if err := os.RemoveAll(saveDir); err != nil {
			log.Fatal().Err(err).Send()
		}
	}()

	profileDir := filepath.Join(saveDir, "profiles", profileId)
	if err := os.MkdirAll(profileDir, os.ModePerm); err != nil {
		log.Fatal().Err(err).Send()
	}
	profileCert, err = ioutil.ReadFile("cert-test.p12")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	contentMap := map[string][]byte{
		"cert.p12":             profileCert,
		"cert_pass.txt":        []byte(profileCertPass),
		"name.txt":             []byte(profileName),
		"prov.mobileprovision": []byte(profileProv),
	}
	for key, val := range contentMap {
		if err := ioutil.WriteFile(filepath.Join(profileDir, key), val, os.ModePerm); err != nil {
			log.Fatal().Err(err).Send()
		}
	}

	config.Current = config.Config{
		Builder: map[string]builders.Builder{
			"selfhosted": builders.MakeSelfHosted(&builders.SelfHostedData{
				Enable: true,
				Url:    workflowAddress,
				Key:    workflowKey,
			}),
		},
		File: &config.File{
			ServerUrl:           serveAddress,
			SaveDir:             saveDir,
			CleanupIntervalMins: 0,
		},
		BuilderKey: builderKey,
		EnvProfile: &config.EnvProfile{},
	}
	storage.Load()

	go startWorkflowServer(listenHost, workflowPort)
	if err := util.WaitForServer(workflowAddress, 5*time.Second); err != nil {
		log.Fatal().Err(err).Send()
	}

	go serve(listenHost, servePort)
	if err := util.WaitForServer(serveAddress, 5*time.Second); err != nil {
		log.Fatal().Err(err).Send()
	}
	m.Run()
}

var triggerHit = false
var secretsHit = false

func startWorkflowServer(host string, port uint64) {
	e := echo.New()
	e.HideBanner = true
	logger := lecho.From(log.Logger)
	e.Logger = logger
	e.Use(lecho.Middleware(lecho.Config{Logger: logger}))

	keyAuth := middleware.KeyAuth(func(s string, c echo.Context) (bool, error) {
		return s == workflowKey, nil
	})

	eg := e.Group("", keyAuth)

	eg.POST("/secrets", func(c echo.Context) error {
		secretsHit = true
		params, err := c.FormParams()
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		for key, val := range params {
			switch key {
			case "SECRET_KEY":
				if val[0] != builderKey {
					log.Fatal().Msg("bad key")
				}
			case "SECRET_URL":
				if val[0] != config.Current.File.ServerUrl {
					log.Fatal().Msg("bad url")
				}
			default:
				log.Fatal().Msg("unknown secret")
			}
		}
		return c.NoContent(200)
	})

	eg.POST("/trigger", func(c echo.Context) error {
		triggerHit = true
		return c.NoContent(200)
	})

	log.Fatal().Err(e.Start(fmt.Sprintf("%s:%d", host, port))).Send()
}

func TestIntegration(t *testing.T) {
	uploadUnsigned(t)
	assert.True(t, triggerHit)
	assert.True(t, secretsHit)
	validateFile(t, unsignedData, func(app storage.App) (storage.ReadonlyFile, error) {
		return app.GetFile(storage.AppUnsignedFile)
	})
	returnId := takeJob(t)
	uploadSignedFile(t, returnId)
	validateFile(t, signedData, func(app storage.App) (storage.ReadonlyFile, error) {
		return app.GetFile(storage.AppSignedFile)
	})
	validateManifest(t)
}

func TestTweakLibrary(t *testing.T) {
	tweakName := "test.dylib"
	tweakData := uuid.NewString()

	// upload a tweak into the library
	uploadId := tusUploadNamed(t, []byte(tweakData), tweakName)
	postForm(t, "/tweaks", url.Values{formNames.FormTweakIds: {uploadId}})
	tweaks, err := storage.Tweaks.GetAll()
	assert.NoError(t, err)
	assert.Len(t, tweaks, 1)
	tweak := tweaks[0]
	name, err := tweak.GetString(storage.TweakName)
	assert.NoError(t, err)
	assert.Equal(t, tweakName, name)
	_, ok := storage.Uploads.Get(uploadId)
	assert.False(t, ok, "tus upload should be consumed")

	// download it back
	resp, err := http.Get(config.Current.ServerUrl + "/tweaks/" + tweak.GetId() + "/download")
	assert.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.NoError(t, err)
	assert.EqualValues(t, tweakData, b)

	// rename it; the original extension is kept when dropped or changed
	resp, err = http.Get(config.Current.ServerUrl + "/tweaks/" + tweak.GetId() + "/rename")
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	postForm(t, "/tweaks/"+tweak.GetId()+"/rename", url.Values{"name": {"../renamed"}})
	name, err = tweak.GetString(storage.TweakName)
	assert.NoError(t, err)
	assert.Equal(t, "renamed.dylib", name)
	postForm(t, "/tweaks/"+tweak.GetId()+"/rename", url.Values{"name": {"renamed.deb"}})
	name, err = tweak.GetString(storage.TweakName)
	assert.NoError(t, err)
	assert.Equal(t, "renamed.deb.dylib", name)
	postForm(t, "/tweaks/"+tweak.GetId()+"/rename", url.Values{"name": {"renamed.dylib"}})
	tweakName = "renamed.dylib"

	// sign an app with the saved tweak
	appsBefore, err := storage.Apps.GetAll()
	assert.NoError(t, err)
	fileId := tusUpload(t, []byte(unsignedData))
	postForm(t, "/apps", url.Values{
		formNames.FormFileId:        {fileId},
		formNames.FormProfileId:     {profileId},
		formNames.FormBuilderId:     {"selfhosted"},
		formNames.FormSavedTweakIds: {tweak.GetId()},
	})
	appsAfter, err := storage.Apps.GetAll()
	assert.NoError(t, err)
	assert.Len(t, appsAfter, len(appsBefore)+1)
	app := appsAfter[0] // newest first
	defer func() {
		if job, ok := storage.Jobs.GetByAppId(app.GetId()); ok {
			storage.Jobs.DeleteById(job.Id)
		}
		assert.NoError(t, storage.Apps.Delete(app.GetId()))
	}()
	file, err := app.GetFile(storage.FSName(path.Join(string(storage.TweaksDir), tweakName)))
	assert.NoError(t, err)
	b, err = io.ReadAll(file)
	file.Close()
	assert.NoError(t, err)
	assert.EqualValues(t, tweakData, b)

	// deleting the library tweak keeps the app's copy
	resp, err = http.Get(config.Current.ServerUrl + "/tweaks/" + tweak.GetId() + "/delete")
	assert.NoError(t, err)
	resp.Body.Close()
	_, ok = storage.Tweaks.Get(tweak.GetId())
	assert.False(t, ok)
	_, err = app.Stat(storage.FSName(path.Join(string(storage.TweaksDir), tweakName)))
	assert.NoError(t, err)
}

func postForm(t *testing.T, endpoint string, form url.Values) {
	resp, err := http.PostForm(config.Current.ServerUrl+endpoint, form)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.NoError(t, util.Check2xxCode(resp.StatusCode))
}

func tusUploadNamed(t *testing.T, data []byte, name string) string {
	tusClient, err := tus.NewClient(config.Current.ServerUrl+"/tus/", nil)
	assert.NoError(t, err)
	upload := tus.NewUploadFromBytes(data)
	if name != "" {
		upload.Metadata["filename"] = name
	}
	tusUpload, err := tusClient.CreateUpload(upload)
	assert.NoError(t, err)
	assert.NoError(t, tusUpload.Upload())
	// completion is delivered asynchronously via tusd's CompleteUploads channel
	id := path.Base(tusUpload.Url())
	for i := 0; i < 100; i++ {
		if _, ok := storage.Uploads.Get(id); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return id
}

func validateManifest(t *testing.T) {
	apps, err := storage.Apps.GetAll()
	assert.NoError(t, err)
	assert.Len(t, apps, 1)
	manifestBytes, err := makeManifest(serveAddress, apps[0])
	assert.NoError(t, err)
	assert.NoError(t, validateXML(string(manifestBytes)))
}

func validateFile(t *testing.T, actualData string, f func(storage.App) (storage.ReadonlyFile, error)) {
	apps, err := storage.Apps.GetAll()
	assert.NoError(t, err)
	assert.Len(t, apps, 1)
	file, err := f(apps[0])
	assert.NoError(t, err)
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	assert.NoError(t, err)
	assert.EqualValues(t, actualData, b)
}

func tusUpload(t *testing.T, data []byte) string {
	return tusUploadNamed(t, data, "")
}

func uploadSignedFile(t *testing.T, returnId string) {
	fileId := tusUpload(t, []byte(signedData))
	form := url.Values{
		"file_id": {fileId}}
	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/jobs/%s/signed", config.Current.ServerUrl, returnId), strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+builderKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, util.Check2xxCode(resp.StatusCode))
}

func takeJob(t *testing.T) string {
	req, err := http.NewRequest("GET", config.Current.ServerUrl+"/jobs", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+builderKey)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, util.Check2xxCode(resp.StatusCode))

	var id string
	reader := tar.NewReader(resp.Body)
	for {
		header, err := reader.Next()
		assert.NoError(t, err)
		if header.Name == "id.txt" {
			b, err := ioutil.ReadAll(reader)
			assert.NoError(t, err)
			id = string(b)
			break
		}
	}
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 404)
	return id
}

func TestAuthenticationNone(t *testing.T) {
	resp, err := http.Get(config.Current.ServerUrl + "/jobs")
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 400)
}

func TestAuthenticationWrong(t *testing.T) {
	req, err := http.NewRequest("GET", config.Current.ServerUrl+"/jobs", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer 1234")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 401)
}

func uploadUnsigned(t *testing.T) {
	fileId := tusUpload(t, []byte(unsignedData))
	form := url.Values{
		formNames.FormFileId:     {fileId},
		formNames.FormProfileId:  {profileId},
		formNames.FormAllDevices: {"true"},
		formNames.FormAppDebug:   {"true"},
		formNames.FormFileShare:  {"true"},
		formNames.FormBuilderId:  {"selfhosted"},
	}
	req, err := http.NewRequest("POST", config.Current.ServerUrl+"/apps", strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, util.Check2xxCode(resp.StatusCode))
}

func TestEscapeXML(t *testing.T) {
	escapedText, err := escapeXML("This & That")
	assert.NoError(t, err)
	assert.Equal(t, "This &amp; That", escapedText)
}

func validateXML(input string) error {
	decoder := xml.NewDecoder(strings.NewReader(input))
	for {
		err := decoder.Decode(new(any))
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

func readSSEEvent(r *bufio.Reader, eventType string) string {
	event, data := "", ""
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return ""
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case line == "":
			if event == eventType {
				return data
			}
			event, data = "", ""
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data += strings.TrimPrefix(line, "data: ")
		}
	}
}

func TestLiveEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", config.Current.ServerUrl+"/events", nil)
	assert.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// changes coalesce, so wait for the expected state rather than counting frames
	reader := bufio.NewReader(resp.Body)
	states := make(chan liveState)
	go func() {
		defer close(states)
		for {
			var s liveState
			data := readSSEEvent(reader, "state")
			if data == "" || json.Unmarshal([]byte(data), &s) != nil {
				return
			}
			select {
			case states <- s:
			case <-ctx.Done():
				return
			}
		}
	}()
	waitFor := func(desc string, pred func(liveState) bool) liveState {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case s, ok := <-states:
				if !ok {
					t.Fatalf("event stream ended while waiting for %s", desc)
				}
				if pred(s) {
					return s
				}
			case <-timeout:
				t.Fatalf("timed out waiting for %s", desc)
			}
		}
	}

	waitFor("initial state", func(liveState) bool { return true })

	tweakName := uuid.NewString() + ".dylib"
	tweak, err := storage.Tweaks.New(strings.NewReader("data"), tweakName)
	if !assert.NoError(t, err) {
		return
	}
	defer storage.Tweaks.Delete(tweak.GetId())
	state := waitFor("tweak added", func(s liveState) bool {
		return strings.Contains(s.Tweaks, tweakName) && strings.Contains(s.SavedTweaks, tweakName)
	})
	assert.Contains(t, state.Tweaks, `data-id="`+tweak.GetId()+`"`)

	assert.NoError(t, storage.Tweaks.Delete(tweak.GetId()))
	waitFor("tweak deleted", func(s liveState) bool {
		return !strings.Contains(s.Tweaks, tweakName) && !strings.Contains(s.SavedTweaks, tweakName)
	})
}
