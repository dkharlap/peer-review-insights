package metrics

import (
	"fmt"
	"log"
	"sort"

	"strings"
	"time"

	"src/gitclient"
)

type ContributorMetrics struct {
	PRsReviewed                        int
	TotalComments                      int
	AverageCommentsPerReview           float64
	AverageTimeToFirstReview           time.Duration
	AverageTimeToCompleteReview        time.Duration
	TotalLinesReviewed                 int
	AverageLinesReviewed               float64
	PercentageCommentsLeadingToChanges float64
}

func CalculateMetrics(client gitclient.GitClient, owner, repo string, dateFrom time.Time, dateTo time.Time) (map[string]*ContributorMetrics, []error) {
	metrics := make(map[string]*ContributorMetrics)

	prs, err := client.GetPullRequests(owner, repo, dateFrom, dateTo)
	if err != nil {
		return nil, []error{err}
	}

	for _, pr := range prs {
		log.Printf("PR: %s (API rate used: %d, API rate remining %d)\n", *pr.Title, client.GetApiRateUsed(), client.GetApiRateRemaining())

		// Fetch reviews
		reviewsRaw, err := client.GetReviews(owner, repo, pr.Number)
		if err != nil {
			return nil, []error{err}
		}
		userReviews := getUserReviews(reviewsRaw)

		// Fetch comments
		comments, err := client.GetComments(owner, repo, pr.Number)
		if err != nil {
			return nil, []error{err}
		}
		reviewComments := getReviewComments(comments)

		// Fetch commits for the PR to track changes after comments
		var commits []*gitclient.RepositoryCommit
		var errs []error

		if len(comments) > 0 {
			commits, errs = client.GetCommits(owner, repo, pr.Number, *comments[0].CreatedAt, true)
			if len(errs) > 0 {
				return nil, errs
			}
		}

		// Iterate through the reviews to calculate metrics
		for user, reviews := range userReviews {

			if user != *pr.UserLogin {
				if _, exists := metrics[user]; !exists {
					metrics[user] = &ContributorMetrics{}
				}

				// Increase number od PRs reviewed
				userMetrics := metrics[user]
				userMetrics.PRsReviewed++

				// Lines of Code Reviewed
				// if pr.ChangedFiles != nil {
				// 	userMetrics.TotalLinesReviewed += *pr.ChangedFiles
				// }

				for _, review := range reviews {
					// Average Time to First Review
					firstReviewTime := review.SubmittedAt
					timeToFirstReview := firstReviewTime.Sub(*pr.CreatedAt)
					userMetrics.AverageTimeToFirstReview += timeToFirstReview

					// Average time for review
					userMetrics.AverageTimeToCompleteReview += CalculateTotalCommentPeriodLength(reviewComments[review.ID][review.UserID], *review.SubmittedAt)

					// Comments per Review
					userMetrics.TotalComments += len(reviewComments[review.ID][review.UserID])

					// Comments Leading to Changes
					commentsLeadingToChanges := 0

					for _, comment := range comments {
						for _, commit := range commits {
							// Only consider commits made after the comment
							if commit.CreatedAt.After(*comment.CreatedAt) {
								if isCommentAddressedByCommit(comment, commit) {
									commentsLeadingToChanges++
									break
								}
							}
						}
					}

					userMetrics.PercentageCommentsLeadingToChanges = float64(commentsLeadingToChanges)
				}
			}
		}
	}

	// Final calculations for averages
	for _, userMetrics := range metrics {
		if userMetrics.PRsReviewed > 0 {
			userMetrics.AverageCommentsPerReview = float64(userMetrics.TotalComments) / float64(userMetrics.PRsReviewed)
			userMetrics.AverageTimeToFirstReview /= time.Duration(userMetrics.PRsReviewed)
			userMetrics.AverageTimeToCompleteReview /= time.Duration(userMetrics.PRsReviewed)
			userMetrics.AverageLinesReviewed = float64(userMetrics.TotalLinesReviewed) / float64(userMetrics.PRsReviewed)
		}
		if userMetrics.TotalComments > 0 {
			userMetrics.PercentageCommentsLeadingToChanges /= (float64(userMetrics.PercentageCommentsLeadingToChanges) / float64(userMetrics.TotalComments)) * 100
		}
	}

	return metrics, nil
}

