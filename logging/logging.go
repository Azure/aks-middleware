package logging

import (
	"strings"
)

var resourceTypes = map[string]bool{
	"resourcegroups":   true,
	"storageaccounts":  true,
	"operationresults": true,
	"asyncoperations":  true,
}

// Shared logging function for REST API interactions
func GetMethodInfo(method string, rawURL string) string {
	url := strings.Split(rawURL, "?api-version")
	parts := strings.Split(url[0], "/")
	resource := url[0]
	counter := 0
	// Start from the end of the split path and move backward
	// to get nested resource type
	for counter = len(parts) - 1; counter >= 0; counter-- {
		currToken := parts[counter]
		if resourceTypes[strings.ToLower(currToken)] {
			resource = currToken
			break
		}
	}

	if method == "GET" {
		// resource name is specified, so it is a READ op
		if counter != len(parts)-1 {
			resource = resource + " - READ"
		} else {
			resource = resource + " - LIST"
		}
	}

	// REST VERB + Resource Type
	methodInfo := method + " " + resource

	return methodInfo
}
