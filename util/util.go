package util

import (
	"fmt"
	"github.com/pkg/errors"
	"net/url"
	"path"
	"path/filepath"
)

func SafeJoin(basePath string, unsafePath ...string) string {
	return filepath.Join(basePath, filepath.Clean("/"+filepath.Join(unsafePath...)))
}

func JoinUrlsPanic(fullBaseUrl string, relativeUrl ...string) string {
	result, err := JoinUrls(fullBaseUrl, relativeUrl...)
	if err != nil {
		panic(err)
	}
	return result
}

func JoinUrls(fullBaseUrl string, relativeUrl ...string) (string, error) {
	parsedBase, err := url.Parse(fullBaseUrl)
	if err != nil {
		return "", err
	}
	return JoinUrlsParsed(parsedBase, relativeUrl...)
}

func JoinUrlsParsed(parsedBase *url.URL, relativeUrl ...string) (string, error) {
	parsedRelative, err := url.Parse(path.Join(relativeUrl...))
	if err != nil {
		return "", err
	}
	return parsedBase.ResolveReference(parsedRelative).String(), nil
}

func Check2xxCode(code int) error {
	if code < 200 || code > 299 {
		return errors.New(fmt.Sprintf("non-2xx code: %d", code))
	}
	return nil
}
