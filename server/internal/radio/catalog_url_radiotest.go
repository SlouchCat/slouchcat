//go:build radiotest

package radio

import "os"

// catalogBase returns the e2e catalog of the radiotest build (web/e2e/radio-server.mjs); no mirror
// lookup happens there. Never part of the production binary (plan 3.8).
func catalogBase() (string, bool) { return os.Getenv("SLOUCHCAT_TEST_RADIO_CATALOG_URL"), false }

// wikidataEndpoint returns the SPARQL answer of the e2e radio server (web/e2e/radio-server.mjs,
// /sparql); "" without one, and the job fails instead of asking Wikidata.
func wikidataEndpoint() string { return os.Getenv("SLOUCHCAT_TEST_RADIO_WIKIDATA_URL") }

// wikidataMinItems is 0: the answer of the e2e radio server lists a dozen stations.
func wikidataMinItems() int { return 0 }
