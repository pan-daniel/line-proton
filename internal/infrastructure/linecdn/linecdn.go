package linecdn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

const InstallerURL = "https://desktop.line-scdn.net/win/new/LineInst.exe"

const redirectLimit = 10

var (
	ErrInsecureURL      = errors.New("refusing to download the LINE installer over plain HTTP")
	ErrTooManyRedirects = errors.New("too many redirects while downloading the LINE installer")
)

type Downloader struct {
	client    *http.Client
	url       string
	directory string
}

func NewDownloader(client *http.Client, url, directory string) Downloader {
	return Downloader{client: httpsOnly(client), url: url, directory: directory}
}

func httpsOnly(client *http.Client) *http.Client {
	secured := *client
	secured.CheckRedirect = refuseInsecureRedirects
	return &secured
}

func refuseInsecureRedirects(request *http.Request, previous []*http.Request) error {
	if request.URL.Scheme != "https" {
		return fmt.Errorf("%w: redirected to %s", ErrInsecureURL, request.URL)
	}
	if len(previous) >= redirectLimit {
		return ErrTooManyRedirects
	}
	return nil
}

func (downloader Downloader) Fetch(ctx context.Context) (string, error) {
	if !strings.HasPrefix(downloader.url, "https://") {
		return "", fmt.Errorf("%w: %s", ErrInsecureURL, downloader.url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloader.url, nil)
	if err != nil {
		return "", err
	}
	response, err := downloader.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", downloader.url, response.Status)
	}
	return downloader.save(response.Body)
}

func (downloader Downloader) Discard(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (downloader Downloader) save(body io.Reader) (string, error) {
	if err := os.MkdirAll(downloader.directory, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(downloader.directory, "LineInst-*.exe")
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(file, body)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return "", errors.Join(err, os.Remove(file.Name()))
	}
	return file.Name(), nil
}
