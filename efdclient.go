// Package efd implements helper functions for interacting with the efd search and managing the results
package efd

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

var csrfCharset []rune = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// cellType enumerates the types of table cells we handle
type cellType int

// Enumeration of different table cells
const (
	// Do nothing
	ignoreCell cellType = 11

	// Transaction Number
	trNumCell cellType = 1

	// Transaction Date
	trDateCell cellType = 2

	// Owner
	ownerCell cellType = 3

	// Ticker
	tickerCell cellType = 4

	// Asset Name
	assetNameCell cellType = 5

	// Asset Type
	assetTypeCell cellType = 6

	// Transaction Type
	trTypeCell cellType = 7

	// Amount
	amountCell cellType = 8

	// Comment
	commentCell cellType = 9

	// Set Valid
	validCell cellType = 10
)

// anchorData is a struct containing common <a> anchor fields
type anchorData struct {
	HREF   string
	Target string
	Text   string
}

// EFDClient is a wrapper struct containing state and parameters
// for interacting with the efdsearch system
type EFDClient struct {
	jar           http.CookieJar
	client        *http.Client
	baseURL       *url.URL
	homeURL       *url.URL
	searchURL     *url.URL
	searchDataURL *url.URL
	userAgent     string
	dateLayout    string

	// Indicates whether we have the cookies accepting the ToS
	authed bool
}

// CreateEFDClient initializes and returns an EFDClient object
// Empty inputs for useragent or datelayout will set default values
func CreateEFDClient(userAgent string, dateLayout string) EFDClient {
	var c EFDClient

	if dateLayout == "" {
		// Default to 01/02/2006
		dateLayout = "01/02/2006"
	}

	if userAgent == "" {
		// Default to Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:75.0) Gecko/20100101 Firefox/75.0
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:75.0) Gecko/20100101 Firefox/75.0"
	}

	c.initParam(userAgent, dateLayout)

	return c
}

// initParam is an initializer function for an EFDClient struct, it sets up some parameters of the client
// and prepares the underlying HTTP client for use
func (c *EFDClient) initParam(userAgent string, dateLayout string) {
	c.dateLayout = dateLayout
	c.userAgent = userAgent

	c.jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	c.client = &http.Client{Jar: c.jar}

	const baseURLString string = "https://efdsearch.senate.gov"
	const homeURLString string = "/search/home/"
	const searchURLString string = "/search/"
	const searchDataURLString string = "/search/report/data/"

	var homeURLComponent, _ = url.Parse(homeURLString)
	var searchURLComponent, _ = url.Parse(searchURLString)
	var searchDataURLComponent, _ = url.Parse(searchDataURLString)

	c.baseURL, _ = url.Parse(baseURLString)
	c.homeURL = c.baseURL.ResolveReference(homeURLComponent)
	c.searchURL = c.baseURL.ResolveReference(searchURLComponent)
	c.searchDataURL = c.baseURL.ResolveReference(searchDataURLComponent)

	c.authed = false
}

