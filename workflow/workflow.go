package workflow

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/collectd/releaser/changelog"
	"github.com/collectd/releaser/version"
	"github.com/google/go-github/github"
	"github.com/octo/retry"
	"golang.org/x/oauth2"
)

type Releaser struct {
	owner, repo string
	branch      string
	client      *github.Client
	gitDir      string
	dryRun      bool
}

type Options struct {
	Owner       string
	Repo        string
	Branch      string
	AccessToken string
	GitDir      string
	DryRun      bool
}

func New(_ context.Context, opts Options) *Releaser {
	return &Releaser{
		owner:  opts.Owner,
		repo:   opts.Repo,
		branch: opts.Branch,
		client: newClient(opts.AccessToken),
		gitDir: opts.GitDir,
		dryRun: opts.DryRun,
	}
}

func (r Releaser) Run(ctx context.Context) error {
	// TODO: check if HEAD commit is "green"

	prevRelease, err := r.lastRelease(ctx)
	if err != nil {
		return err
	}
	log.Printf("Previous release was %q at tag %q", prevRelease.GetName(), prevRelease.GetTagName())

	prs, err := r.pullRequestsSince(ctx, prevRelease)
	if err != nil {
		return err
	}
	log.Printf("Found %d pull request(s)", len(prs))

	if len(prs) == 0 {
		return nil
	}

	prevVersion, err := version.New(prevRelease)
	if err != nil {
		return err
	}

	nextVersion, err := prevVersion.Next(prs)
	if err != nil {
		return err
	}
	log.Printf("The next version is %s", nextVersion)

	changeLog := changelog.New(time.Now(), nextVersion, prs)
	fmt.Printf("ChangeLog:\n%v", changeLog)

	if err := r.updateChangeLog(ctx, nextVersion, changeLog); err != nil {
		return err
	}

	if err := r.createGitHubRelease(ctx, nextVersion, changeLog); err != nil {
		return err
	}

	return nil
}

func (r Releaser) pullRequestsSince(ctx context.Context, prevRelease *github.RepositoryRelease) ([]*github.PullRequest, error) {
	ids, err := r.prIDsSince(ctx, prevRelease.GetTagName())
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		log.Println("No new pull requests found.")
		return nil, nil
	}

	log.Printf("Found %d pull request(s):\n", len(ids))

	var ret []*github.PullRequest
	for _, id := range ids {
		pr, _, err := r.client.PullRequests.Get(ctx, r.owner, r.repo, id)
		if err != nil {
			return nil, fmt.Errorf("PullRequests.Get(%q, %q, %d): %w", r.owner, r.repo, id, err)
		}
		ret = append(ret, pr)
		log.Printf("* #%d %q\n", pr.GetNumber(), pr.GetTitle())
	}

	return ret, nil
}

func (r Releaser) prIDsSince(ctx context.Context, ref string) ([]int, error) {
	log.Printf("git log --merges --pretty=oneline --grep='Merge pull request' %s..%s", r.branch, ref)
	cmd := exec.CommandContext(ctx, "git", "log", "--merges", "--pretty=oneline", "--grep=Merge pull request", ref+".."+r.branch)
	cmd.Env = append(os.Environ(), "GIT_DIR="+r.gitDir)

	reader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cmd.StdoutPipe(): %w", err)
	}
	errReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cmd.StderrPipe(): %w", err)
	}
	go func() {
		s := bufio.NewScanner(errReader)
		for s.Scan() {
			fmt.Printf("ERROR: git log: %s\n", s.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cmd.Start(): %w", err)
	}

	var ret []int
	s := bufio.NewScanner(reader)
	for s.Scan() {
		line := s.Text()
		fields := strings.Split(line, " ")
		if len(fields) < 5 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(fields[4], "#"))
		if err != nil || n <= 0 {
			continue
		}
		ret = append(ret, n)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("cmd.Wait(): %w", err)
	}

	return ret, nil
}

func (r Releaser) lastRelease(ctx context.Context) (*github.RepositoryRelease, error) {
	var (
		opt = github.ListOptions{
			PerPage: 100,
		}
		ret *github.RepositoryRelease
	)

	for {
		releases, resp, err := r.client.Repositories.ListReleases(ctx, r.owner, r.repo, &opt)
		if err != nil {
			return nil, fmt.Errorf("Repositories.ListReleases(%q, %q): %w", r.owner, r.repo, err)
		}

		for _, r := range releases {
			if !strings.HasPrefix(r.GetName(), "6") || r.GetDraft() {
				continue
			}

			if ret == nil || ret.GetCreatedAt().Before(r.GetCreatedAt().Time) {
				ret = r
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	if ret == nil {
		return nil, fmt.Errorf("no release found")
	}

	return ret, nil
}

func newClient(accessToken string) *github.Client {
	t := &retry.Transport{
		RoundTripper: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}),
		},
	}

	return github.NewClient(&http.Client{
		Transport: t,
	})
}

func (r Releaser) updateChangeLog(ctx context.Context, version version.Version, changes changelog.Data) error {
	b, err := r.GitCheckout(ctx, r.branch)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, changes.FileFormat())

	prevContent, err := b.CatFile(ctx, "ChangeLog")
	if err != nil {
		return err
	}

	if _, err := io.Copy(&buf, bytes.NewReader(prevContent)); err != nil {
		return err
	}

	if r.dryRun {
		log.Println("File ChangeLog:")
		log.Println(changes.FileFormat())
		return nil
	}

	b.GitAdd("ChangeLog", buf.Bytes())
	return b.GitCommit(ctx, fmt.Sprintf("Update ChangeLog for version %s.", version))
}

func (r Releaser) createGitHubRelease(ctx context.Context, version version.Version, changes changelog.Data) error {
	rel := &github.RepositoryRelease{
		TagName:         github.String(version.Tag()),
		TargetCommitish: github.String(r.branch),
		Name:            github.String(version.String()),
		Body:            github.String(changes.Markdown()),
		Prerelease:      github.Bool(true),
	}

	if r.dryRun {
		log.Println("GitHub Release:")
		log.Printf("%v\n", rel)
		return nil
	}

	rel, _, err := r.client.Repositories.CreateRelease(ctx, r.owner, r.repo, rel)
	if err != nil {
		return fmt.Errorf("Repositories.CreateRelease(%q, %q, %q): %w", r.owner, r.repo, version, err)
	}

	log.Printf("Successfully created release: %s", rel.GetHTMLURL())
	return nil
}
