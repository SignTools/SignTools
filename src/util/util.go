package util

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"time"
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

func WaitForServer(url string, timeout time.Duration) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()
	for {
		select {
		case <-ctx.Done():
			return errors.New("reaching server timed out: " + url)
		default:
			if _, err := http.Get(url); err == nil {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
