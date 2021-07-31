package main

import (
	"archive/tar"
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/ziflex/lecho/v2"
	"io"
	"io/ioutil"
	"ios-signer-service/src/builders"
	"ios-signer-service/src/config"
	"ios-signer-service/src/storage"
	"ios-signer-service/src/util"
	"mime/multipart"
	"net/http"
	"os"
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
		return app.GetUnsigned()
	})
	returnId := takeJob(t)
	uploadSignedFile(t, returnId)
	validateFile(t, signedData, func(app storage.App) (storage.ReadonlyFile, error) {
		return app.GetSigned()
	})
	validateManifest(t)
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

func uploadSignedFile(t *testing.T, returnId string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	field, err := w.CreateFormFile("file", "file.ipa")
	assert.NoError(t, err)
	field.Write([]byte(signedData))
	w.Close()
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/jobs/%s/signed", config.Current.ServerUrl, returnId), &b)
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
		formNames.FormFile:       {"This & That.ipa", unsignedData},
		formNames.FormProfileId:  {profileId},
		formNames.FormAllDevices: {"true"},
		formNames.FormAppDebug:   {"true"},
		formNames.FormFileShare:  {"true"},
		formNames.FormBuilderId:  {"selfhosted"},
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

func TestEscapeXML(t *testing.T) {
	escapedText, err := escapeXML("This & That")
	assert.NoError(t, err)
	assert.Equal(t, "This &amp; That", escapedText)
}

func validateXML(input string) error {
	decoder := xml.NewDecoder(strings.NewReader(input))
	for {
		err := decoder.Decode(new(interface{}))
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
	}
}
