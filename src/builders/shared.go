package builders

import (
	"net/http"
)

func MakeClient() *http.Client {
	return http.DefaultClient
}

type Builder interface {
	Trigger() error
	SetSecrets(map[string]string) error
	GetStatusUrl() (string, error)
}

// static check to ensure all methods are implemented
var _ = []Builder{&GitHub{}, &Semaphore{}, &SelfHosted{}}
