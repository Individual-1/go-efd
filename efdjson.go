// Package efd implements helper functions for interacting with the efd search and managing the results
package efd

import (
	"encoding/json"
	"net/url"
	"time"
)

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

// ReportJson is a combined format for Transaction and SearchResult for JSON serialization
type ReportJson struct {
	FirstName     string        `json:"firstname"`
	LastName      string        `json:"lastname"`
	ReportName    string        `json:"reportname"`
	ReportURL     JSONURL       `json:"reporturl,string"`
	DateSubmitted time.Time     `json:"datesubmitted"`
	ReportFormat  ReportFormat  `json:"reportformat"`
	ReportID      string        `json:"reportid"`
	Transactions  []Transaction `json:"transactions"`
	Pages         []JSONURL     `json:"pages"`
}

// ReportToJson takes a SearchResult object and results array, then marshals it into a JSON byte array
func ReportToJson(result SearchResult, parsedReport ParsedReport) ([]byte, error) {
	var ptrj ReportJson

	ptrj.FirstName = result.FirstName
	ptrj.LastName = result.LastName
	ptrj.ReportName = result.ReportName
	ptrj.ReportFormat = result.ReportFormat
	ptrj.ReportURL.URL = result.FileURL
	ptrj.DateSubmitted = result.DateSubmitted
	ptrj.ReportID = result.ReportID

	switch ptrj.ReportFormat {
	case AnnualFormat, PTRFormat:
		ptrj.Transactions = parsedReport.Transactions
	case PaperFormat:
		ptrj.Pages = make([]JSONURL, len(parsedReport.Pages.PageURLs))
		for i, page := range parsedReport.Pages.PageURLs {
			ptrj.Pages[i].URL = page
		}
	}

	out, err := json.Marshal(ptrj)
	if err != nil {
		return nil, err
	}

	return out, nil
}
