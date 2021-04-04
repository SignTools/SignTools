package tunnel

import (
	"github.com/pkg/errors"
	"time"
)

type Provider interface {
	getPublicUrl(timeout time.Duration) (string, error)
}

func GetPublicUrl(provider Provider, timeout time.Duration) (string, error) {
	timer := time.After(timeout)
	var url string
	var err error
	for len(timer) < 1 {
		url, err = provider.getPublicUrl(timeout)
		if err == nil {
			return url, nil
		} else if !errors.Is(err, ErrTunnelNotFound) {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", err
}

var ErrTunnelNotFound = errors.New("tunnel not found")
