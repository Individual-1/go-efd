// Package efdtools implements helper functions for interacting with the efd search
package efdtools

import (
	"encoding/json"
	"net/url"
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

// PTRToJSON takes a SearchResult object and results array, then marshals it into a JSON byte array
func PTRToJSON(result SearchResult, ptrs []PTRTransaction) ([]byte, error) {
	var ptrj PTRJson

	ptrj.Transactions = ptrs
	ptrj.FirstName = result.FirstName
	ptrj.LastName = result.LastName
	ptrj.PTRURL.URL = result.FileURL
	ptrj.DateSubmitted = result.DateSubmitted
	ptrj.ReportID = result.ReportID
	out, err := json.Marshal(ptrj)
	if err != nil {
		return nil, err
	}

	return out, nil
}
