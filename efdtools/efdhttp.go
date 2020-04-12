// Package efdtools implements helper functions for interacting with the efd search
package efdtools

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"regexp"
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

// cellType enumerates the types of table cells we handle
type ptrCellType = int

// Enumeration of different table cells
const (
	// Do nothing
	ignoreCell ptrCellType = 11

	// Transaction Number
	trNumCell ptrCellType = 1

	// Transaction Date
	trDateCell ptrCellType = 2

	// Owner
	ownerCell ptrCellType = 3

	// Ticker
	tickerCell ptrCellType = 4

	// Asset Name
	assetNameCell ptrCellType = 5

	// Asset Type
	assetTypeCell ptrCellType = 6

	// Transaction Type
	trTypeCell ptrCellType = 7

	// Amount
	amountCell ptrCellType = 8

	// Comment
	commentCell ptrCellType = 9

	// Set Valid
	validCell ptrCellType = 10
)

// anchorData is a struct containing common <a> anchor fields
type anchorData struct {
	HREF   string
	Target string
	Text   string
}

func init() {
	jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	/*
		var proxyURL, _ = url.Parse("http://172.31.208.1:8080")
		var tr = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	*/
	client = &http.Client{Jar: jar}

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
	return SearchReportData("", "", []FilerType{Senator}, "", []ReportType{PeriodicTransactionReport}, startTime, endTime)
}

// SearchReportData is a wrapper around searchReportDataPaged which automatically iterates over the full number of results
func SearchReportData(firstName string, lastName string, filerTypes []FilerType, state string, reportTypes []ReportType,
	startTime time.Time, endTime time.Time) ([]SearchResult, error) {
	var finalResults []SearchResult
	var err error
	const length int = 100

	start := 0
	remainder := 1

	// As long as (Total records - start - length) > 0, we have more records to read
	for remainder > 0 {
		var results []SearchResult
		results, remainder, err = searchReportDataPaged(firstName, lastName, filerTypes, state, reportTypes, startTime, endTime, start, length)
		if err != nil {
			return nil, err
		}

		finalResults = append(finalResults, results...)

		start = start + length
	}

	return finalResults, nil
}

// searchReportDataPaged calls the /search/report/data/ endpoint to get a list of records for the provided inputs
// For both Time inputs, only Day, Month, and Year will be used
// start and length indicate the result number to start from and length to go
// Returns search results, number of records remaining, and error status
func searchReportDataPaged(firstName string, lastName string, filerTypes []FilerType, state string, reportTypes []ReportType,
	startTime time.Time, endTime time.Time, start int, length int) ([]SearchResult, int, error) {
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
	// Starting entry to read from
	data.Set("start", strconv.Itoa(start))
	// Number of entries to read
	data.Set("length", strconv.Itoa(length))
	// CSRF token
	data.Set("csrftoken", csrfToken)

	req, err := http.NewRequest("POST", searchDataURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Add("Referer", searchURL.String())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("X-CSRFToken", csrfToken)
	req.AddCookie(&http.Cookie{Name: "csrftoken", Value: csrfToken})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	if resp.Header.Get("content-type") != "application/json" {
		return nil, 0, errors.New("Response content type is not json")
	}

	var results SearchResults
	var searchResults []SearchResult
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return nil, 0, err
	} else if results.Result != "ok" {
		return nil, 0, errors.New("Search results endpoint returned error status")
	}

	searchResults = make([]SearchResult, len(results.Data))
	for i, result := range results.Data {
		var sResult *SearchResult = &searchResults[i]
		// If this array is not 5 strings long, then it is malformed
		if len(result) != 5 {
			break
		}

		sResult.DateSubmitted, err = time.Parse(layoutUS, result[4])
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

		sResult.FileURL = baseURL.ResolveReference(docURL)
		sResult.ReportType = anchor.Text
		sResult.ReportID = path.Base(sResult.FileURL.Path)
		sResult.FirstName = strings.ToLower(result[0])
		sResult.LastName = strings.ToLower(result[1])
		sResult.FullName = strings.ToLower(result[2])

		sResult.Valid = true
	}

	searchFiltered := searchResults[:0]
	for _, record := range searchResults {
		if record.Valid {
			searchFiltered = append(searchFiltered, record)
		}
	}

	remainder := results.RecordsTotal - start - length

	return searchFiltered, remainder, nil
}

