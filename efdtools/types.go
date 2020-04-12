package efdtools

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"
)

// LayoutUS is the date format MM/DD/YYY used within efdtools
const LayoutUS = "01/02/2006"

// FilerType enumerates the types of targets that can be filtered for in efd search
type FilerType = int

// Enumeration of different users documents are available for in efd search
const (
	Senator       FilerType = 1
	Candidate     FilerType = 4
	FormerSenator FilerType = 5
)

// ReportType enumerates the types of reports available in efd search
type ReportType = int

// Enumeration of types of reports and documents available in efd search
const (
	Annual                    ReportType = 7
	DueDateExtension          ReportType = 10
	PeriodicTransactionReport ReportType = 11
	BlindTrust                ReportType = 14
	OtherDocuments            ReportType = 15
)

// SearchResult is a struct matching an individual result array from efdsearch
// Each result is an array of 5 strings containing
// First Name, Last Name, Full Name, File Link, and Date Submitted
type SearchResult struct {
	FirstName     string
	LastName      string
	FullName      string
	FileURL       *url.URL
	ReportType    string
	ReportID      string
	DateSubmitted time.Time
	Valid         bool
}

// GenPTRSearchResultPath generates a filepath for a given PTR SearchResult
// Format is /%YYYY/%MM/%DD/ReportID
func (s SearchResult) GenPTRSearchResultPath() string {
	return filepath.Join(fmt.Sprintf("%04d", s.DateSubmitted.Year()), fmt.Sprintf("%02d", s.DateSubmitted.Month()),
		fmt.Sprintf("%02d", s.DateSubmitted.Day()), s.ReportID)
}

// GenAnnualSearchResultPath generates a filepath for a given Annual SearchResult
// Format is /%YYYY/ReportID
func (s SearchResult) GenAnnualSearchResultPath() string {
	return filepath.Join(fmt.Sprintf("%04d", s.DateSubmitted.Year()), s.ReportID)
}

// SearchResults is a struct matching the results json from efdsearch
// Data is an array of SearchResult-type objects
type SearchResults struct {
	Draw            int        `json:"draw"`
	RecordsTotal    int        `json:"recordsTotal"`
	RecordsFiltered int        `json:"recordsFiltered"`
	Data            [][]string `json:"data"`
	Result          string     `json:"result"`
}

// PTRTransaction is a struct matching the output of a digital PTR report
type PTRTransaction struct {
	Date      time.Time `json:"date"`
	Owner     string    `json:"owner,omitempty"`
	Ticker    string    `json:"ticker,omitempty"`
	AssetName string    `json:"assetname,omitempty"`
	AssetType string    `json:"assettype,omitempty"`
	Type      string    `json:"type,omitempty"`
	Amount    string    `json:"amount,omitempty"`
	Comment   string    `json:"comment,omitempty"`
	Valid     bool      `json:"-"`
}

// PTRJson is a combined format for PTRTransaction and SearchResult for JSON serialization
type PTRJson struct {
	FirstName     string           `json:"firstname"`
	LastName      string           `json:"lastname"`
	ReportType    string           `json:"reporttype"`
	PTRURL        JSONURL          `json:"ptrurl,string"`
	DateSubmitted time.Time        `json:"datesubmitted"`
	ReportID      string           `json:"reportid"`
	Transactions  []PTRTransaction `json:"transactions"`
}
