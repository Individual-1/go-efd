// Package efdtools implements helper functions for interacting with the efd search
package efdtools

import (
	"encoding/json"
	"path"
)

// PTRToJSON takes a SearchResult object, parses the resulting PTR, and marshals it into a JSON byte array
func PTRToJSON(result SearchResult) ([]byte, error) {
	var ptrj PTRJson
	ptrs, err := HandlePTRSearchResult(result)
	if err != nil {
		return nil, err
	}

	ptrj.Transactions = ptrs
	ptrj.FirstName = result.FirstName
	ptrj.LastName = result.LastName
	ptrj.PTRURL.URL = result.FileURL
	ptrj.DateSubmitted = result.DateSubmitted
	ptrj.ReportID = path.Base(result.FileURL.Path)
	out, err := json.Marshal(ptrj)
	if err != nil {
		return nil, err
	}

	return out, nil
}
