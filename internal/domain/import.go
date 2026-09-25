package domain

// ImportError describes a validation or parsing failure on a specific CSV row.
type ImportError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// ImportResult summarizes the batch transaction intake outcome.
type ImportResult struct {
	Total    int           `json:"total"`
	Imported int           `json:"imported"`
	Failed   int           `json:"failed"`
	Errors   []ImportError `json:"errors"`
}