// HandlePTRSearchResult takes a SearchResult struct and parses out transaction from the digital PTR
func HandlePTRSearchResult(result SearchResult) ([]PTRTransaction, error) {
	var ptrTransactions []PTRTransaction

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
		var ptrTransaction *PTRTransaction = &ptrTransactions[i]
		tds := s.ChildrenFiltered("td")
		if tds.Length() != 9 {
			return
		}

		tds.EachWithBreak(func(j int, t *goquery.Selection) bool {

			switch j {
			case 1:
				// Transaction Date
				return handlePTRCell(ptrTransaction, t, trDateCell)
			case 2:
				// Owner
				return handlePTRCell(ptrTransaction, t, ownerCell)
			case 3:
				// Ticker
				return handlePTRCell(ptrTransaction, t, tickerCell)
			case 4:
				// Asset Name
				return handlePTRCell(ptrTransaction, t, assetNameCell)
			case 5:
				// Asset Type
				return handlePTRCell(ptrTransaction, t, assetTypeCell)
			case 6:
				// Transaction Type
				return handlePTRCell(ptrTransaction, t, trTypeCell)
			case 7:
				// Amount
				return handlePTRCell(ptrTransaction, t, amountCell)
			case 8:
				// Comment
				if handlePTRCell(ptrTransaction, t, commentCell) {
					handlePTRCell(ptrTransaction, t, validCell)
				} else {
					return false
				}
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

// HandleAnnualSearchResult takes a SearchResult struct and parses out transaction from the digital Annual report
// Structured very similarly to HandlePTRSearchResult with some minor column ordering differences
// TODO: Can we consolidate this and PTRSearchResult handler?
// TODO: Handle scanned documents, maybe search for `div.img-wrap` ?
func HandleAnnualSearchResult(result SearchResult) ([]PTRTransaction, error) {
	var ptrTransactions []PTRTransaction
	var regTransactions []PTRTransaction
	var totalTransactions []PTRTransaction

	resp, err := client.Get(result.FileURL.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Section 4a PTR table rows have 8 elements each
	// Transaction #, Transaction Date, Owner, Ticker, Asset Name, Transaction Type, Amount, and Comment
	// Section 4b Transactions have 8 elements each
	// Transaction #, Owner, Ticker, Asset Name, Transaction Type, Transaction Date, Amount, and Comment
	hdrs := doc.Find("section.card div.card-body h3.h4")
	hdrs.Each(func(i int, s *goquery.Selection) {
		title, _ := removeHTMLSelection(s)

		// Handle modified PTR format in Annual Reports
		if title == "Part 4a. Periodic Transaction Report Summary" {
			section := s.Parent().Parent()
			trs := section.Find("div.table-responsive table.table tbody tr")
			ptrTransactions = make([]PTRTransaction, trs.Length())
			trs.Each(func(u int, r *goquery.Selection) {
				var ptrTransaction *PTRTransaction = &ptrTransactions[u]
				tds := r.ChildrenFiltered("td")
				if tds.Length() != 9 {
					return
				}

				tds.EachWithBreak(func(j int, t *goquery.Selection) bool {

					switch j {
					case 0:
						// whitespace
					case 1:
						// Transaction Number
					case 2:
						// Transaction Date
						return handlePTRCell(ptrTransaction, t, trDateCell)
					case 3:
						// Owner
						return handlePTRCell(ptrTransaction, t, ownerCell)
					case 4:
						// Ticker
						return handlePTRCell(ptrTransaction, t, tickerCell)
					case 5:
						// Asset Name
						return handlePTRCell(ptrTransaction, t, assetNameCell)
					case 6:
						// Transaction Type
						return handlePTRCell(ptrTransaction, t, trTypeCell)
					case 7:
						// Amount
						return handlePTRCell(ptrTransaction, t, amountCell)
					case 8:
						// Comment
						if handlePTRCell(ptrTransaction, t, commentCell) {
							handlePTRCell(ptrTransaction, t, validCell)
						} else {
							return false
						}
					}

					return true
				})
			})
		} else if title == "Part 4b. Transactions" {
			section := s.Parent().Parent()
			trs := section.Find("div.table-responsive table.table tbody tr")
			regTransactions = make([]PTRTransaction, trs.Length())
			trs.Each(func(u int, r *goquery.Selection) {
				var regTransaction *PTRTransaction = &regTransactions[u]
				tds := r.ChildrenFiltered("td")
				if tds.Length() != 9 {
					return
				}

				tds.EachWithBreak(func(j int, t *goquery.Selection) bool {

					switch j {
					case 0:
						// whitespace
					case 1:
						// Transaction Number
					case 2:
						// Owner
						return handlePTRCell(regTransaction, t, ownerCell)
					case 3:
						// Ticker
						return handlePTRCell(regTransaction, t, tickerCell)
					case 4:
						// Asset Name
						return handlePTRCell(regTransaction, t, assetNameCell)
					case 5:
						// Transaction Type
						return handlePTRCell(regTransaction, t, trTypeCell)
					case 6:
						// Transaction Date
						return handlePTRCell(regTransaction, t, trDateCell)
					case 7:
						// Amount
						return handlePTRCell(regTransaction, t, amountCell)
					case 8:
						// Comment
						if handlePTRCell(regTransaction, t, commentCell) {
							handlePTRCell(regTransaction, t, validCell)
						} else {
							return false
						}
					}

					return true
				})
			})
		}
	})

	totalTransactions = append(ptrTransactions, regTransactions...)

	totalFiltered := totalTransactions[:0]
	for _, record := range totalTransactions {
		if record.Valid {
			totalFiltered = append(totalFiltered, record)
		}
	}

	return totalFiltered, nil
}

func handlePTRCell(transaction *PTRTransaction, t *goquery.Selection, ct ptrCellType) bool {
	switch ct {
	case trNumCell:
		// Transaction Number
	case trDateCell:
		// Transaction Date
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Date, err = time.Parse(LayoutUS, tString)
		if err != nil {
			return false
		}
	case ownerCell:
		// Owner
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Owner = tString
	case tickerCell:
		// Ticker
		tString, err := stripHTMLSelection(t)
		if err != nil {
			return false
		}

		if tString == "--" {
			transaction.Ticker = tString
		} else {
			anchor, err := parseAnchor(tString)
			if err != nil || anchor.Text == "" {
				return false
			}
			transaction.Ticker = anchor.Text
		}
	case assetNameCell:
		// Asset Name
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.AssetName = tString
	case assetTypeCell:
		// Asset Type
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.AssetType = tString
	case trTypeCell:
		// Transaction Type
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Type = tString
	case amountCell:
		// Amount
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Amount = tString
	case commentCell:
		// Comment
		tString, err := removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Comment = tString
	case validCell:
		// Set Valid
		transaction.Valid = true
	case ignoreCell:
		// NOP
	}

	return true
}

// trimHTMLSelection takes a goquery selection and retrieves the innerHTML,
// then filters out newlines and whitespace from either end
func trimHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := s.Html()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(htmlString), nil
}

// stripHTMLSelection is a more aggressive whitespace remover
// It replaces all consecutive whitespace with a single space
func stripHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := trimHTMLSelection(s)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(htmlString, " "), nil
}

// removeHTMLSelection goes a step further than strip and removed all tag-likes
// This is very unsophisticated, but works for trivial cases
func removeHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := s.Html()
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`<.*?>`)
	space := regexp.MustCompile(`\s+`)
	str := re.ReplaceAllString(htmlString, " ")
	str = strings.TrimSpace(str)
	return space.ReplaceAllString(str, " "), nil
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
