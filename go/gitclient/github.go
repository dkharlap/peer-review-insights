package gitclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client           *github.Client
	apiRateUsed      int
	apiRateRemaining int
}

func (g *GitHubClient) GetApiRateUsed() int {
	return g.apiRateUsed
}

func (g *GitHubClient) GetApiRateRemaining() int {
	return g.apiRateRemaining
}

type Logger interface {
	Info(msg string)
	Error(err error)
}

func NewGitHubClient(token string) (*GitHubClient, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(tc)

	// Check if authentication was successful
	_, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to create github client: %v", err)
	}

	return &GitHubClient{client: client, apiRateUsed: 1}, nil
}

func (g *GitHubClient) GetPullRequests(owner string, repo string, dateFrom, DateTo time.Time) ([]*PullRequest, error) {
	ctx := context.Background()
	allPRs := []*PullRequest{}

	opts := &github.PullRequestListOptions{
		State:       "all",                           // Fetch all pull requests (open, closed, merged)
		Sort:        "created",                       // Sort by creation date
		Direction:   "desc",                          // Descending order
		ListOptions: github.ListOptions{PerPage: 50}, // Number of pull requests per page
	}

	// Paginate through all pull requests
	for {
		prs, resp, err := g.client.PullRequests.List(ctx, owner, repo, opts)
		g.verifyRateLimit(resp)
		if err != nil {
			return nil, err
		}

		// Filter pull requests within the time range
		prsFiltered, found, foundBeforeDateFrom := filterPullRequests(prs, dateFrom, DateTo)

		if found {
			allPRs = append(allPRs, newPullRequestSlice(prsFiltered)...)
		}

		// Exit if reached the earliest record, or if there are no more pages
		if resp.NextPage == 0 || foundBeforeDateFrom {
			break
		}

		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

func filterPullRequests(prs []*github.PullRequest, dateFrom time.Time, dateTo time.Time) (result []*github.PullRequest, found bool, foundBeforeDateFrom bool) {
	if len(prs) == 0 {
		return nil, false, false
	}

	// Initialize indices
	startIndex := -1
	endIndex := -1
	foundBeforeDateFrom = false
	found = false

	for i, pr := range prs {
		createdAt := pr.GetCreatedAt()

		if createdAt.Before(dateFrom) {
			foundBeforeDateFrom = true
			break // No need to continue since the list is sorted in descending order
		} else {
			endIndex = i
		}

		if createdAt.After(dateTo) {
			continue // Skip records outside the upper boundary
		} else {
			found = true
			if startIndex == -1 {
				startIndex = i
			}
		}
	}

	// Handle case where no pull requests match the conditions
	if !found {
		return nil, false, foundBeforeDateFrom // Return an empty slice to indicate no matching PRs
	}

	// Return the slice of pull requests between startIndex and endIndex
	return prs[startIndex : endIndex+1], found, foundBeforeDateFrom
}

func (g *GitHubClient) GetComments(owner string, repo string, prNumber int) ([]*PullRequestComment, error) {
	ctx := context.Background()

	comments, resp, err := g.client.PullRequests.ListComments(ctx, owner, repo, prNumber, nil)
	g.verifyRateLimit(resp)

	return newPullRequestCommentSlice(comments), err
}

func (g *GitHubClient) GetReviews(owner string, repo string, prNumber int) ([]*PullRequestReview, error) {
	ctx := context.Background()

	reviews, resp, err := g.client.PullRequests.ListReviews(ctx, owner, repo, prNumber, nil)
	g.verifyRateLimit(resp)

	return newPullRequestReviewSlice(reviews), err
}

func (g *GitHubClient) GetCommits(owner string, repo string, prNumber int, firstCommentTime time.Time, includeFiles bool) ([]*RepositoryCommit, []error) {
	ctx := context.Background()
	errs := make([]error, 0)

	commits, resp, err := g.client.PullRequests.ListCommits(ctx, owner, repo, prNumber, nil)
	g.verifyRateLimit(resp)
	processError(&err, &errs)

	for _, commit := range commits {
		if commit.Commit.Committer.Date.After(firstCommentTime) {
			if includeFiles {
				// Fetch the files changed in this commit
				detailedCommit, resp, err := g.client.Repositories.GetCommit(ctx, owner, repo, commit.GetSHA(), nil)
				g.verifyRateLimit(resp)
				processError(&err, &errs)

				commit.Files = detailedCommit.Files
			}
		}
	}

	return newRepositoryCommitSlice(commits), errs
}

// Generic function to transform array of one type to another
func mapSlice[T any, U any](input []T, transform func(T) U) []U {
	result := make([]U, len(input))
	for i, item := range input {
		result[i] = transform(item)
	}
	return result
}

// Creates PullRequest from github.PullRequest
func newPullRequest(pr *github.PullRequest) *PullRequest {
	return &PullRequest{Number: *pr.Number, Title: pr.Title, UserLogin: pr.User.Login, CreatedAt: &pr.CreatedAt.Time}
}

// Creates PullRequest slice from github.PullRequest slice
func newPullRequestSlice(rcs []*github.PullRequest) []*PullRequest {
	return mapSlice(rcs, newPullRequest)
}

// Creates RepositoryCommit slice from github.RepositoryCommit slice
func newRepositoryCommitSlice(rcs []*github.RepositoryCommit) []*RepositoryCommit {
	return mapSlice(rcs, newRepositoryCommit)
}

// Creates RepositoryCommit from github.RepositoryCommit
func newRepositoryCommit(rc *github.RepositoryCommit) *RepositoryCommit {
	result := RepositoryCommit{CreatedAt: &rc.Commit.Committer.Date.Time, Files: make([]*RepositoryCommitFile, len(rc.Files))}

	for i, file := range rc.Files {
		result.Files[i] = &RepositoryCommitFile{Filename: file.Filename, Patch: file.Patch}
	}

	return &result
}

// Creates PullRequestComment from github.PullRequestComment
func newPullRequestComment(prc *github.PullRequestComment) *PullRequestComment {
	return &PullRequestComment{PullRequestReviewID: *prc.PullRequestReviewID, UserID: *prc.User.ID, Path: prc.Path, OriginalPosition: *prc.OriginalPosition, CreatedAt: &prc.CreatedAt.Time}
}

// Creates RepositoryCommit slice from github.RepositoryCommit slice
func newPullRequestCommentSlice(rcs []*github.PullRequestComment) []*PullRequestComment {
	return mapSlice(rcs, newPullRequestComment)
}

// Creates PullRequestReview from github.PullRequestReview
func newPullRequestReview(prr *github.PullRequestReview) *PullRequestReview {

	if prr.SubmittedAt == nil {
		return nil
	}

	return &PullRequestReview{ID: *prr.ID, UserID: *prr.User.ID, UserLogin: prr.User.Login, SubmittedAt: &prr.SubmittedAt.Time}
}

// Creates RepositoryReview slice from github.RepositoryReview slice
func newPullRequestReviewSlice(prr []*github.PullRequestReview) []*PullRequestReview {
	return mapSlice(prr, newPullRequestReview)
}

// Appends the error to the slice if it's not nil.
func processError(err *error, errs *[]error) {
	if *err != nil {
		if *errs != nil {
			*errs = append(*errs, *err)
		}
	}
}

// Updates API rate usage and checks if the rate limit is exceeded. Returns an error with the reset duration if the limit is reached.
func (g *GitHubClient) verifyRateLimit(resp *github.Response) error {
	g.apiRateUsed++
	g.apiRateRemaining = resp.Rate.Remaining

	if resp.Rate.Remaining == 0 {
		duration := time.Until(resp.Rate.Reset.Time)
		return errors.New("Rate limit reached and will be reset in " + duration.String())
	}

	return nil
}
