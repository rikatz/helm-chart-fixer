package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"log"

	github "github.com/google/go-github/v52/github"
	"github.com/rikatz/helm-chart-fixer/pkg/utils"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

type modifiedChart struct {
	// releaseName is the release name that needs to have the new artifact uploaded
	releaseName string
	// artifactFile is the full path of the new modified artifact
	artifactFile string
}

type HelmModifier struct {
	name       string
	indexfile  *repo.IndexFile
	downloader utils.Downloader
	tmpDir     string
	ghrepo     string
	branch     string
	dryrun     bool
	client     *github.Client
}

func NewReleaseModifier(client *github.Client, chartname string, repository string, tmpdir string, branch string, dryrun bool) (*HelmModifier, error) {
	helmTmpDir, err := os.MkdirTemp(tmpdir, "helm-fix")
	if err != nil {
		return nil, fmt.Errorf("failed creating tmpdir: %s", err)
	}

	downloader := utils.SimpleDownloader{
		TarBinary: *tarbinary,
	}

	return &HelmModifier{
		client:     client,
		name:       chartname,
		downloader: &downloader,
		tmpDir:     helmTmpDir,
		ghrepo:     repository,
		branch:     branch,
		dryrun:     dryrun,
	}, nil
}

func (h *HelmModifier) GetHelmIndex() error {
	urlIndex := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/index.yaml", h.ghrepo, h.branch)

	log.Printf("Reading chart index %s", urlIndex)

	indexContent, err := h.downloader.Download(urlIndex)
	if err != nil {
		return err
	}

	var indexFile repo.IndexFile
	if err := yaml.Unmarshal(indexContent, &indexFile); err != nil {
		return fmt.Errorf("error unmarshalling index file: %s", err)
	}

	_, ok := indexFile.Entries[h.name]
	if !ok {
		return fmt.Errorf("chart not found on index file")
	}
	h.indexfile = &indexFile
	return nil

}

func (h *HelmModifier) downloadAndMutateReleases() ([]modifiedChart, error) {
	modifiedCharts := make([]modifiedChart, 0)
	if h.indexfile == nil {
		return nil, fmt.Errorf("indexFile struct cannot be null")
	}

	entries, ok := h.indexfile.Entries[h.name]
	if !ok {
		return nil, fmt.Errorf("nil entries")
	}
	for _, release := range entries {
		version := release.Version
		if len(release.URLs) != 1 {
			log.Printf("release %s not supported, should have only 1 url, have %d", version, len(release.URLs))
			continue
		}
		urlRelease := release.URLs[0]
		// We need to extract the release name from artifact, like:
		// https://github.com/kubernetes/ingress-nginx/releases/download/release-name-VERSION/ingress-nginx-VERSION.tar.gz
		// We can first trim the beginning, then get the first item from the remaining
		prefix := fmt.Sprintf("https://github.com/%s/releases/download/", h.ghrepo)
		releaseArtifact := strings.Split(strings.TrimPrefix(urlRelease, prefix), "/")

		fullChartDir, _, err := h.downloader.DownloadAndUncompress(urlRelease, h.tmpDir, version)
		if err != nil {
			log.Printf("unable to download release %s, skipping: %s", urlRelease, err)
			continue
		}

		valuesFiles := fmt.Sprintf("%s/%s/values.yaml", fullChartDir, h.name)
		content, err := os.ReadFile(valuesFiles)
		if err != nil {
			log.Printf("cannot read values file %s, %s", valuesFiles, err)
			continue
		}
		if bytes.Contains(content, []byte("k8s.gcr.io")) {
			log.Printf("replacing content on file %s", valuesFiles)
			content = bytes.ReplaceAll(content, []byte("k8s.gcr.io"), []byte("registry.k8s.io"))
			if err := os.WriteFile(valuesFiles, content, 0644); err != nil {
				log.Printf("cannot write file %s: %s", valuesFiles, err)
				continue
			}

			newFile, digest, err := h.compressAndGetDigest(fullChartDir, releaseArtifact[1])
			if err != nil {
				log.Printf("error creating new helm file for %s/%s: %s skipping", fullChartDir, releaseArtifact[1], err)
			}

			log.Printf("setting new digest for %s as %s", newFile, digest)
			release.Digest = digest

			modified := modifiedChart{
				releaseName:  releaseArtifact[0],
				artifactFile: newFile,
			}
			modifiedCharts = append(modifiedCharts, modified)
		}
	}
	return modifiedCharts, nil

}

// compressAndGetDigest returns the path of the new artifact, digest and an error
func (h *HelmModifier) compressAndGetDigest(chartDir string, filename string) (string, string, error) {
	newDirLoc := fmt.Sprintf("%s/new", chartDir)
	if err := os.Mkdir(newDirLoc, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory for new artifact: %s", err)
	}

	fullName := fmt.Sprintf("%s/%s", newDirLoc, filename)
	cmd := exec.Command(*tarbinary, "cvzf", fullName, h.name)
	cmd.Dir = chartDir

	log.Printf("compressing back %s", fullName)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("error compressing new file %s: %s", fullName, err)
	}

	tarFile, err := os.ReadFile(fullName)
	if err != nil {
		return "", "", fmt.Errorf("error opening file %s to get digest: %s", fullName, err)
	}

	digest := sha256.Sum256(tarFile)
	return fullName, fmt.Sprintf("%x", digest), nil

}

// generateNewIndex generates the new modified values file
func (h *HelmModifier) generateNewIndex(location string) error {
	content, err := yaml.Marshal(h.indexfile)
	if err != nil {
		return fmt.Errorf("failed marshalling file %s: %s", location, err)
	}
	if err := os.WriteFile(location, content, 0644); err != nil {
		return fmt.Errorf("error writing new file: %s", err)
	}
	return nil
}
