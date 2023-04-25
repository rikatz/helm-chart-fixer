package main

import (
	"context"
	"flag"
	"log"
	"strings"

	github "github.com/google/go-github/v52/github"
	"golang.org/x/oauth2"
)

var (
	repository = flag.String("repo", "", "Repository that should be updated, like kubernetes/ingress-nginx")
	dryrun     = flag.Bool("dry-run", false, "Execute dry-run (don't commit actions)")
	tmpDir     = flag.String("tmp-dir", "/tmp", "Defines the main tmpdir to download and modify files")
	// ghbinary   = flag.String("gh-path", "/usr/local/bin/gh", "Path to Github client")
	tarbinary  = flag.String("tar-binary", "/usr/bin/tar", "Path to tar")
	helmbranch = flag.String("branch", "gh-pages", "Branch where helm metadata lives")
	ghtoken    = flag.String("token", "", "Github token with permission to list releases, push code, etc")
	chartName  = flag.String("chart name", "ingress-nginx", "What is the chart name")
	outputFile = flag.String("output", "index.yaml", "Where to output the modified index.yaml")
)

func main() {
	flag.Parse()
	ownerRepo := strings.Split(*repository, "/")
	if len(ownerRepo) != 2 {
		log.Fatal("repo should be in format org/repo")
	}
	if *ghtoken == "" {
		log.Fatal("github token cannot be empty")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *ghtoken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	if _, _, err := client.Repositories.Get(ctx, ownerRepo[0], ownerRepo[1]); err != nil {
		log.Fatalf("error testing github client: %s", err)
	}

	helmModifier, err := NewReleaseModifier(client, *chartName, *repository, *tmpDir, *helmbranch, *dryrun)
	if err != nil {
		log.Fatal(err)
	}

	if err := helmModifier.GetHelmIndex(); err != nil {
		log.Fatal(err)
	}

	modifiedCharts, err := helmModifier.downloadAndMutateReleases()
	if err != nil {
		log.Fatal(err)
	}

	if err := helmModifier.generateNewIndex(*outputFile); err != nil {
		log.Fatal(err)
	}

	if !*dryrun {
		log.Printf("should implement pushing releases here for modifiedCharts %d", len(modifiedCharts))
	}

	log.Println("finished processing file")
}
