package metrics_test

import (
	"errors"
	"testing"
	"time"

	"src/gitclient"
	"src/metrics"

	"github.com/google/go-github/v50/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitClient is a mock implementation of gitclient.GitClient
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) GetPullRequests(owner, repo string, dateFrom, dateTo time.Time) ([]*gitclient.PullRequest, error) {
	args := m.Called(owner, repo, dateFrom, dateTo)
	return args.Get(0).([]*gitclient.PullRequest), args.Error(1)
}

func (m *MockGitClient) GetReviews(owner, repo string, prNumber int) ([]*gitclient.PullRequestReview, error) {
	args := m.Called(owner, repo, prNumber)
	return args.Get(0).([]*gitclient.PullRequestReview), args.Error(1)
}

func (m *MockGitClient) GetComments(owner, repo string, prNumber int) ([]*gitclient.PullRequestComment, error) {
	args := m.Called(owner, repo, prNumber)
	return args.Get(0).([]*gitclient.PullRequestComment), args.Error(1)
}

func (m *MockGitClient) GetCommits(owner, repo string, prNumber int, since time.Time, filtered bool) ([]*gitclient.RepositoryCommit, []error) {
	args := m.Called(owner, repo, prNumber, since, filtered)
	return args.Get(0).([]*gitclient.RepositoryCommit), []error{}
}

func (m *MockGitClient) GetApiRateUsed() int {
	return m.Called().Int(0)
}

func (m *MockGitClient) GetApiRateRemaining() int {
	return m.Called().Int(0)
}

func TestCalculateMetrics_Success(t *testing.T) {
	mockClient := new(MockGitClient)

	// Mock data
	dateFrom := time.Now().Add(-7 * 24 * time.Hour)
	dateTo := time.Now()

	mockPullRequests := []*gitclient.PullRequest{
		{
			Number:    1,
			Title:     github.String("Fix issue #123"),
			CreatedAt: &dateFrom,
			UserLogin: github.String("contributor1"),
		},
	}

	mockReviews := []*gitclient.PullRequestReview{
		{
			ID:          1,
			UserID:      11,
			UserLogin:   github.String("reviewer1"),
			SubmittedAt: &dateTo,
		},
	}

	mockComments := []*gitclient.PullRequestComment{
		{
			PullRequestReviewID: 1,
			UserID:              11,
			Path:                github.String("file.go"),
			CreatedAt:           &dateTo,
			OriginalPosition:    10,
		},
	}

	mockCommits := []*gitclient.RepositoryCommit{
		{
			CreatedAt: &dateTo,
			Files: []*gitclient.RepositoryCommitFile{
				{Filename: github.String("file.go"), Patch: github.String("@@ -10,7 +10,9 @@")},
			},
		},
	}

	// Set up mock expectations
	mockClient.On("GetPullRequests", "owner", "repo", dateFrom, dateTo).Return(mockPullRequests, nil)
	mockClient.On("GetReviews", "owner", "repo", 1).Return(mockReviews, nil)
	mockClient.On("GetComments", "owner", "repo", 1).Return(mockComments, nil)
	mockClient.On("GetCommits", "owner", "repo", 1, *mockComments[0].CreatedAt, true).Return(mockCommits, nil)
	mockClient.On("GetApiRateUsed").Return(10)
	mockClient.On("GetApiRateRemaining").Return(90)

	// Call the method
	metricsResult, errs := metrics.CalculateMetrics(mockClient, "owner", "repo", dateFrom, dateTo)

	// Assertions
	assert.Len(t, errs, 0)
	assert.NotNil(t, metricsResult)

	// Validate metrics for reviewer1
	reviewerMetrics := metricsResult["reviewer1"]
	assert.NotNil(t, reviewerMetrics)
	assert.Equal(t, 1, reviewerMetrics.PRsReviewed)
	assert.Equal(t, 1, reviewerMetrics.TotalComments)
	assert.Greater(t, reviewerMetrics.AverageTimeToFirstReview, time.Duration(0))
}

func TestCalculateMetrics_ErrorFetchingPullRequests(t *testing.T) {
	mockClient := new(MockGitClient)

	// Mock data
	dateFrom := time.Now().Add(-7 * 24 * time.Hour)
	dateTo := time.Now()

	// Set up mock expectations
	mockClient.On("GetPullRequests", "owner", "repo", dateFrom, dateTo).Return([]*gitclient.PullRequest{}, errors.New("failed to fetch PRs"))

	// Call the method
	metricsResult, errs := metrics.CalculateMetrics(mockClient, "owner", "repo", dateFrom, dateTo)

	// Assertions
	assert.Nil(t, metricsResult)
	assert.Len(t, errs, 1)
	assert.EqualError(t, errs[0], "failed to fetch PRs")
}

func TestCalculateMetrics_ErrorFetchingReviews(t *testing.T) {
	mockClient := new(MockGitClient)

	// Mock data
	dateFrom := time.Now().Add(-7 * 24 * time.Hour)
	dateTo := time.Now()

	mockPullRequests := []*gitclient.PullRequest{
		{
			Number:    1,
			Title:     github.String("Fix issue #123"),
			CreatedAt: &dateFrom,
			UserLogin: github.String("contributor1"),
		},
	}

	mockPullRequestReviews := []*gitclient.PullRequestReview{}

	// Set up mock expectations
	mockClient.On("GetPullRequests", "owner", "repo", dateFrom, dateTo).Return(mockPullRequests, nil)
	mockClient.On("GetReviews", "owner", "repo", 1).Return(mockPullRequestReviews, errors.New("failed to fetch reviews"))
	mockClient.On("GetApiRateUsed").Return(1)
	mockClient.On("GetApiRateRemaining").Return(4999)

	// Call the method
	metricsResult, errs := metrics.CalculateMetrics(mockClient, "owner", "repo", dateFrom, dateTo)

	// Assertions
	assert.Nil(t, metricsResult)
	assert.Len(t, errs, 1)
	assert.EqualError(t, errs[0], "failed to fetch reviews")
}
