package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/Individual-1/go-efd"
)

const dateLayout = "01/02/2006"

func main() {
	filerType := []efd.FilerType{efd.SenatorFiler}
	startTime, _ := time.Parse(dateLayout, "01/01/2020")
	endTime, _ := time.Parse(dateLayout, "12/31/2020")

	// In order to interact with the EFDSearch backend, we need to create a client
	// This client will manage the necessary authorization and cookies to hit each endpoint
	c := efd.CreateEFDClient("", "")

	// Here we call the SearchReportData client with a FilerType of Senator, and search for the
	// PeriodicTransactionReport and Annual report types
	// This method will return an array of SearchResult objects, which contain details about each line item
	// in the search results
	searchResults, err := c.SearchReportData("", "", filerType, "",
		[]efd.ReportType{efd.AnnualReport}, startTime, endTime)
	if err != nil {
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "./"
	}

	// Now that we have the array of search results, we iterate over them and act upon them
	for _, result := range searchResults {

		// In this example, we are generating a path based on the date the report was uploaded and saving our
		// output json to that path if it does not already exist
		jsonpath := filepath.Join(cwd, "data", fmt.Sprintf("%s.json", result.GenPTRSearchResultPath()))
		if !fileExists(jsonpath) {

			// HandleResult is a wrapper around individual result handlers, and will select the most appropriate one
			// to use based on the ReportFormat parsed out of the search objects
			// Individual handlers can be called too if you want to override this behavior
			parsedReport, err := c.HandleResult(result)
			if err != nil {
				fmt.Println(err)
				continue
			}

			//
			js, err := efd.ReportToJson(result, parsedReport)
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
