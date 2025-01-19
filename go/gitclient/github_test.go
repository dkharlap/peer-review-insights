package gitclient

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/stretchr/testify/assert"
)

func TestNewGitHubClient_Failure(t *testing.T) {
	token := "invalid-token"
	_, err := NewGitHubClient(token)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create github client")
}

func TestGetApiRateUsed(t *testing.T) {
	client := &GitHubClient{apiRateUsed: 5}
	assert.Equal(t, 5, client.GetApiRateUsed())
}

func TestGetApiRateRemaining(t *testing.T) {
	client := &GitHubClient{apiRateRemaining: 10}
	assert.Equal(t, 10, client.GetApiRateRemaining())
}

func TestVerifyRateLimit(t *testing.T) {
	client := &GitHubClient{}
	resp := &github.Response{
		Rate: github.Rate{
			Remaining: 0,
			Reset:     github.Timestamp{Time: time.Now().Add(1 * time.Minute)},
		},
	}

	err := client.verifyRateLimit(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rate limit reached")

	resp.Rate.Remaining = 5
	err = client.verifyRateLimit(resp)
	assert.NoError(t, err)
	assert.Equal(t, 5, client.GetApiRateRemaining())
}

func TestFilterPullRequests(t *testing.T) {
	pr1 := &github.PullRequest{CreatedAt: &github.Timestamp{Time: time.Now().Add(-2 * time.Hour)}}
	pr2 := &github.PullRequest{CreatedAt: &github.Timestamp{Time: time.Now().Add(-1 * time.Hour)}}
	prs := []*github.PullRequest{pr1, pr2}

	dateFrom := time.Now().Add(-3 * time.Hour)
	dateTo := time.Now().Add(-30 * time.Minute)

	result, found, foundBeforeDateFrom := filterPullRequests(prs, dateFrom, dateTo)

	assert.True(t, found)
	assert.False(t, foundBeforeDateFrom)
	assert.Len(t, result, 2)
}

func TestProcessError(t *testing.T) {
	errs := []error{}
	err := errors.New("test error")
	processError(&err, &errs)

	assert.Len(t, errs, 1)
	assert.Equal(t, "test error", errs[0].Error())

	noErr := error(nil)
	processError(&noErr, &errs)
	assert.Len(t, errs, 1) // No new error added
}

func TestNewPullRequest(t *testing.T) {
	now := time.Now()
	pr := &github.PullRequest{
		Number: github.Int(1),
		Title:  github.String("Test PR"),
		User:   &github.User{Login: github.String("test-user")},
		CreatedAt: &github.Timestamp{
			Time: now,
		},
	}

	result := newPullRequest(pr)
	assert.Equal(t, 1, result.Number)
	assert.Equal(t, "Test PR", *result.Title)
	assert.Equal(t, "test-user", *result.UserLogin)
	assert.Equal(t, now, *result.CreatedAt)
}

func TestNewPullRequestSlice(t *testing.T) {
	createAt1 := time.Now().Add(-3 * time.Hour)
	createAt2 := time.Now().Add(-30 * time.Minute)

	prs := []*github.PullRequest{
		{Number: github.Int(1), Title: github.String("1"), User: &github.User{Login: github.String("1")}, CreatedAt: &github.Timestamp{Time: createAt1}},
		{Number: github.Int(2), Title: github.String("2"), User: &github.User{Login: github.String("2")}, CreatedAt: &github.Timestamp{Time: createAt2}},
	}

	result := newPullRequestSlice(prs)
	assert.Len(t, result, 2)

	assert.Equal(t, 1, result[0].Number)
	assert.Equal(t, "1", *result[0].Title)
	assert.Equal(t, "1", *result[0].UserLogin)
	assert.Equal(t, createAt1, *result[0].CreatedAt)

	assert.Equal(t, 2, result[1].Number)
	assert.Equal(t, "2", *result[1].Title)
	assert.Equal(t, "2", *result[1].UserLogin)
	assert.Equal(t, createAt2, *result[1].CreatedAt)

}

func TestNewPullRequestCommentSlice(t *testing.T) {
	comment := &github.PullRequestComment{
		PullRequestReviewID: github.Int64(1),
		User: &github.User{
			ID: github.Int64(123),
		},
		Path:             github.String("//test-path"),
		OriginalPosition: github.Int(45),
		CreatedAt:        &github.Timestamp{Time: time.Now()},
	}
	result := newPullRequestCommentSlice([]*github.PullRequestComment{comment})

	assert.Len(t, result, 1)
	assert.Equal(t, *comment.PullRequestReviewID, result[0].PullRequestReviewID)
	assert.Equal(t, *comment.User.ID, result[0].UserID)
	assert.Equal(t, *comment.Path, *result[0].Path)
	assert.Equal(t, *comment.OriginalPosition, result[0].OriginalPosition)
	assert.Equal(t, comment.CreatedAt.Time, *result[0].CreatedAt)
}

func TestNewPullRequestReviewSlice(t *testing.T) {
	review := &github.PullRequestReview{
		ID: github.Int64(1),
		User: &github.User{
			ID:    github.Int64(123),
			Login: github.String("login1"),
		},
		SubmittedAt: &github.Timestamp{Time: time.Now()},
	}
	result := newPullRequestReviewSlice([]*github.PullRequestReview{review})

	assert.Len(t, result, 1)
	assert.Equal(t, *review.ID, result[0].ID)
	assert.Equal(t, *review.User.ID, result[0].UserID)
	assert.Equal(t, *review.User.Login, *result[0].UserLogin)
	assert.Equal(t, review.SubmittedAt.Time, *result[0].SubmittedAt)
}

func TestMapSlice(t *testing.T) {
	input := []int{1, 2, 3}
	output := mapSlice(input, func(i int) string {
		return fmt.Sprintf("Number: %d", i)
	})

	assert.Equal(t, []string{"Number: 1", "Number: 2", "Number: 3"}, output)
}
