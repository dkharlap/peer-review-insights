package main

import (
	"flag"
	"log"
	"src/gitclient"
	"src/metrics"
	"time"
)

func main() {
	// Parse command-line parameters
	token, owner, repo, dateFrom, dateTo := ParseFlags()

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
func ParseFlags() (string, string, string, time.Time, time.Time) {
	token := flag.String("token", "", "GitHub access token")
	owner := flag.String("owner", "", "Repository owner (GitHub username or organization)")
	repo := flag.String("repo", "", "Repository name")
	dateFromFlag := flag.String("dateFrom", "", "Start date in YYYY-MM-DD format (required)")
	dateToFlag := flag.String("dateTo", "", "End date in YYYY-MM-DD format (optional, defaults to today)")

	flag.Parse()

	if *token == "" || *owner == "" || *repo == "" || *dateFromFlag == "" {
		log.Fatal("Error: All parameters (token, owner, repo, and dateFrom) are required")
	}

	// Parse dateFrom
	dateFrom, err := time.Parse("2006-01-02", *dateFromFlag)
	if err != nil {
		log.Fatalf("Error: Invalid value for 'dateFrom'. Please use YYYY-MM-DD format. %v", err)
	}

	// Parse dateTo, default to today if not specified
	var dateTo time.Time
	if *dateToFlag == "" {
		now := time.Now()
		dateTo = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), now.Location())
	} else {
		dateTo, err = time.Parse("2006-01-02", *dateToFlag)
		if err != nil {
			log.Fatalf("Error: Invalid value for 'dateTo'. Please use YYYY-MM-DD format. %v", err)
		}
		// Set to end of day for dateTo
		dateTo = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), dateTo.Location())
	}

	return *token, *owner, *repo, dateFrom, dateTo
}

// ...existing code...

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
