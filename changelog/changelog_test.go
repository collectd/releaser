package changelog

import (
	"testing"
	"time"

	"github.com/collectd/releaser/version"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/github"
)

type pr struct {
	body   string
	author string
	number int
	labels []string
}

func (pr pr) toGithub() *github.PullRequest {
	ret := github.PullRequest{
		Body: github.String(pr.body),
		User: &github.User{
			Login: github.String(pr.author),
		},
		Number: github.Int(pr.number),
	}
	for _, l := range pr.labels {
		ret.Labels = append(ret.Labels, &github.Label{
			Name: github.String(l),
		})
	}
	return &ret
}

func makePullRequests(prs []pr) []*github.PullRequest {
	var ret []*github.PullRequest
	for _, pr := range prs {
		ret = append(ret, pr.toGithub())
	}
	return ret
}

func TestDataString(t *testing.T) {
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
		name         string
		date         time.Time
		version      version.Version
		prs          []pr
		wantMarkdown string
		wantFile     string
	}{
		{
			name:    "real world example",
			date:    time.Date(2024, time.January, 26, 0, 0, 0, 0, time.UTC),
			version: makeVersion("collectd-6.0.1"),
			prs: []pr{
				{
					body: `By default, plugins that use the "compatibility mode" (i.e. they are still using 'value_list_t' / 'plugin_dispatch_values()') are disabled.

ChangeLog: Build system: the '--enable-compatibility-mode' has been added to control whether or not to build plugins using the compatibility mode. Such plugins are considered "unstable" and the metrics reported by these plugins will change in the future.
`,
					author: "octo",
					number: 4236,
					labels: []string{"core", "Feature"},
				},
			},
			wantMarkdown: `*   Build system: the '--enable-compatibility-mode' has been added to control whether or not to build plugins using the compatibility mode. Such plugins are considered "unstable" and the metrics reported by these plugins will change in the future. Thanks to @octo. #4236` + "\n",
			wantFile: "2024-01-26, Version 6.0.1\n" +
				"\t* Build system: the '--enable-compatibility-mode' has been added to\n" +
				"\t  control whether or not to build plugins using the compatibility mode.\n" +
				"\t  Such plugins are considered \"unstable\" and the metrics reported by\n" +
				"\t  these plugins will change in the future. Thanks to @octo. #4236\n",
		},
		{
			name:    "core PRs are sorted to the front",
			date:    time.Date(2024, time.January, 26, 0, 0, 0, 0, time.UTC),
			version: makeVersion("collectd-6.0.1"),
			prs: []pr{
				{
					body:   "ChangeLog: aaa: Text.",
					author: "user1",
					number: 1,
				},
				{
					body:   "ChangeLog: zzz: Text.",
					author: "user9",
					number: 9,
					labels: []string{"core"},
				},
			},
			wantMarkdown: "*   zzz: Text. Thanks to @user9. #9\n" +
				"*   aaa: Text. Thanks to @user1. #1\n",
			wantFile: "2024-01-26, Version 6.0.1\n" +
				"\t* zzz: Text. Thanks to @user9. #9\n" +
				"\t* aaa: Text. Thanks to @user1. #1\n",
		},
		{
			name:    "core PRs are sorted by number",
			date:    time.Date(2024, time.January, 26, 0, 0, 0, 0, time.UTC),
			version: makeVersion("collectd-6.0.1"),
			prs: []pr{
				{
					body:   "ChangeLog: aaa: Text.",
					author: "user9",
					number: 9,
					labels: []string{"core"},
				},
				{
					body:   "ChangeLog: zzz: Text.",
					author: "user1",
					number: 1,
					labels: []string{"core"},
				},
			},
			wantMarkdown: "*   zzz: Text. Thanks to @user1. #1\n" +
				"*   aaa: Text. Thanks to @user9. #9\n",
			wantFile: "2024-01-26, Version 6.0.1\n" +
				"\t* zzz: Text. Thanks to @user1. #1\n" +
				"\t* aaa: Text. Thanks to @user9. #9\n",
		},
		{
			name:    "non-core PRs are sorted alphabetically",
			date:    time.Date(2024, time.January, 26, 0, 0, 0, 0, time.UTC),
			version: makeVersion("collectd-6.0.1"),
			prs: []pr{
				{
					body:   "ChangeLog: zzz: Text.",
					author: "user1",
					number: 1,
				},
				{
					body:   "ChangeLog: aaa: Text.",
					author: "user9",
					number: 9,
				},
			},
			wantMarkdown: "*   aaa: Text. Thanks to @user9. #9\n" +
				"*   zzz: Text. Thanks to @user1. #1\n",
			wantFile: "2024-01-26, Version 6.0.1\n" +
				"\t* aaa: Text. Thanks to @user9. #9\n" +
				"\t* zzz: Text. Thanks to @user1. #1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := New(tc.date, tc.version, makePullRequests(tc.prs))

			gotMarkdown := data.String()
			if diff := cmp.Diff(tc.wantMarkdown, gotMarkdown); diff != "" {
				t.Errorf("Data.String() differs (-want/+got):\n%s", diff)
			}

			gotFile := data.FileFormat()
			if diff := cmp.Diff(tc.wantFile, gotFile); diff != "" {
				t.Errorf("Data.FileFormat() differs (-want/+got):\n%s", diff)
			}
		})
	}
}
