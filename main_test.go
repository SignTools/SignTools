package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"ios-signer-service/builders"
	"ios-signer-service/config"
	"ios-signer-service/storage"
	"ios-signer-service/util"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

var (
	workflowData          = uuid.NewString()
	workflowAuthorization = map[string]string{
		uuid.NewString(): uuid.NewString(),
		uuid.NewString(): uuid.NewString(),
	}
	builderKey   = uuid.NewString()
	saveDir      = ""
	profileId    = uuid.NewString()
	profileCert  = uuid.NewString()
	profileName  = uuid.NewString()
	profilePass  = uuid.NewString()
	profileProv  = uuid.NewString()
	unsignedData = uuid.NewString()
	signedData   = uuid.NewString()
)

func TestMain(m *testing.M) {
	var err error
	saveDir, err = os.MkdirTemp(".", "data-test")
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := os.RemoveAll(saveDir); err != nil {
			log.Fatalln(err)
		}
	}()

	profileDir := filepath.Join(saveDir, "profiles", profileId)
	if err := os.MkdirAll(profileDir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}
	contentMap := map[string]string{
		"cert.p12":             profileCert,
		"name.txt":             profileName,
		"pass.txt":             profilePass,
		"prov.mobileprovision": profileProv,
	}
	for key, val := range contentMap {
		if err := ioutil.WriteFile(filepath.Join(profileDir, key), []byte(val), os.ModePerm); err != nil {
			log.Fatalln(err)
		}
	}

	serveHost := "localhost"
	servePort := uint64(8098)
	workflowPort := uint64(8099)

	config.Current = config.Config{
		Builder: builders.MakeGeneric(&builders.GenericData{
			StatusUrl:    "",
			TriggerUrl:   fmt.Sprintf("http://localhost:%d/trigger", workflowPort),
			SecretsUrl:   fmt.Sprintf("http://localhost:%d/secrets", workflowPort),
			TriggerBody:  workflowData,
			Headers:      workflowAuthorization,
			AttemptHTTP2: true,
		}),
		File: &config.File{
			ServerUrl:           fmt.Sprintf("http://localhost:%d", servePort),
			SaveDir:             saveDir,
			CleanupMins:         0,
			CleanupIntervalMins: 0,
		},
		BuilderKey: builderKey,
	}

	storage.Load()
	go serve(serveHost, servePort)
	go startWorkflowServer(workflowPort)

	m.Run()
}

var triggerHit = false
var secretsHit = false

func startWorkflowServer(port uint64) {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())

	e.POST("/secrets", func(c echo.Context) error {
		secretsHit = true
		params, err := c.FormParams()
		if err != nil {
			log.Fatalln(err)
		}
		for key, val := range params {
			switch key {
			case "SECRET_KEY":
				if val[0] != builderKey {
					log.Fatalln("bad key")
				}
			case "SECRET_URL":
				if val[0] != config.Current.File.ServerUrl {
					log.Fatalln("bad url")
				}
			default:
				log.Fatalln("unknown secret")
			}
		}
		return c.NoContent(200)
	})

	e.POST("/trigger", func(c echo.Context) error {
		triggerHit = true
		builder := config.Current.Builder.(*builders.Generic)
		for key, val := range builder.Headers {
			if c.Request().Header.Get(key) != val {
				log.Fatalln("bad header")
			}
		}
		if c.Request().Body == nil {
			log.Fatalln("trigger body is nil")
		}
		bodyBytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			log.Fatalln(err)
		}
		if string(bodyBytes) != builder.TriggerBody {
			log.Fatalln("mismatching data")
		}
		return c.NoContent(200)
	})

	log.Fatalln(e.Start(fmt.Sprintf("localhost:%d", port)))
}

func TestIntegration(t *testing.T) {
	uploadUnsigned(t)
	assert.True(t, triggerHit)
	assert.True(t, secretsHit)
	validateFile(t, unsignedData, func(app storage.App) (storage.ReadonlyFile, error) {
		return app.GetUnsigned()
	})
	returnId := takeJob(t)
	uploadSignedFile(t, returnId)
	validateFile(t, signedData, func(app storage.App) (storage.ReadonlyFile, error) {
		return app.GetSigned()
	})
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

func uploadSignedFile(t *testing.T, returnId string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	field, err := w.CreateFormFile("file", "file.ipa")
	assert.NoError(t, err)
	field.Write([]byte(signedData))
	w.Close()
	req, err := http.NewRequest("POST", config.Current.ServerUrl+"/jobs/"+returnId, &b)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+builderKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
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
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	formData := map[string][]string{
		"file":         {"file.ipa", unsignedData},
		"profile_name": {profileName},
		"all_devices":  {"true"},
		"app_debug":    {"true"},
		"file_share":   {"true"},
	}
	for key, val := range formData {
		var field io.Writer
		var err error
		if len(val) > 1 {
			field, err = w.CreateFormFile(key, val[0])
			val = val[1:]
		} else {
			field, err = w.CreateFormField(key)
		}
		assert.NoError(t, err)
		_, err = field.Write([]byte(val[0]))
		assert.NoError(t, err)
	}
	assert.NoError(t, w.Close())
	req, err := http.NewRequest("POST", config.Current.ServerUrl+"/apps", &b)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.NoError(t, util.Check2xxCode(resp.StatusCode))
}
