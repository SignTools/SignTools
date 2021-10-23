package tunnel

import (
	"SignTools/src/util"
	"fmt"
	"github.com/ViRb3/sling/v2"
	"github.com/pkg/errors"
	"io"
	"regexp"
	"time"
)

type Cloudflare struct {
	Port uint64
}

var publicUrlRegex = regexp.MustCompile(`cloudflared_tunnel_user_hostnames_counts{userHostname="(.+)"}`)

func (c *Cloudflare) getPublicUrl(timeout time.Duration) (string, error) {
	url := fmt.Sprintf("http://localhost:%d/metrics", c.Port)
	if err := util.WaitForServer(url, timeout); err != nil {
		return "", errors.WithMessage(err, "connect to cloudflared")
	}
	response, err := sling.New().Get(url).ReceiveBody()
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if err := util.Check2xxCode(response.StatusCode); err != nil {
		return "", err
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if matches := publicUrlRegex.FindStringSubmatch(string(data)); len(matches) > 0 {
		return matches[1], nil
	}
	return "", ErrTunnelNotFound
}