// AcceptDisclaimer goes through the process of accepting the initial disclaimer
// and setting up session data for report retrieval
// This doesn't need to be called explicitly, but can be
func (c *EFDClient) AcceptDisclaimer() error {
	csrftoken, err := c.parseCSRFToken(c.homeURL)
	if err != nil {
		return err
	}

	data := url.Values{}
	data.Set("prohibition_agreement", "1")
	data.Set("csrfmiddlewaretoken", csrftoken)

	req, err := http.NewRequest("POST", c.homeURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Referer", c.homeURL.String())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("User-Agent", c.userAgent)

	_, err = c.client.Do(req)
	if err != nil {
		return err
	}

	c.authed = true

	return nil
}

// SearchSenatorPTR is a wrapper around searchReportData which retrieves a list of Senator's PTRs for a given timeframe
func (c *EFDClient) SearchSenatorPTR(startTime time.Time, endTime time.Time) ([]SearchResult, error) {
	return c.SearchReportData("", "", []FilerType{SenatorFiler}, "", []ReportType{PeriodicTransactionReport}, startTime, endTime)
}

// SearchReportData is a wrapper around searchReportDataPaged which automatically iterates over the full number of results
func (c *EFDClient) SearchReportData(firstName string, lastName string, filerTypes []FilerType, state string, reportTypes []ReportType,
	startTime time.Time, endTime time.Time) ([]SearchResult, error) {
	var finalResults []SearchResult
	var err error
	const length int = 100

	start := 0
	remainder := 1

	// As long as (Total records - start - length) > 0, we have more records to read
	for remainder > 0 {
		var results []SearchResult
		results, remainder, err = c.searchReportDataPaged(firstName, lastName, filerTypes, state, reportTypes, startTime, endTime, start, length)
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
func (c *EFDClient) searchReportDataPaged(firstName string, lastName string, filerTypes []FilerType, state string, reportTypes []ReportType,
	startTime time.Time, endTime time.Time, start int, length int) ([]SearchResult, int, error) {
	csrfToken := c.genCSRFToken()

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
	data.Set("filer_types", intEnumArrayToString(filerTypes, ","))
	// Target state represented
	data.Set("senator_state", state)
	// Report type (Format is [11, 12])
	data.Set("report_types", intEnumArrayToString(reportTypes, ","))
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

	req, err := http.NewRequest("POST", c.searchDataURL.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Add("Referer", c.searchURL.String())
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("User-Agent", c.userAgent)
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

		sResult.DateSubmitted, err = time.Parse(c.dateLayout, result[4])
		if err != nil {
			break
		}

		anchor, err := c.parseAnchor(result[3])
		if err != nil || anchor.HREF == "" {
			break
		}

		docURL, err := url.Parse(anchor.HREF)
		if err != nil {
			break
		}

		sResult.FileURL = c.baseURL.ResolveReference(docURL)
		sResult.ReportFormat = URLToReportFormat(sResult.FileURL)

		// Rewrite URL to be more useful if it is a paper copy
		if sResult.ReportFormat == PaperFormat {
			sResult.FileURL.Path = strings.Replace(sResult.FileURL.Path, "view", "print", 1)
		}

		sResult.ReportName = anchor.Text
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

// HandleResult is a wrapper around other handler types, selecting one based on the ReportType in the request
// It returns a ParsedReport type
func (c *EFDClient) HandleResult(result SearchResult) (ParsedReport, error) {
	var parsedReport ParsedReport
	var err error

	parsedReport.ReportFormat = result.ReportFormat
	switch result.ReportFormat {
	case PTRFormat:
		parsedReport.Transactions, err = c.HandlePTRSearchResult(result)
	case AnnualFormat:
		parsedReport.Transactions, err = c.HandleAnnualSearchResult(result)
	case PaperFormat:
		parsedReport.Pages, err = c.HandlePaperSearchResult(result)
	}

	return parsedReport, err
}

// HandlePTRSearchResult takes a SearchResult struct and parses out transaction from the digital PTR
func (c *EFDClient) HandlePTRSearchResult(result SearchResult) ([]Transaction, error) {
	var ptrTransactions []Transaction
	var err error

	if !c.authed {
		err = c.AcceptDisclaimer()
		if err != nil {
			return nil, err
		}
	}

	resp, err := c.client.Get(result.FileURL.String())
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
	ptrTransactions = make([]Transaction, trs.Length())
	trs.Each(func(i int, s *goquery.Selection) {
		var ptrTransaction *Transaction = &ptrTransactions[i]
		tds := s.ChildrenFiltered("td")
		if tds.Length() != 9 {
			return
		}

		tds.EachWithBreak(func(j int, t *goquery.Selection) bool {

			switch j {
			case 1:
				// Transaction Date
				return c.handleTransactionCell(ptrTransaction, t, trDateCell)
			case 2:
				// Owner
				return c.handleTransactionCell(ptrTransaction, t, ownerCell)
			case 3:
				// Ticker
				return c.handleTransactionCell(ptrTransaction, t, tickerCell)
			case 4:
				// Asset Name
				return c.handleTransactionCell(ptrTransaction, t, assetNameCell)
			case 5:
				// Asset Type
				return c.handleTransactionCell(ptrTransaction, t, assetTypeCell)
			case 6:
				// Transaction Type
				return c.handleTransactionCell(ptrTransaction, t, trTypeCell)
			case 7:
				// Amount
				return c.handleTransactionCell(ptrTransaction, t, amountCell)
			case 8:
				// Comment
				if c.handleTransactionCell(ptrTransaction, t, commentCell) {
					c.handleTransactionCell(ptrTransaction, t, validCell)
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
func (c *EFDClient) HandleAnnualSearchResult(result SearchResult) ([]Transaction, error) {
	var ptrTransactions []Transaction
	var regTransactions []Transaction
	var totalTransactions []Transaction
	var err error

	if !c.authed {
		err = c.AcceptDisclaimer()
		if err != nil {
			return nil, err
		}
	}

	resp, err := c.client.Get(result.FileURL.String())
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
		title, _ := c.removeHTMLSelection(s)

		// Handle modified PTR format in Annual Reports
		if title == "Part 4a. Periodic Transaction Report Summary" {
			section := s.Parent().Parent()
			trs := section.Find("div.table-responsive table.table tbody tr")
			ptrTransactions = make([]Transaction, trs.Length())
			trs.Each(func(u int, r *goquery.Selection) {
				var ptrTransaction *Transaction = &ptrTransactions[u]
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
						return c.handleTransactionCell(ptrTransaction, t, trDateCell)
					case 3:
						// Owner
						return c.handleTransactionCell(ptrTransaction, t, ownerCell)
					case 4:
						// Ticker
						return c.handleTransactionCell(ptrTransaction, t, tickerCell)
					case 5:
						// Asset Name
						return c.handleTransactionCell(ptrTransaction, t, assetNameCell)
					case 6:
						// Transaction Type
						return c.handleTransactionCell(ptrTransaction, t, trTypeCell)
					case 7:
						// Amount
						return c.handleTransactionCell(ptrTransaction, t, amountCell)
					case 8:
						// Comment
						if c.handleTransactionCell(ptrTransaction, t, commentCell) {
							c.handleTransactionCell(ptrTransaction, t, validCell)
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
			regTransactions = make([]Transaction, trs.Length())
			trs.Each(func(u int, r *goquery.Selection) {
				var regTransaction *Transaction = &regTransactions[u]
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
						return c.handleTransactionCell(regTransaction, t, ownerCell)
					case 3:
						// Ticker
						return c.handleTransactionCell(regTransaction, t, tickerCell)
					case 4:
						// Asset Name
						return c.handleTransactionCell(regTransaction, t, assetNameCell)
					case 5:
						// Transaction Type
						return c.handleTransactionCell(regTransaction, t, trTypeCell)
					case 6:
						// Transaction Date
						return c.handleTransactionCell(regTransaction, t, trDateCell)
					case 7:
						// Amount
						return c.handleTransactionCell(regTransaction, t, amountCell)
					case 8:
						// Comment
						if c.handleTransactionCell(regTransaction, t, commentCell) {
							c.handleTransactionCell(regTransaction, t, validCell)
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

// HandlePaperSearchResult takes a SearchResult struct and collects the page URLs from the scanned paper
func (c *EFDClient) HandlePaperSearchResult(result SearchResult) (PaperReport, error) {
	var paperReport PaperReport
	var err error

	if !c.authed {
		err = c.AcceptDisclaimer()
		if err != nil {
			return paperReport, err
		}
	}

	// We do this earlier, but just in case I guess?
	fileURL := result.FileURL
	fileURL.Path = strings.Replace(fileURL.Path, "view", "print", 1)

	resp, err := c.client.Get(fileURL.String())
	if err != nil {
		return paperReport, err
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return paperReport, err
	}

	pages := doc.Find("img.filingImage")
	paperReport.PageURLs = make([]*url.URL, pages.Length())
	pages.Each(func(i int, s *goquery.Selection) {
		var pageURL *url.URL

		pageURLString, exists := s.Attr("src")
		if !exists {
			return
		}

		pageURL, err = url.Parse(pageURLString)
		if err != nil {
			return
		}

		paperReport.PageURLs[i] = pageURL
		return
	})

	return paperReport, nil
}

func (c EFDClient) handleTransactionCell(transaction *Transaction, t *goquery.Selection, ct cellType) bool {
	switch ct {
	case trNumCell:
		// Transaction Number
	case trDateCell:
		// Transaction Date
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Date, err = time.Parse(c.dateLayout, tString)
		if err != nil {
			return false
		}
	case ownerCell:
		// Owner
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Owner = tString
	case tickerCell:
		// Ticker
		tString, err := c.stripHTMLSelection(t)
		if err != nil {
			return false
		}

		if tString == "--" {
			transaction.Ticker = tString
		} else {
			anchor, err := c.parseAnchor(tString)
			if err != nil || anchor.Text == "" {
				return false
			}
			transaction.Ticker = anchor.Text
		}
	case assetNameCell:
		// Asset Name
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.AssetName = tString
	case assetTypeCell:
		// Asset Type
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.AssetType = tString
	case trTypeCell:
		// Transaction Type
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Type = tString
	case amountCell:
		// Amount
		tString, err := c.removeHTMLSelection(t)
		if err != nil {
			return false
		}

		transaction.Amount = tString
	case commentCell:
		// Comment
		tString, err := c.removeHTMLSelection(t)
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
func (c EFDClient) trimHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := s.Html()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(htmlString), nil
}

// stripHTMLSelection is a more aggressive whitespace remover
// It replaces all consecutive whitespace with a single space
func (c EFDClient) stripHTMLSelection(s *goquery.Selection) (string, error) {
	htmlString, err := c.trimHTMLSelection(s)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(htmlString, " "), nil
}

// removeHTMLSelection goes a step further than strip and removed all tag-likes
// This is very unsophisticated, but works for trivial cases
func (c EFDClient) removeHTMLSelection(s *goquery.Selection) (string, error) {
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

// genCSRFToken generates a token for use with the /search/report/data endpoint
// These tokens are 64 characters alphanumeric including upper and lowercase alphabet
func (c EFDClient) genCSRFToken() string {
	b := make([]rune, 64)
	for i := range b {
		b[i] = csrfCharset[rand.Intn(len(csrfCharset))]
	}
	return string(b)
}

// parseCSRFToken parses the `csrfmiddlewaretoken` field from pages with form data
// On success, the token string will be returned
func (c EFDClient) parseCSRFToken(url *url.URL) (string, error) {
	var csrftoken string = ""

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return csrftoken, err
	}

	req.Header.Add("User-Agent", c.userAgent)

	resp, err := c.client.Do(req)
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
func (c EFDClient) parseAnchor(tag string) (anchorData, error) {
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
func (c EFDClient) parseHTMLAnchor(anchor *html.Node) (anchorData, error) {
	var contents anchorData

	if anchor.Type != html.ElementNode || anchor.Data != "a" {
		return contents, errors.New("Node is not an anchor")
	}

	contents.HREF, _ = c.findAttributes(anchor.Attr, "href")
	contents.Target, _ = c.findAttributes(anchor.Attr, "target")

	if anchor.FirstChild != nil {
		contents.Text = anchor.FirstChild.Data
	}

	return contents, nil
}

// findAttributes iterates over an Attribute slice and attempts to find the corresponding key
func (c EFDClient) findAttributes(attrs []html.Attribute, key string) (string, error) {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Val, nil
		}
	}

	return "", fmt.Errorf("Key %s did not exist in attribute", key)
}

// clearClient initializes an empty cookiejar and http client. This should be run to clear all client context.
func (c *EFDClient) clearClient() {
	// Should be safe to run this multiple times
	// There is no cookiejar.Clear type method so we need to create a new one to empty it out
	c.jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	c.client = &http.Client{Jar: c.jar}
}
