package changelog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/collectd/releaser/version"
	"github.com/google/go-github/github"
)

type Data struct {
	date    time.Time
	version version.Version
	entries []entry
}

func New(date time.Time, version version.Version, prs []*github.PullRequest) Data {
	cl := Data{
		date:    date,
		version: version,
	}
	for _, pr := range prs {
		if e, ok := parseEntry(pr); ok {
			cl.entries = append(cl.entries, e)
		}
	}

	sort.Sort(cl)
	return cl
}

func (cl Data) Len() int {
	return len(cl.entries)
}

func (cl Data) Less(i, j int) bool {
	if cl.entries[i].isCore != cl.entries[j].isCore {
		return cl.entries[i].isCore
	}
	if cl.entries[i].isCore {
		return cl.entries[i].prID < cl.entries[j].prID
	}
	return cl.entries[i].text < cl.entries[j].text
}

func (cl Data) Swap(i, j int) {
	cl.entries[i], cl.entries[j] = cl.entries[j], cl.entries[i]
}

func (cl Data) String() string {
	return cl.Markdown()
}

func (cl Data) Markdown() string {
	var b strings.Builder
	for _, e := range cl.entries {
		fmt.Fprintln(&b, "*  ", e)
	}

	return b.String()
}

func (cl Data) FileFormat() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, Version %s\n", cl.date.Format("2006-01-02"), cl.version)
	for _, e := range cl.entries {
		fmt.Fprint(&b, e.FileFormat())
	}

	return b.String()
}

type entry struct {
	text   string
	author string
	prID   int
	isCore bool
}

var changeLogRE = regexp.MustCompile(`(?m)^ChangeLog: (.*)`)

func parseEntry(pr *github.PullRequest) (entry, bool) {
	m := changeLogRE.FindStringSubmatch(pr.GetBody())
	if len(m) < 2 {
		return entry{}, false
	}

	text := strings.TrimSpace(m[1])
	if !strings.HasSuffix(text, ".") {
		text = text + "."
	}

	return entry{
		text:   text,
		author: pr.GetUser().GetLogin(),
		prID:   pr.GetNumber(),
		isCore: hasLabel(pr, "core"),
	}, true
}

func hasLabel(pr *github.PullRequest, name string) bool {
	for _, l := range pr.Labels {
		if l.GetName() == name {
			return true
		}
	}
	return false
}

func (e entry) String() string {
	return fmt.Sprintf("%s Thanks to @%s. #%d", e.text, e.author, e.prID)
}

func (e entry) FileFormat() string {
	const textWidth = 80
	var b strings.Builder

	fmt.Fprint(&b, "\t*")
	col := 9

	words := strings.Split(e.String(), " ")
	for _, word := range words {
		if col+1+len(word) > textWidth {
			fmt.Fprint(&b, "\n\t ")
			col = 9
		}
		fmt.Fprint(&b, " ", word)
		col += 1 + len(word)
	}

	fmt.Fprint(&b, "\n")
	return b.String()
}
