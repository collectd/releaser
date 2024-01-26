package main

import (
	"context"
	"log"
	"os"

	"github.com/collectd/releaser/workflow"
)

const (
	owner = "collectd"
	repo  = "collectd"
)

const tokenEnv = "GITHUB_TOKEN"

func main() {
	ctx := context.Background()

	opts := workflow.Options{
		Owner:       owner,
		Repo:        repo,
		Branch:      "collectd-6.0",
		AccessToken: os.Getenv(tokenEnv),
		GitDir:      "/home/octo/collectd/.git",
	}

	if opts.AccessToken == "" {
		log.Fatalf("The environment variable %q is empty or unset", tokenEnv)
	}

	wf := workflow.New(ctx, opts)
	if err := wf.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
