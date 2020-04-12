package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/Individual-1/go-efd/efdtools"
)

func main() {
	var ptrSearchResults []efdtools.SearchResult
	var annualSearchResults []efdtools.SearchResult
	var err error
	filerType := []efdtools.FilerType{efdtools.Senator}

	startTime, _ := time.Parse(efdtools.LayoutUS, "01/01/2020")
	endTime, _ := time.Parse(efdtools.LayoutUS, "04/30/2020")

	ptrSearchResults, err = efdtools.SearchReportData("", "", filerType, "",
		[]efdtools.ReportType{efdtools.PeriodicTransactionReport}, startTime, endTime)
	if err != nil {
		return
	}

	annualSearchResults, err = efdtools.SearchReportData("", "", filerType, "",
		[]efdtools.ReportType{efdtools.Annual}, startTime, endTime)
	if err != nil {
		return
	}

	efdtools.AcceptDisclaimer()

	/*
		testURL, _ := url.Parse("https://efdsearch.senate.gov/search/view/ptr/2ba66b69-cdd5-4679-ae70-a588325f9d72/")
		searchResults = append(searchResults, efdtools.SearchResult{FileURL: testURL})
	*/

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "./"
	}
	// Iterate over PTRs first
	for _, result := range ptrSearchResults {
		jsonpath := filepath.Join(cwd, "data", fmt.Sprintf("%s.json", result.GenPTRSearchResultPath()))

		if !fileExists(jsonpath) {
			ptrs, err := efdtools.HandlePTRSearchResult(result)
			if err != nil {
				fmt.Println(err)
				continue
			}

			js, err := efdtools.PTRToJSON(result, ptrs)
			if err != nil {
				fmt.Println(err)
				continue
			}

			os.MkdirAll(filepath.Dir(jsonpath), os.ModePerm)
			ioutil.WriteFile(jsonpath, js, 0644)
		}
	}

	// Then do Annual reports
	for _, result := range annualSearchResults {
		jsonpath := filepath.Join(cwd, "data", fmt.Sprintf("%s.json", result.GenAnnualSearchResultPath()))

		if !fileExists(jsonpath) {
			ptrs, err := efdtools.HandleAnnualSearchResult(result)
			if err != nil {
				fmt.Println(err)
				continue
			}

			js, err := efdtools.PTRToJSON(result, ptrs)
			if err != nil {
				fmt.Println(err)
				continue
			}

			os.MkdirAll(filepath.Dir(jsonpath), os.ModePerm)
			ioutil.WriteFile(jsonpath, js, 0644)
		}
	}

	return
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}