// Groups pull request comments by their associated review ID and user ID.
func getReviewComments(comments []*gitclient.PullRequestComment) map[int64](map[int64][]*gitclient.PullRequestComment) {
	// Initialize the top-level map
	result := make(map[int64](map[int64][]*gitclient.PullRequestComment))

	// Iterate through all comments provided in the input slice.
	for _, comment := range comments {
		// Extract the review ID and user ID from the comment.
		reviewID := comment.PullRequestReviewID
		userID := comment.UserID

		// Ensure the inner map for the review ID exists.
		if _, exists := result[reviewID]; !exists {
			result[reviewID] = make(map[int64][]*gitclient.PullRequestComment) // Initialize the inner map
		}

		// Check if a slice exists for the user ID within the review ID.
		if _, exists := result[reviewID][userID]; !exists {
			// Initialize a new slice with the first comment.
			result[reviewID][userID] = []*gitclient.PullRequestComment{comment}
		} else {
			// Append the comment to the existing slice.
			result[reviewID][userID] = append(result[reviewID][userID], comment)
		}
	}

	// Return the map containing comments grouped by review ID and user ID.
	return result
}

// Groups pull request reviews by their associated user ID.
func getUserReviews(reviews []*gitclient.PullRequestReview) map[string][]*gitclient.PullRequestReview {
	// Initialize the map
	result := make(map[string][]*gitclient.PullRequestReview)

	// Iterate through all comments provided in the input slice.
	for _, review := range reviews {
		if review.SubmittedAt != nil {
			userLogin := *review.UserLogin

			// Ensure the inner map for the review ID exists.
			if _, exists := result[userLogin]; !exists {
				result[userLogin] = make([]*gitclient.PullRequestReview, 0, len(reviews)) // Initialize the inner map
			}

			// Append the comment to the existing slice.
			result[userLogin] = append(result[userLogin], review)
		}
	}

	// Return the map containing comments grouped by review ID and user ID.
	return result
}

// CalculateTotalPeriodLength computes the total duration of all periods based on a 30-minute threshold.
func CalculateTotalCommentPeriodLength(reviewComments []*gitclient.PullRequestComment, reviewSubmittedAt time.Time) time.Duration {
	minDuration := 3 * time.Minute // If there are no comments, use this value. There is no easy way to identify when user started the review, so use this value if time less than minDuration.

	if len(reviewComments) == 0 {
		return minDuration
	}

	// Create a copy of the input slice to ensure the original is not modified
	dateTimes := make([]time.Time, 0, len(reviewComments)+1) // Preallocate slice with the expected size

	dateTimes = append(dateTimes, reviewSubmittedAt)

	for _, comment := range reviewComments {
		dateTimes = append(dateTimes, *comment.CreatedAt)
	}

	// Sort the input array to ensure chronological order
	sort.Slice(dateTimes, func(i, j int) bool {
		return dateTimes[i].Before(dateTimes[j])
	})

	totalDuration := time.Duration(0) // Initialize total duration
	start := dateTimes[0]             // Start time of the current period
	end := start                      // End time of the current period

	// Iterate through the datetime array
	for i := 1; i < len(dateTimes); i++ {
		// Calculate the time difference from the previous item
		diff := dateTimes[i].Sub(dateTimes[i-1])

		if diff <= 30*time.Minute {
			// Part of the same period, update the end time
			end = dateTimes[i]
		} else {
			// Time difference > 30 minutes, calculate the current period duration
			totalDuration += end.Sub(start)
			// Start a new period
			start = dateTimes[i]
			end = start
		}
	}

	// Add the last period duration
	totalDuration += end.Sub(start)

	if totalDuration <= minDuration {
		return minDuration
	} else {
		return totalDuration
	}
}

// New helper function to find if a comment is addressed by a commit
func isCommentAddressedByCommit(comment *gitclient.PullRequestComment, commit *gitclient.RepositoryCommit) bool {
	for _, file := range commit.Files {
		if file.Filename == comment.Path { // Same file as the comment
			// Check if the lines in the comment are affected in the commit
			if *file.Patch != "" && strings.Contains(*file.Patch, fmt.Sprintf("@@ -%d", comment.OriginalPosition)) {
				return true
			}
		}
	}
	return false
}
