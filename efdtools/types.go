package efdtools

import (
	"encoding/json"
	"net/url"
	"time"
)

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
	DateSubmitted time.Time
	Valid         bool
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
	Owner     string    `json:"owner"`
	Ticker    string    `json:"ticker"`
	AssetName string    `json:"assetname"`
	AssetType string    `json:"assettype"`
	Type      string    `json:"type"`
	Amount    string    `json:"amount"`
	Comment   string    `json:"comment"`
	Valid     bool      `json:"-"`
}

// PTRJson is a combined format for PTRTransaction and SearchResult for JSON serialization
type PTRJson struct {
	FirstName     string           `json:"firstname"`
	LastName      string           `json:"lastname"`
	PTRURL        JSONURL          `json:"ptrurl,string"`
	DateSubmitted time.Time        `json:"datesubmitted"`
	ReportID      string           `json:"reportid"`
	Transactions  []PTRTransaction `json:"transactions"`
}

// JSONURL is a wrapper for URL type with additional unmarshalling
type JSONURL struct {
	URL *url.URL
}

// MarshalJSON implement json marshalling for the URL type
func (j JSONURL) MarshalJSON() ([]byte, error) {
	s := j.URL.String()

	return json.Marshal(s)
}

// UnmarshalJSON implements json unmarshalling for the URL type
func (j JSONURL) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	url, err := url.Parse(s)
	if err != nil {
		return err
	}

	j.URL = url
	return nil
}
