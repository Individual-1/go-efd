// Package efdtools implements helper functions for interacting with the efd search
package efdtools

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"

	"github.com/PuerkitoBio/goquery"
)

var jar http.CookieJar
var client *http.Client
var baseURL *url.URL
var homeURL *url.URL
var searchURL *url.URL
var searchDataURL *url.URL

var csrfCharset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

const userAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:75.0) Gecko/20100101 Firefox/75.0"
const layoutUS = "01/02/2006"

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
	Valid         bool
	FirstName     string
	LastName      string
	FullName      string
	FileURL       *url.URL
	DateSubmitted time.Time
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
	Date      time.Time
	Owner     string
	Ticker    string
	AssetName string
	AssetType string
	Type      string
	Amount    string
	Comment   string
	ReportID  string
	Valid     bool
}

type anchorData struct {
	HREF   string
	Target string
	Text   string
}

func init() {
	jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	var proxyURL, _ = url.Parse("http://172.31.208.1:8080")
	var tr = &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client = &http.Client{Jar: jar, Transport: tr}

	const baseURLString string = "https://efdsearch.senate.gov"
	const homeURLString string = "/search/home/"
	const searchURLString string = "/search/"
	const searchDataURLString string = "/search/report/data/"

	var homeURLComponent, _ = url.Parse(homeURLString)
	var searchURLComponent, _ = url.Parse(searchURLString)
	var searchDataURLComponent, _ = url.Parse(searchDataURLString)

	baseURL, _ = url.Parse(baseURLString)
	homeURL = baseURL.ResolveReference(homeURLComponent)
	searchURL = baseURL.ResolveReference(searchURLComponent)
	searchDataURL = baseURL.ResolveReference(searchDataURLComponent)
}

// AcceptDisclaimer goes through the process of accepting the initial disclaimer
// and setting up session data for search
func AcceptDisclaimer() error {
	csrftoken, err := parseCSRFToken(homeURL)
	if err != nil {
		return err
	}

	data := url.Values{}
	data.Set("prohibition_agreement", "1")
	data.Set("csrfmiddlewaretoken", csrftoken)

	req, err := http.NewRequest("POST", homeURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Referer", homeURL.String())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("User-Agent", userAgent)

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// SearchSenatorPTR is a wrapper around searchReportData which retrieves a list of Senator's PTRs for a given timeframe
func SearchSenatorPTR(startTime time.Time, endTime time.Time) ([]SearchResult, error) {
	return searchReportData("", "", []FilerType{Senator}, "", []ReportType{PeriodicTransactionReport}, startTime, endTime)
}

// searchReportData calls the /search/report/data/ endpoint to get a list of records for the provided inputs
// For both Time inputs, only Day, Month, and Year will be used
func searchReportData(firstName string, lastName string, filerTypes []FilerType, state string, reportTypes []ReportType, startTime time.Time, endTime time.Time) ([]SearchResult, error) {
	csrfToken := genCSRFToken()

	startTimeString := fmt.Sprintf("%02d/%02d/%04d 00:00:00",
		startTime.Month(), startTime.Day(), startTime.Year())

	endTimeString := fmt.Sprintf("%02d/%02d/%04d 23:59:59",
		endTime.Month(), endTime.Day(), endTime.Year())

	data := url.Values{}

	// Target first name
	data.Set("first_name", firstName)
	// Target last name
	data.Set("last_name", lastName)
	// Target filer type (Format is [1, 2])
	data.Set("filer_types", intArrayToString(filerTypes, ","))
	// Target state represented
	data.Set("senator_state", state)
	// Report type (Format is [11, 12])
	data.Set("report_types", intArrayToString(reportTypes, ","))
	// Beginning of date range to search (Format is MM/DD/YYYY HH:MM:SS)
	data.Set("submitted_start_date", startTimeString)
	// End of date range to search (Format is MM/DD/YYYY HH:MM:SS)
	data.Set("submitted_end_date", endTimeString)
	// CSRF token
	data.Set("csrftoken", csrfToken)

	req, err := http.NewRequest("POST", searchDataURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Referer", searchURL.String())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("X-CSRFToken", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrftoken", Value: csrfToken})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.Header.Get("content-type") != "application/json" {
		return nil, errors.New("Response content type is not json")
	}

	var results SearchResults
	var searchResults []SearchResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return nil, err
	} else if results.Result != "ok" {
		return nil, errors.New("Search results endpoint returned error status")
	}

	searchResults = make([]SearchResult, len(results.Data))
	for i, result := range results.Data {
		// If this array is not 5 strings long, then it is malformed
		if len(result) != 5 {
			break
		}

		searchResults[i].DateSubmitted, err = time.Parse(layoutUS, result[4])
		if err != nil {
			break
		}

		anchor, err := parseAnchor(result[3])
		if err != nil || anchor.HREF == "" {
			break
		}

		docURL, err := url.Parse(anchor.HREF)
		if err != nil {
			break
		}

		searchResults[i].FileURL = baseURL.ResolveReference(docURL)
		searchResults[i].FirstName = strings.ToLower(result[0])
		searchResults[i].LastName = strings.ToLower(result[1])
		searchResults[i].FullName = strings.ToLower(result[2])

		searchResults[i].Valid = true
	}

	searchFiltered := searchResults[:0]
	for _, record := range searchResults {
		if record.Valid {
			searchFiltered = append(searchFiltered, record)
		}
	}

	return searchFiltered, nil
}

