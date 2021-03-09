package builders

import (
	"net"
	"net/http"
	"time"
)

func MakeClient(attemptHttp2 bool) *http.Client {
	transport := http.Transport{
		// Cloned http.DefaultTransport.
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     attemptHttp2,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: &transport}
}

type Builder interface {
	Trigger() error
	SetSecrets(map[string]string) error
	GetStatusUrl() string
}

// static check to ensure all methods are implemented
var _ = []Builder{&GitHub{}, &Semaphore{}, &Generic{}}
