package utils

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Downloader interface {
	Download(url string) ([]byte, error)
	DownloadAndUncompress(url string, dir string, subdir string) (string, []byte, error)
}

type SimpleDownloader struct {
	TarBinary string
}

func (s *SimpleDownloader) Download(url string) ([]byte, error) {
	log.Printf("Downloading file from %s", url)
	resp, err := http.Get(url) //nolint: gosec
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 305 {
		return nil, fmt.Errorf("could not download the index file file %s: %s", url, resp.Status)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %s", url, err)
	}

	return content, err
}

// downloadAndUncompress download a chart, put it on a subdir (so there's no conflict)
// and return the full path of uncompressed files and the values.yaml content to be replaced
// (yeah yeah, I'm lazy...)
func (s *SimpleDownloader) DownloadAndUncompress(url string, dir string, subdir string) (string, []byte, error) {
	content, err := s.Download(url)
	if err != nil {
		return "", nil, err
	}

	urlFullName := strings.Split(url, "/")
	fileName := urlFullName[len(urlFullName)-1]
	fullPath := dir + "/" + fileName
	log.Printf("writing file %s", fullPath)
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return "", nil, err
	}

	fullDir := fmt.Sprintf("%s/%s", dir, subdir)
	if err := os.Mkdir(fullDir, 0755); err != nil && !os.IsExist(err) {
		return "", nil, err
	}

	log.Printf("uncompressing %s to %s", fullPath, fullDir)
	uncompressCmd := exec.Command(s.TarBinary, "xvzf", fullPath, "-C", fullDir)
	if err := uncompressCmd.Run(); err != nil {
		return "", nil, err
	}

	return fullDir, content, nil

}
