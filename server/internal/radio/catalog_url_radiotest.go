//go:build radiotest

package radio

import "os"

// catalogBase returns the e2e catalog of the radiotest build (web/e2e/radio-server.mjs); no mirror
// lookup happens there. Never part of the production binary (plan 3.8).
func catalogBase() (string, bool) { return os.Getenv("SLOUCHCAT_TEST_RADIO_CATALOG_URL"), false }
