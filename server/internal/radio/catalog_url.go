//go:build !radiotest

package radio

// catalogBase returns the base URL of the Radio Browser API and whether mirrors may stand in for
// it. all.api resolves round-robin to the mirrors and carries the wildcard certificate.
func catalogBase() (string, bool) { return "https://all.api.radio-browser.info", true }

// wikidataEndpoint returns the SPARQL endpoint of Wikidata (B7.3).
func wikidataEndpoint() string { return "https://query.wikidata.org/sparql" }
