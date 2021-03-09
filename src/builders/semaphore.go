package builders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ViRb3/sling/v2"
	"github.com/pkg/errors"
	"ios-signer-service/src/util"
)

type SemaphoreData struct {
	Enabled    bool   `yaml:"enabled"`
	ProjectId  string `yaml:"project_id"`
	OrgName    string `yaml:"org_name"`
	Token      string `yaml:"token"`
	Ref        string `yaml:"ref"`
	SecretName string `yaml:"secret_name"`
}

func MakeSemaphore(data *SemaphoreData) *Semaphore {
	return &Semaphore{
		data: data,
		client: sling.New().Client(MakeClient(false)).
			SetMany(map[string]string{
				"Authorization": "Token " + data.Token,
			}),
	}
}

type Semaphore struct {
	data   *SemaphoreData
	client *sling.Sling
}

func (s *Semaphore) Trigger() error {
	url := fmt.Sprintf("https://%s.semaphoreci.com/api/v1alpha/plumber-workflows", s.data.OrgName)
	body := fmt.Sprintf(`project_id=%s&reference=%s`, s.data.ProjectId, s.data.Ref)
	resp, err := s.client.New().
		Body(bytes.NewReader([]byte(body))).
		Set("Content-Type", "application/x-www-form-urlencoded").
		Post(url).
		ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return util.Check2xxCode(resp.StatusCode)
}

func (s *Semaphore) GetStatusUrl() string {
	return fmt.Sprintf("https://%s.semaphoreci.com/projects/%s", s.data.OrgName, s.data.ProjectId)
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
		Patch(fmt.Sprintf("https://%s.semaphoreci.com/api/v1beta/secrets/%s", s.data.OrgName, s.data.SecretName)).
		ReceiveSuccess(nil)
	if err != nil {
		return errors.WithMessage(err, "update existing secret")
	}
	// secret doesn't exist, create it
	if response.StatusCode == 404 {
		response, err = s.client.New().
			Body(bytes.NewReader(bodyBytes)).
			Set("Content-Type", "application/json").
			Post(fmt.Sprintf("https://%s.semaphoreci.com/api/v1beta/secrets", s.data.OrgName)).
			ReceiveSuccess(nil)
		if err != nil {
			return errors.WithMessage(err, "create new secret")
		}
	}
	return util.Check2xxCode(response.StatusCode)
}
