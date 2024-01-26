package version

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/google/go-github/github"
)

type Version struct {
	major, minor, patch int
	suffix              string
}

var tagRE = regexp.MustCompile(`^collectd-(6)\.([0-9]+)\.([0-9]+)(.*)$`)

func New(rel *github.RepositoryRelease) (Version, error) {
	m := tagRE.FindStringSubmatch(rel.GetTagName())
	if len(m) != 4 && len(m) != 5 {
		return Version{}, fmt.Errorf("unable to parse tag %q", rel.GetTagName())
	}

	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	var suffix string
	if len(m) > 4 {
		suffix = m[4]
	}

	return Version{
		major:  major,
		minor:  minor,
		patch:  patch,
		suffix: suffix,
	}, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d%s", v.major, v.minor, v.patch, v.suffix)
}

func (v Version) Tag() string {
	return "collectd-" + v.String()
}

func (v Version) Next(prs []*github.PullRequest) (Version, error) {
	var maxPRType prType
	for _, pr := range prs {
		if t := classifyPR(pr); maxPRType < t {
			maxPRType = t
		}
	}

	ret := v
	switch maxPRType {
	case typeFeature:
		ret.minor++
	case typeFix:
		ret.patch++
	default:
		return Version{}, errors.New("no features or fixes in list of PRs")
	}

	if v.suffix != "" {
		return v.nextSuffix()
	}

	return ret, nil
}

var suffixRE = regexp.MustCompile(`^([^0-9]*)([0-9]+)(.*)$`)

func (v Version) nextSuffix() (Version, error) {
	ret := v

	m := suffixRE.FindStringSubmatch(v.suffix)
	if len(m) == 4 {
		n, _ := strconv.Atoi(m[2])
		ret.suffix = fmt.Sprintf("%s%d%s", m[1], n+1, m[3])
	} else {
		ret.suffix += "0"
	}

	return ret, nil
}

type prType int

const (
	typeMaintenance prType = iota
	typeFix
	typeFeature
)

func classifyPR(pr *github.PullRequest) prType {
	var isFix bool
	for _, label := range pr.Labels {
		switch label.GetName() {
		case "Feature":
			return typeFeature
		case "Fix":
			isFix = true
		default:
			// no op
		}
	}
	if isFix {
		return typeFix
	}
	return typeMaintenance
}
