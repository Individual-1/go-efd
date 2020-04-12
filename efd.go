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
	var searchResults []efdtools.SearchResult
	var err error
	filerType := []efdtools.FilerType{efdtools.SenatorFiler}

	startTime, _ := time.Parse(efdtools.DateLayoutUS, "01/01/2020")
	endTime, _ := time.Parse(efdtools.DateLayoutUS, "04/30/2020")

	searchResults, err = efdtools.SearchReportData("", "", filerType, "",
		[]efdtools.ReportType{efdtools.AnnualReport}, startTime, endTime)
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

	for _, result := range searchResults {
		jsonpath := filepath.Join(cwd, "data", fmt.Sprintf("%s.json", result.GenPTRSearchResultPath()))
		if !fileExists(jsonpath) {
			var parsedReport efdtools.ParsedReport
			var err error

			parsedReport.ReportFormat = result.ReportFormat
			switch result.ReportFormat {
			case efdtools.PTRFormat:
				parsedReport.Transactions, err = efdtools.HandlePTRSearchResult(result)
			case efdtools.AnnualFormat:
				parsedReport.Transactions, err = efdtools.HandleAnnualSearchResult(result)
			case efdtools.PaperFormat:
				parsedReport.Pages, err = efdtools.HandlePaperSearchResult(result)
			}
			if err != nil {
				fmt.Println(err)
				continue
			}

			js, err := efdtools.PTRToJSON(result, parsedReport)
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