// HandlePTRSearchResult takes a SearchResult struct and parses out transaction from the digital PTR
func HandlePTRSearchResult(result SearchResult) ([]PTRTransaction, error) {
	var ptrTransactions []PTRTransaction
	reportID := path.Base(result.FileURL.Path)

	resp, err := client.Get(result.FileURL.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// PTR table rows have 9 elements each
	// Transaction #, Transaction Date, Owner, Ticker, Asset Name, Asset Type, Transaction Type, Amount, and Comment
	trs := doc.Find("div.table-responsive table.table tbody tr")
	ptrTransactions = make([]PTRTransaction, trs.Length())
	trs.Each(func(i int, s *goquery.Selection) {
		tds := s.ChildrenFiltered("td")
		if tds.Length() != 9 {
			return
		}

		ptrTransactions[i].ReportID = reportID
		tds.EachWithBreak(func(j int, t *goquery.Selection) bool {

			switch j {
			case 1:
				// Transaction Date
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].Date, err = time.Parse(layoutUS, tString)
				if err != nil {
					return false
				}
			case 2:
				// Owner
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].Owner = tString
			case 3:
				// Ticker
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				anchor, err := parseAnchor(tString)
				if err != nil || anchor.Text == "" {
					return false
				}
				ptrTransactions[i].Ticker = anchor.Text
			case 4:
				// Asset Name
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].AssetName = tString
			case 5:
				// Asset Type
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].AssetType = tString
			case 6:
				// Transaction Type
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].Type = tString
			case 7:
				// Amount
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].Amount = tString
			case 8:
				// Comment
				tString, err := trimHTMLSelection(t)
				if err != nil {
					return false
				}

				ptrTransactions[i].Comment = tString
				ptrTransactions[i].Valid = true
			}

			return true
		})
	})

	ptrFiltered := ptrTransactions[:0]
	for _, record := range ptrTransactions {
		if record.Valid {
			ptrFiltered = append(ptrFiltered, record)
		}
	}

	return ptrFiltered, nil
}

// trimHTMLSelection takes a goquery selection and retrieves the innerHTML,
// then filters out newlines and whitespace from either end
func trimHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := s.Html()
	if err != nil {
		return "", err
	}

	return strings.Trim(htmlString, "\n "), nil
}

// intArrayToString takes an array of integers and joins them into a delimited string
// This is primary used for joining ReportType and FilerType arrays to usable formats
// This is not necessarily particularly performant
func intArrayToString(ints []int, delim string) string {
	strs := []string{}
	for _, val := range ints {
		strs = append(strs, strconv.Itoa(val))
	}

	return "[" + strings.Join(strs, delim) + "]"
}

// genCSRFToken generates a token for use with the /search/report/data endpoint
// These tokens are 64 characters alphanumeric including upper and lowercase alphabet
func genCSRFToken() string {
	b := make([]rune, 64)
	for i := range b {
		b[i] = csrfCharset[rand.Intn(len(csrfCharset))]
	}
	return string(b)
}

// parseCSRFToken parses the `csrfmiddlewaretoken` field from pages with form data
// On success, the token string will be returned
func parseCSRFToken(url *url.URL) (string, error) {
	var csrftoken string = ""

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return csrftoken, err
	}

	req.Header.Add("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("Request returned non-200 status code")
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	doc.Find("[name=csrfmiddlewaretoken]").First().Each(func(i int, s *goquery.Selection) {
		csrftoken, _ = s.Attr("value")
	})

	if csrftoken == "" {
		return "", errors.New("Failed to parse csrftoken")
	}

	return csrftoken, nil
}

// parseAnchor takes in a string and attempts to parse it as if it were an <a> anchor tag
// On success it returns the structured contents of the field
func parseAnchor(tag string) (anchorData, error) {
	var contents anchorData

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(tag))
	if err != nil {
		return contents, err
	}

	a := doc.Find("a").First()
	contents.HREF, _ = a.Attr("href")
	contents.Target, _ = a.Attr("target")
	contents.Text = a.Text()

	return contents, nil
}

// parseHTMLAnchor takes in an html node and attempts to parse out its contents
// On success it returns the structured contents of the field
func parseHTMLAnchor(anchor *html.Node) (anchorData, error) {
	var contents anchorData

	if anchor.Type != html.ElementNode || anchor.Data != "a" {
		return contents, errors.New("Node is not an anchor")
	}

	contents.HREF, _ = findAttributes(anchor.Attr, "href")
	contents.Target, _ = findAttributes(anchor.Attr, "target")

	if anchor.FirstChild != nil {
		contents.Text = anchor.FirstChild.Data
	}

	return contents, nil
}

// findAttributes iterates over an Attribute slice and attempts to find the corresponding key
func findAttributes(attrs []html.Attribute, key string) (string, error) {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Val, nil
		}
	}

	return "", fmt.Errorf("Key %s did not exist in attribute", key)
}

// clearClient initializes an empty cookiejar and http client. This should be run to clear all client context.
func clearClient() {
	// Should be safe to run this multiple times
	// There is no cookiejar.Clear type method so we need to create a new one to empty it out
	jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client = &http.Client{Jar: jar}
}
