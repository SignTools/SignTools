package builders

import (
	"SignTools/src/util"
	"bytes"
	"github.com/ViRb3/sling/v2"
	"net/url"
)

type SelfHostedData struct {
	Enable bool   `yaml:"enable"`
	Url    string `yaml:"url"`
	Key    string `yaml:"key"`
}

func MakeSelfHosted(data *SelfHostedData) *SelfHosted {
	return &SelfHosted{
		SelfHostedData: data,
		Client: sling.New().Client(MakeClient()).
			Base(data.Url).
			SetMany(map[string]string{
				"Authorization": "Bearer " + data.Key,
			}),
	}
}

type SelfHosted struct {
	*SelfHostedData
	Client *sling.Sling
}

func (g *SelfHosted) Trigger() error {
	resp, err := g.Client.New().Post("/trigger").ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return util.Check2xxCode(resp.StatusCode)
}

func (g *SelfHosted) GetStatusUrl() (string, error) {
	return util.JoinUrls(g.Url, "/status")
}

func (g *SelfHosted) SetSecrets(secrets map[string]string) error {
	body := url.Values{}
	for key, val := range secrets {
		body.Set(key, val)
	}
	resp, err := g.Client.New().
		Set("Content-Type", "application/x-www-form-urlencoded").
		Body(bytes.NewReader([]byte(body.Encode()))).
		Post("/secrets").
		ReceiveSuccess(nil)
	if err != nil {
		return err
	}
	return util.Check2xxCode(resp.StatusCode)
}
