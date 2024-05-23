package logging

import (
	"strings"
)

// TODO: Get this info dynamically
var resourceTypes = [4]string{"resourcegroups", "storageAccounts", "operationresults", "asyncoperations"}

// Shared logging function for REST API interactions

func GetMethodInfo(method string, rawURL string) string {
	url := strings.Split(rawURL, "?api-version")
	parts := strings.Split(url[0], "/")
	resource := url[0]
	foundResource := false
	// Start from the end of the split path and move backward
	// to get nested resource type
	counter := 0
	for counter = len(parts) - 1; counter >= 0; counter-- {
		currToken := parts[counter]
		for _, rType := range resourceTypes {
			if strings.Compare(currToken, rType) == 0 {
				// Found the appropriate resource type
				foundResource = true
				break
			}
		}
		if foundResource {
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
