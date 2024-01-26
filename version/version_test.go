package version_test

import (
	"testing"

	"github.com/collectd/releaser/version"
	"github.com/google/go-github/github"
)

func makePRWithLabels(labels ...string) *github.PullRequest {
	var pr github.PullRequest
	for _, name := range labels {
		pr.Labels = append(pr.Labels, &github.Label{
			Name: github.String(name),
		})
	}
	return &pr
}

func TestNew(t *testing.T) {
	cases := []struct {
		name    string
		release *github.RepositoryRelease
		want    string
		wantErr bool
	}{
		{
			name: "normal",
			release: &github.RepositoryRelease{
				TagName: github.String("collectd-6.0.0"),
			},
			want: "6.0.0",
		},
		{
			name: "with suffix",
			release: &github.RepositoryRelease{
				TagName: github.String("collectd-6.0.0.rc0"),
			},
			want: "6.0.0.rc0",
		},
		{
			name: "invalid prefix",
			release: &github.RepositoryRelease{
				TagName: github.String("foo-6.0.0"),
			},
			wantErr: true,
		},
		{
			name: "not enough components",
			release: &github.RepositoryRelease{
				TagName: github.String("collectd-6.0"),
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := version.New(tc.release)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("version.New(%v) = %v, want error %v", tc.release, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got := v.String(); got != tc.want {
				t.Errorf("version.New().String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNext(t *testing.T) {
	makeVersion := func(tag string) version.Version {
		v, err := version.New(&github.RepositoryRelease{
			TagName: github.String(tag),
		})
		if err != nil {
			panic(err)
		}
		return v
	}

	cases := []struct {
		name    string
		prev    version.Version
		prs     []*github.PullRequest
		want    string
		wantErr bool
	}{
		{
			name: "feature release",
			prev: makeVersion("collectd-6.0.0"),
			prs: []*github.PullRequest{
				makePRWithLabels("Feature"),
				makePRWithLabels("Fix"),
				makePRWithLabels("Maintenance"),
			},
			want: "6.1.0",
		},
		{
			name: "fix release",
			prev: makeVersion("collectd-6.0.0"),
			prs: []*github.PullRequest{
				makePRWithLabels("Maintenance"),
				makePRWithLabels("Fix"),
				makePRWithLabels("Maintenance"),
			},
			want: "6.0.1",
		},
		{
			name: "maintenance only",
			prev: makeVersion("collectd-6.0.0"),
			prs: []*github.PullRequest{
				makePRWithLabels("Maintenance"),
				makePRWithLabels("Maintenance"),
			},
			wantErr: true,
		},
		{
			name: "feature release with suffix",
			prev: makeVersion("collectd-6.0.0.rc0"),
			prs: []*github.PullRequest{
				makePRWithLabels("Feature"),
				makePRWithLabels("Fix"),
				makePRWithLabels("Maintenance"),
			},
			want: "6.0.0.rc1",
		},
		{
			name: "fix release with suffix",
			prev: makeVersion("collectd-6.0.0.rc0"),
			prs: []*github.PullRequest{
				makePRWithLabels("Maintenance"),
				makePRWithLabels("Fix"),
				makePRWithLabels("Maintenance"),
			},
			want: "6.0.0.rc1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, err := tc.prev.Next(tc.prs)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("Version.Next() = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got := next.String(); got != tc.want {
				t.Errorf("version.New().String() = %q, want %q", got, tc.want)
			}
		})
	}
}
