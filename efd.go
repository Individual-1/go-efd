package main

import (
	"net/url"

	"github.com/Individual-1/efd/efdtools"
)

func main() {

	const layoutUS = "01/02/2006"
	/*
		startTime, _ := time.Parse(layoutUS, "03/30/2020")
		endTime, _ := time.Parse(layoutUS, "03/31/2020")
	*/
	var searchResults []efdtools.SearchResult

	efdtools.AcceptDisclaimer()
	/*
		searchResults, err := efdtools.SearchSenatorPTR(startTime, endTime)
		if err != nil {
			return
		}
	*/

	testURL, _ := url.Parse("https://efdsearch.senate.gov/search/view/ptr/829529d5-698a-4b58-9af0-8a189cb7a6a8/")
	searchResults = append(searchResults, efdtools.SearchResult{FileURL: testURL})
	for _, result := range searchResults {
		efdtools.HandlePTRSearchResult(result)
	}
}
