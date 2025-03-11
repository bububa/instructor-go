package instructor

import "strings"

// Removes any prefixes before the JSON (like "Sure, here you go:")
func trimPrefixBeforeJSON(json *string) string {
	startObject := strings.IndexByte(*json, '{')
	startArray := strings.IndexByte(*json, '[')

	var start int
	if startObject == -1 && startArray == -1 {
		return *json // No opening brace or bracket found, return the original string
	} else if startObject == -1 {
		start = startArray
	} else if startArray == -1 {
		start = startObject
	} else {
		start = min(startObject, startArray)
	}

	return (*json)[start:]
}

// Removes any postfixes after the JSON
func trimPostfixAfterJSON(jsonStr string) string {
	endObject := strings.LastIndexByte(jsonStr, '}')
	endArray := strings.LastIndexByte(jsonStr, ']')

	var end int
	if endObject == -1 && endArray == -1 {
		return jsonStr // No closing brace or bracket found, return the original string
	} else if endObject == -1 {
		end = endArray
	} else if endArray == -1 {
		end = endObject
	} else {
		end = max(endObject, endArray)
	}

	return jsonStr[:end+1]
}

// Extracts the JSON by trimming prefixes and postfixes
func ExtractJSON(json *string) string {
	trimmedPrefix := trimPrefixBeforeJSON(json)
	trimmedJSON := trimPostfixAfterJSON(trimmedPrefix)
	return trimmedJSON
}
