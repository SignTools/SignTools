package builders

import (
	"bytes"
	"github.com/ViRb3/sling/v2"
	"net/url"
)

type GenericData struct {
	Enabled      bool              `yaml:"enabled"`
	StatusUrl    string            `yaml:"status_url"`
	TriggerUrl   string            `yaml:"trigger_url"`
	SecretsUrl   string            `yaml:"secrets_url"`
	TriggerBody  string            `yaml:"trigger_body"`
	Headers      map[string]string `yaml:"headers"`
	AttemptHTTP2 bool              `yaml:"attempt_http2"`
}

func MakeGeneric(data *GenericData) *Generic {
	return &Generic{
		GenericData: data,
		Client:      sling.New().Client(MakeClient(data.AttemptHTTP2)).SetMany(data.Headers),
	}
}

type Generic struct {
	*GenericData
	Client *sling.Sling
}

func (g *Generic) Trigger() error {
	_, err := g.Client.New().Body(bytes.NewReader([]byte(g.TriggerBody))).Post(g.TriggerUrl).ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return nil
}

func (g *Generic) GetStatusUrl() string {
	return g.StatusUrl
}

func (g *Generic) SetSecrets(secrets map[string]string) error {
	body := url.Values{}
	for key, val := range secrets {
		body.Set(key, val)
	}
	_, err := g.Client.New().Body(bytes.NewReader([]byte(body.Encode()))).Post(g.SecretsUrl).ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return nil
}
