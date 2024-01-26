package workflow

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-github/github"
)

type GitBranch struct {
	releaser Releaser
	stage    []github.TreeEntry
	*github.Branch
}

func (r Releaser) GitCheckout(ctx context.Context, branchName string) (*GitBranch, error) {
	branch, _, err := r.client.Repositories.GetBranch(ctx, r.owner, r.repo, branchName)
	if err != nil {
		return nil, fmt.Errorf("Repositories.GetBranch(%q, %q, %q): %w", r.owner, r.repo, branchName, err)
	}

	return &GitBranch{
		releaser: r,
		Branch:   branch,
	}, nil
}

func (b *GitBranch) CatFile(ctx context.Context, path string) ([]byte, error) {
	content, _, _, err := b.releaser.client.Repositories.GetContents(ctx, b.releaser.owner, b.releaser.repo, path, &github.RepositoryContentGetOptions{
		Ref: b.GetCommit().GetSHA(),
	})
	if err != nil {
		return nil, fmt.Errorf("Repositories.GetContents(%q, %q, %q, %q): %w", b.releaser.owner, b.releaser.repo, path, b.GetCommit().GetSHA(), err)
	}

	decodedContent, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("content.GetContent(): %w", err)
	}
	return []byte(decodedContent), nil
}

func (b *GitBranch) GitAdd(path string, content []byte) {
	b.stage = append(b.stage, github.TreeEntry{
		Path:    github.String(path),
		Mode:    github.String("100644"),
		Type:    github.String("blob"),
		Content: github.String(string(content)),
	})
}

func (b *GitBranch) GitCommit(ctx context.Context, message string) error {
	if b.stage == nil {
		return nil
	}

	parent, _, err := b.releaser.client.Git.GetCommit(ctx, b.releaser.owner, b.releaser.repo, b.GetCommit().GetSHA())
	if err != nil {
		return fmt.Errorf("Git.GetCommit(%q, %q, %q): %w", b.releaser.owner, b.releaser.repo, b.GetCommit().GetSHA(), err)
	}

	tree, _, err := b.releaser.client.Git.CreateTree(ctx, b.releaser.owner, b.releaser.repo, parent.GetTree().GetSHA(), b.stage)
	if err != nil {
		return fmt.Errorf("Git.CreateTree(%q, %q, %q): %w", b.releaser.owner, b.releaser.repo, parent.GetTree().GetSHA(), err)
	}

	log.Printf("parent.GetSHA() = %q", parent.GetSHA())
	commit, _, err := b.releaser.client.Git.CreateCommit(ctx, b.releaser.owner, b.releaser.repo, &github.Commit{
		Message: github.String(message),
		Tree:    tree,
		Parents: []github.Commit{*parent},
	})
	if err != nil {
		return fmt.Errorf("Git.CreateCommit(%q, %q): %w", b.releaser.owner, b.releaser.repo, err)
	}
	log.Printf("Successfully created new commit: %s", commit.GetHTMLURL())

	const force = false
	_, _, err = b.releaser.client.Git.UpdateRef(ctx, b.releaser.owner, b.releaser.repo, &github.Reference{
		Ref: github.String("heads/" + b.GetName()),
		Object: &github.GitObject{
			Type: github.String("commit"),
			SHA:  github.String(commit.GetSHA()),
		},
	}, force)
	if err != nil {
		return fmt.Errorf("Git.UpdateRef(%q, %q, %q): %w", b.releaser.owner, b.releaser.repo, b.GetName(), err)
	}

	b.stage = nil
	return nil
}
