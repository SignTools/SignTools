package builders

import (
	"SignTools/src/util"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ViRb3/sling/v2"
	"github.com/pkg/errors"
)

type SemaphoreData struct {
	Enable      bool   `yaml:"enable"`
	ProjectName string `yaml:"project_name"`
	OrgName     string `yaml:"org_name"`
	Token       string `yaml:"token"`
	Ref         string `yaml:"ref"`
	SecretName  string `yaml:"secret_name"`
}

func MakeSemaphore(data *SemaphoreData) *Semaphore {
	baseUrl := fmt.Sprintf("https://%s.semaphoreci.com/", data.OrgName)
	return &Semaphore{
		data:    data,
		baseUrl: baseUrl,
		client: sling.New().Client(MakeClient()).
			Base(baseUrl+"api/").
			// default Go http2 user agent is (accidentally) blocked by the API server
			Set("User-Agent", "curl/7.75.0").
			SetMany(map[string]string{
				"Authorization": "Token " + data.Token,
			}),
	}
}

type Semaphore struct {
	data    *SemaphoreData
	client  *sling.Sling
	baseUrl string
}

func (s *Semaphore) Trigger() error {
	projectId, err := s.getProjectId()
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`project_id=%s&reference=%s`, projectId, s.data.Ref)
	resp, err := s.client.New().
		Body(bytes.NewReader([]byte(body))).
		Set("Content-Type", "application/x-www-form-urlencoded").
		Post("v1alpha/plumber-workflows").
		ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return util.Check2xxCode(resp.StatusCode)
}

func (s *Semaphore) GetStatusUrl() (string, error) {
	return util.JoinUrls(s.baseUrl, "projects/"+s.data.ProjectName)
}

func (s *Semaphore) getProjectId() (string, error) {
	data := semaphoreProject{}
	resp, err := s.client.New().
		Get("v1alpha/projects/" + s.data.ProjectName).
		ReceiveSuccess(&data)
	if err != nil {
		return "", err
	}
	if err := util.Check2xxCode(resp.StatusCode); err != nil {
		return "", err
	}
	return data.Metadata.ID, nil
}

type semaphoreProject struct {
	Spec struct {
		Visibility string        `json:"visibility"`
		Schedulers []interface{} `json:"schedulers"`
		Repository struct {
			Whitelist struct {
				Tags     []interface{} `json:"tags"`
				Branches []interface{} `json:"branches"`
			} `json:"whitelist"`
			URL    string `json:"url"`
			Status struct {
				PipelineFiles []struct {
					Path  string `json:"path"`
					Level string `json:"level"`
				} `json:"pipeline_files"`
			} `json:"status"`
			RunOn              []interface{} `json:"run_on"`
			PipelineFile       string        `json:"pipeline_file"`
			Owner              string        `json:"owner"`
			Name               string        `json:"name"`
			ForkedPullRequests struct {
				AllowedSecrets []interface{} `json:"allowed_secrets"`
			} `json:"forked_pull_requests"`
		} `json:"repository"`
	} `json:"spec"`
	Metadata struct {
		OwnerID     string `json:"owner_id"`
		OrgID       string `json:"org_id"`
		Name        string `json:"name"`
		ID          string `json:"id"`
		Description string `json:"description"`
	} `json:"metadata"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

type semaphoreSecret struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Metadata   semaphoreSecretMetadata `json:"metadata"`
	Data       semaphoreSecretData     `json:"data"`
}
type semaphoreSecretMetadata struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
type semaphoreSecretEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type semaphoreSecretData struct {
	EnvVars []semaphoreSecretEnvVar `json:"env_vars"`
}

func (s *Semaphore) SetSecrets(secrets map[string]string) error {
	body := semaphoreSecret{
		APIVersion: "v1beta",
		Kind:       "Secret",
		Metadata: semaphoreSecretMetadata{
			Name: s.data.SecretName,
		},
		Data: semaphoreSecretData{[]semaphoreSecretEnvVar{}},
	}
	for key, val := range secrets {
		body.Data.EnvVars = append(body.Data.EnvVars, semaphoreSecretEnvVar{key, val})
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return errors.WithMessage(err, "json marshal secret")
	}
	response, err := s.client.New().
		Body(bytes.NewReader(bodyBytes)).
		Set("Content-Type", "application/json").
		Patch("v1beta/secrets/" + s.data.SecretName).
		ReceiveSuccess(nil)
	if err != nil {
		return errors.WithMessage(err, "update existing secret")
	}
	// secret doesn't exist, create it
	if response.StatusCode == 404 {
		response, err = s.client.New().
			Body(bytes.NewReader(bodyBytes)).
			Set("Content-Type", "application/json").
			Post("v1beta/secrets").
			ReceiveSuccess(nil)
		if err != nil {
			return errors.WithMessage(err, "create new secret")
		}
	}
	return util.Check2xxCode(response.StatusCode)
}
