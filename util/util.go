package util

import (
	"fmt"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"net/url"
	"path"
	"path/filepath"
)

// Casts a type into another type.
// Works like mapstructure, but more reliable.
func Restructure(source interface{}, dest interface{}) error {
	bytes, err := yaml.Marshal(source)
	if err != nil {
		return errors.WithMessage(err, "marshal")
	}
	if err := yaml.Unmarshal(bytes, dest); err != nil {
		return errors.WithMessage(err, "unmarshal")
	}
	return nil
}

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