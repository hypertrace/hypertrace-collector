package matcher

// Matcher allows to match and filter regexs in key and string values for attributes.
type Matcher interface {
	// Looks into the key to decide whether filter the value or not
	FilterKeyRegexs(keyToMatch string, actualKey string, value string, path string) (isRedacted bool, redactedValue string)

	// Looks into the string value to decide whether filter the value or not
	FilterStringValueRegexs(value string, key string, path string) (isRedacted bool, redactedValue string)
}
