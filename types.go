// Package efd implements helper functions for interacting with the efd search and managing the results
package efd

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type intEnum int

// intArrayToString takes an array of intEnum and joins them into a delimited string
// This is primary used for joining ReportType and FilerType arrays to usable formats
// This is not necessarily particularly performant
func intEnumArrayToString(ints []intEnum, delim string) string {
	strs := []string{}
	for _, val := range ints {
		strs = append(strs, strconv.Itoa(int(val)))
	}

	return "[" + strings.Join(strs, delim) + "]"
}

// FilerType enumerates the types of targets that can be filtered for in efd search
type FilerType = intEnum

// Enumeration of different users documents are available for in efd search
const (
	SenatorFiler       FilerType = 1
	CandidateFiler     FilerType = 4
	FormerSenatorFiler FilerType = 5
)

// ReportType enumerates the types of reports available in efd search
type ReportType = intEnum

// Enumeration of types of reports and documents available in efd search
const (
	AnnualReport              ReportType = 7
	DueDateExtensionReport    ReportType = 10
	PeriodicTransactionReport ReportType = 11
	BlindTrustReport          ReportType = 14
	OtherDocumentsReport      ReportType = 15
)

// ReportFormat indicates the format of the report data
type ReportFormat int

// Enumeration of type of report formats
const (
	AnnualFormat ReportFormat = iota
	DueDateExtensionFormat
	PTRFormat
	BlindTrustFormat
	OtherDocumentFormat
	PaperFormat
	UnknownFormat
)

// URLToReportFormat returns a report format based on the URL
// https://efdsearch.senate.gov/search/view/<format>/<uuid>/
// Possible values for format include:
// ptr: Digital PTR, extension-notice/<>: Due date extension
// annual: Digital Annual
// No examples exist for Blind Trust or Other Documents
func URLToReportFormat(link *url.URL) ReportFormat {
	dir, file := path.Split(link.Path)

	// If we have a leading '/', then remove it and cut off the uuid
	if file == "" {
		dir = path.Dir(path.Dir(dir))
	}

	switch path.Base(dir) {
	case "ptr":
		return PTRFormat
	case "annual":
		return AnnualFormat
	case "paper":
		return PaperFormat
	case "regular":
		// Potential extension-notice path
		if path.Base(path.Dir(dir)) == "extension-notice" {
			return DueDateExtensionFormat
		}
	}

	return UnknownFormat
}

// SearchResults is a struct matching the results json from efdsearch
// Data is an array of SearchResult-type objects
type SearchResults struct {
	Draw            int
	RecordsTotal    int
	RecordsFiltered int
	Data            [][]string
	Result          string
}

// SearchResult is a struct matching an individual result array from efdsearch
// Each result is an array of 5 strings containing
// First Name, Last Name, Full Name, File Link, and Date Submitted
type SearchResult struct {
	FirstName     string
	LastName      string
	FullName      string
	FileURL       *url.URL
	ReportName    string
	ReportFormat  ReportFormat
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

// ParsedReport is a small wrapper around possible report outputs
type ParsedReport struct {
	ReportFormat ReportFormat
	Transactions []Transaction
	Pages        PaperReport
}

// Transaction is a struct matching the output of a digital PTR report
type Transaction struct {
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

// PaperReport is a struct containing data about filed paper reports
type PaperReport struct {
	PageURLs []*url.URL
}
