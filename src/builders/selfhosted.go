package builders

import (
	"SignTools/src/util"
	"bytes"
	"context"
	"github.com/ViRb3/sling/v2"
	"github.com/pkg/errors"
	"io"
	"net/http"
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

// Status returns the builder's status page, which streams the last job's log while it runs.
// The page needs the builder key, so it's only shown through the web service.
func (g *SelfHosted) Status(ctx context.Context) (io.ReadCloser, error) {
	statusUrl, err := g.GetStatusUrl()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.Key)
	resp, err := MakeClient().Do(req)
	if err != nil {
		return nil, err
	}
	if err := util.Check2xxCode(resp.StatusCode); err != nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
			return nil, errors.WithMessage(err, "builder rejected the key")
		}
		return nil, err
	}
	return resp.Body, nil
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
