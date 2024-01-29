package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/collectd/releaser/workflow"
)

const (
	owner = "collectd"
	repo  = "collectd"
)

var (
	dryRun = flag.Bool("dryrun", true, "controls whether changes are made upstream")
)

const tokenEnv = "GITHUB_TOKEN"

func main() {
	flag.Parse()
	ctx := context.Background()

	opts := workflow.Options{
		Owner:       owner,
		Repo:        repo,
		Branch:      "collectd-6.0",
		AccessToken: os.Getenv(tokenEnv),
		GitDir:      "/home/octo/collectd/.git",
		DryRun:      *dryRun,
	}

	if opts.AccessToken == "" {
		log.Fatalf("The environment variable %q is empty or unset", tokenEnv)
	}

	wf := workflow.New(ctx, opts)
	if err := wf.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
