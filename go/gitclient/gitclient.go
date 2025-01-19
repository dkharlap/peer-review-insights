package gitclient

import (
	"time"
)

type GitClient interface {
	GetApiRateUsed() int
	GetApiRateRemaining() int
	GetPullRequests(owner string, repo string, dateFrom, dateTo time.Time) ([]*PullRequest, error)
	GetComments(owner string, repo string, prNumber int) ([]*PullRequestComment, error)
	GetReviews(owner string, repo string, prNumber int) ([]*PullRequestReview, error)
	GetCommits(owner string, repo string, prNumber int, firstCommentTime time.Time, includeFiles bool) ([]*RepositoryCommit, []error)
}

// Types included in the interface definition.
type PullRequestComment struct {
	PullRequestReviewID int64
	UserID              int64
	Path                *string
	OriginalPosition    int
	CreatedAt           *time.Time
}

type PullRequestReview struct {
	ID          int64
	UserID      int64
	UserLogin   *string
	SubmittedAt *time.Time
}

type RepositoryCommit struct {
	CreatedAt *time.Time
	Files     []*RepositoryCommitFile
}

type RepositoryCommitFile struct {
	Filename *string
	Patch    *string
}

type PullRequest struct {
	Number    int
	Title     *string
	UserLogin *string
	CreatedAt *time.Time
}
