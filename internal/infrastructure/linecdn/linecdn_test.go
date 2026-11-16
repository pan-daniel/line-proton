package linecdn_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pan-daniel/line-proton/internal/infrastructure/linecdn"
)

func TestFetchSavesTheInstaller(test *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("MZ installer"))
	}))
	defer server.Close()
	path, err := linecdn.NewDownloader(server.Client(), server.URL+"/LineInst.exe", test.TempDir()).Fetch(test.Context())
	if err != nil {
		test.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "MZ installer" {
		test.Errorf("content = %q", content)
	}
	if err := linecdn.NewDownloader(server.Client(), server.URL, test.TempDir()).Discard(path); err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		test.Error("installer was not discarded")
	}
}

func TestFetchRejectsPlainHTTPAndBadStatus(test *testing.T) {
	_, err := linecdn.NewDownloader(http.DefaultClient, "http://example.invalid/x.exe", test.TempDir()).Fetch(test.Context())
	if !errors.Is(err, linecdn.ErrInsecureURL) {
		test.Errorf("err = %v, want ErrInsecureURL", err)
	}
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := linecdn.NewDownloader(server.Client(), server.URL, test.TempDir()).Fetch(test.Context()); err == nil {
		test.Error("404 was accepted")
	}
}

func TestFetchRefusesRedirectsToPlainHTTP(test *testing.T) {
	server := httptest.NewTLSServer(http.RedirectHandler("http://127.0.0.1:1/LineInst.exe", http.StatusFound))
	defer server.Close()
	_, err := linecdn.NewDownloader(server.Client(), server.URL+"/LineInst.exe", test.TempDir()).Fetch(test.Context())
	if !errors.Is(err, linecdn.ErrInsecureURL) {
		test.Errorf("err = %v, want ErrInsecureURL", err)
	}
}

func TestNewDownloaderLeavesTheCallersClientAlone(test *testing.T) {
	client := &http.Client{}
	linecdn.NewDownloader(client, "https://example.invalid/x.exe", test.TempDir())
	if client.CheckRedirect != nil {
		test.Error("the shared client was modified")
	}
}

func TestFetchLeavesNothingBehindWhenTheDownloadBreaksOff(test *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "100")
		_, _ = writer.Write([]byte("MZ in"))
	}))
	defer server.Close()
	downloads := test.TempDir()
	path, err := linecdn.NewDownloader(server.Client(), server.URL+"/LineInst.exe", downloads).Fetch(test.Context())
	if err == nil || path != "" {
		test.Errorf("Fetch = %q, %v; want no path and an error", path, err)
	}
	if entries, _ := os.ReadDir(downloads); len(entries) != 0 {
		test.Errorf("partial download left behind: %v", entries)
	}
}
