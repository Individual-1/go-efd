# Golang tools for https://efdsearch.senate.gov

This is a set of Golang tools for interacting with [EFD Search](https://efdsearch.senate.gov).
It provides an interface to search and read documents, with additional handling for certain transaction types.

## Getting Started

Import the package into your project then, use an `EFDClient` to interact with the EFD Search backend.

```
import "github.com/Individual-1/go-efd"

var client efd.EFDClient = efd.CreateEFDClient("My user agent", "My date format")
```

This client can then be used to search and retrieve results from EFD via `SearchReportData` and `HandleResult` methods.

```
client.SearchReportData("First name", "Last name", []efd.FilterType{SenatorFiler}, "State", 
        []efd.ReportType{efd.AnnualReport, efd.PeriodicTransactionReport},
        startTime, endTime)
```

```
var result efd.SearchResult = ...

parsedReport, err := client.HandleResult(result)
```
The parsed report can be converted to json via use of `ReportToJson` or can be manipulated directly.

```
json, err := efd.ReportToJson(result, parsedReport)
```

## License

This project is licensed under ??
