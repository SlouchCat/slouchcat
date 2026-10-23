// Command apicheck is the third-party client of criterion P-16 (plan section 8, acceptance row
// C-5): it knows the server only through api/openapi.yaml, speaks plain HTTP against a running
// instance and runs every viewer function the criterion lists.
//
// It imports no package of this repository on purpose — that is what "Fremd-Client" means, and
// the depguard rule "apicheck" in server/.golangci.yml keeps it that way. Everything it sends and
// every answer it reads comes from the contract: before each call it looks the path and the
// method up in openapi.yaml, and a call the contract does not describe fails the run.
//
// Usage:
//
//	apicheck -base http://127.0.0.1:8099 -spec ../api/openapi.yaml -testdata ../testdata
//
// The output is the evidence of the acceptance: one line per viewer function with "ok" or
// "fehlgeschlagen", the operations it used and what the server answered. The exit code is 0 only
// when every line is ok. With -report the same table is written as Markdown.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// downloadTimeout bounds how long the client waits for a prepared download.
const downloadTimeout = 10 * time.Minute

// result is one row of the report.
type result struct {
	name     string
	ok       bool
	detail   string
	failure  string
	ops      []string
	duration time.Duration
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8099", "base URL of the running server")
	spec := flag.String("spec", "api/openapi.yaml", "the contract, relative to the repository root")
	data := flag.String("testdata", "testdata", "test data directory with media/ and podcasts/")
	report := flag.String("report", "", "write the table as Markdown to this file as well")
	wait := flag.Duration("wait", time.Minute, "how long to wait for the server to become healthy")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout of a single HTTP request")
	budget := flag.Duration("budget", 30*time.Minute, "time budget of the whole run")
	flag.Parse()

	if err := run(*base, *spec, *data, *report, *wait, *timeout, *budget); err != nil {
		fmt.Fprintf(os.Stderr, "apicheck: %v\n", err)
		os.Exit(2)
	}
}

// run builds the client, prepares the data and walks the checks. It returns an error only when
// the run itself could not happen; a failed check ends the process with exit code 1.
func run(base, spec, data, report string, wait, timeout, budget time.Duration) error {
	contract, err := loadContract(spec)
	if err != nil {
		return err
	}
	c, err := newClient(base, contract, timeout)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancelBudget := context.WithTimeout(ctx, budget)
	defer cancelBudget()

	feed, err := startFeed(ctx, data, 2)
	if err != nil {
		return err
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		feed.stop(stopCtx)
	}()

	s := &state{c: c, testdata: data, feed: feed}
	started := time.Now()
	if err := prepareAll(ctx, s, wait); err != nil {
		return err
	}

	results := make([]result, 0, len(checks()))
	failed := 0
	for _, check := range checks() {
		c.begin()
		start := time.Now()
		detail, err := check.run(ctx, s)
		row := result{
			name: check.name, ok: err == nil, detail: detail,
			ops: c.used, duration: time.Since(start),
		}
		if err != nil {
			row.failure = err.Error()
			failed++
		}
		results = append(results, row)
		if ctx.Err() != nil {
			return fmt.Errorf("run stopped: %w", ctx.Err())
		}
	}

	document := render(contract, base, results, started, c.calls)
	fmt.Print(document)
	if report != "" {
		if err := os.WriteFile(report, []byte("# Fremd-Client-Test P-16\n\n"+document), 0o644); err != nil { //nolint:gosec // G306: a report to read and copy, no secrets
			return fmt.Errorf("write report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Bericht geschrieben: %s\n", report)
	}
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

// prepareAll builds every fixture the checks need and leaves the server logged out with a PIN on
// the only profile, so the login and the profile picker have something to do.
func prepareAll(ctx context.Context, s *state, wait time.Duration) error {
	if err := s.waitForServer(ctx, wait); err != nil {
		return err
	}
	if err := s.prepare(ctx); err != nil {
		return fmt.Errorf("prepare the installation: %w", err)
	}
	if err := s.seedNotification(ctx); err != nil {
		return fmt.Errorf("prepare the inbox: %w", err)
	}
	if err := s.seedAnnouncement(ctx); err != nil {
		return fmt.Errorf("prepare the announcement: %w", err)
	}
	if err := s.seedTrailer(ctx); err != nil {
		return fmt.Errorf("prepare the trailer: %w", err)
	}
	if err := s.lockProfile(ctx); err != nil {
		return fmt.Errorf("prepare the profile PIN: %w", err)
	}
	return nil
}

// render builds the report: one row per viewer function of P-16, then the summary.
func render(c *contract, base string, results []result, started time.Time, calls int) string {
	failed := 0
	for _, r := range results {
		if !r.ok {
			failed++
		}
	}
	header := [4]string{"Zuschauerfunktion (P-16)", "Status", "Operationen", "Beleg"}
	rows := make([][4]string, 0, len(results))
	for _, r := range results {
		status := "ok"
		evidence := r.detail
		if !r.ok {
			status = "fehlgeschlagen"
			evidence = r.failure
		}
		rows = append(rows, [4]string{r.name, status, strings.Join(r.ops, ", "), evidence})
	}
	width := [4]int{}
	for i, title := range header {
		width[i] = len(title)
	}
	for _, row := range rows {
		for i, cell := range row {
			if n := len([]rune(cell)); n > width[i] {
				width[i] = n
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Vertrag:   %s %s, %d Operationen\n", c.title, c.version, len(c.operations))
	fmt.Fprintf(&b, "Server:    %s%s\n", base, c.basePath)
	fmt.Fprintf(&b, "Lauf:      %s, Dauer %s, %d Aufrufe\n\n",
		started.Format(time.RFC3339), time.Since(started).Round(time.Second), calls)
	writeRow(&b, width, header)
	writeSeparator(&b, width)
	for _, row := range rows {
		writeRow(&b, width, row)
	}
	fmt.Fprintf(&b, "\nZusammenfassung: %d von %d Zuschauerfunktionen ok, %d fehlgeschlagen.\n",
		len(results)-failed, len(results), failed)
	b.WriteString("Nicht geprüft: \"Wünschen\" ist mit dem M9-Zuschnitt vom 22.10.2026 ersatzlos " +
		"entfallen (Entscheid 10) und steht nicht mehr im Vertrag.\n")
	return b.String()
}

// writeRow writes one Markdown table row padded to the column widths.
func writeRow(b *strings.Builder, width [4]int, cells [4]string) {
	b.WriteString("|")
	for i, cell := range cells {
		fmt.Fprintf(b, " %s%s |", cell, strings.Repeat(" ", width[i]-len([]rune(cell))))
	}
	b.WriteString("\n")
}

// writeSeparator writes the Markdown separator below the header.
func writeSeparator(b *strings.Builder, width [4]int) {
	b.WriteString("|")
	for _, n := range width {
		fmt.Fprintf(b, " %s |", strings.Repeat("-", n))
	}
	b.WriteString("\n")
}

// unmarshal decodes a JSON body and names the operation in the error.
func unmarshal(body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode answer: %w", err)
	}
	return nil
}
