package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeDownloader struct {
	file string
}

func (f *fakeDownloader) Download(url string) ([]byte, error) {
	return os.ReadFile(f.file)
}

func (f *fakeDownloader) DownloadAndUncompress(url string, dir string, subdir string) (string, []byte, error) {
	return "", nil, nil
}

func TestGetHelmIndex(t *testing.T) {

	tests := []struct {
		name    string
		file    string
		entries int
		wantErr bool
	}{
		{
			name:    "should parse yaml correctly",
			file:    "testdata/index-ok.yaml",
			entries: 14,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloader := fakeDownloader{file: tt.file}
			h := &HelmModifier{
				downloader: &downloader,
				name:       "ingress-nginx",
			}
			if err := h.GetHelmIndex(); (err != nil) != tt.wantErr {
				t.Errorf("HelmModifier.GetHelmIndex() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.Len(t, h.indexfile.Entries["ingress-nginx"], tt.entries)
		})
	}
}
