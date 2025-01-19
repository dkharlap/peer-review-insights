package main

import (
	"flag"
	"log"
	"src/gitclient"
	"src/metrics"
	"strconv"
	"time"
)

func main() {
	// Parse command-line parameters
	token, owner, repo, days := ParseFlags()

	// Calculate dateFrom and dateTo based on 'days'
	dateFrom, dateTo := CalculateDateRange(days)

	// Get the GitHub client
	client, err := gitclient.NewGitHubClient(token)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	// Calculate the metrics based on date range
	results, errs := metrics.CalculateMetrics(client, owner, repo, dateFrom, dateTo)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Fatal(err.Error())
		}

		return
	}

	// Log the results
	logResults(results)
}

// ParseFlags handles the parsing of command-line flags
func ParseFlags() (string, string, string, int) {
	token := flag.String("token", "", "GitHub access token")
	owner := flag.String("owner", "", "Repository owner (GitHub username or organization)")
	repo := flag.String("repo", "", "Repository name")
	daysFlag := flag.String("days", "", "Number of days ago to calculate the date range (e.g., 10)")

	// Parse the flags provided by the user
	flag.Parse()

	// Ensure all required parameters are provided
	if *token == "" || *owner == "" || *repo == "" || *daysFlag == "" {
		log.Fatal("Error: All parameters (token, owner, repo, and days) are required")
	}

	// Convert 'days' from string to integer
	days, err := strconv.Atoi(*daysFlag)
	if err != nil {
		log.Fatalf("Error: Invalid value for 'days'. Please provide a valid integer. %v", err)
	}

	return *token, *owner, *repo, days
}

// CalculateDateRange calculates the date range (dateFrom and dateTo) based on 'days'
func CalculateDateRange(days int) (time.Time, time.Time) {
	// Get the current date and time
	now := time.Now()

	// Truncate the time to midnight (12:00 AM) for today
	midnightNow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Calculate the start date (dateFrom) as 'days' ago at midnight
	dateFrom := midnightNow.AddDate(0, 0, -days)

	// Set dateTo to the end of today (23:59:59.999999)
	dateTo := midnightNow.Add(24*time.Hour - time.Nanosecond)

	return dateFrom, dateTo
}

// logResults logs the calculated metrics
func logResults(results map[string]*metrics.ContributorMetrics) {
	for contributor, metrics := range results {
		log.Printf("\nContributor: %s\n", contributor)
		log.Printf("PRs Reviewed: %d\n", metrics.PRsReviewed)
		log.Printf("Average Comments per Review: %.2f\n", metrics.AverageCommentsPerReview)
		log.Printf("Average Time to Complete Review: %v\n", metrics.AverageTimeToCompleteReview)
		log.Printf("Average Time to First Review: %v\n", metrics.AverageTimeToFirstReview)
		log.Printf("Total Comments: %d\n", metrics.TotalComments)
		log.Printf("Percentage of Comments Leading to Changes: %.2f%%\n", metrics.PercentageCommentsLeadingToChanges)
	}
}
