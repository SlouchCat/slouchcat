package ciworkflows

import (
	"strings"
	"testing"
)

// valid is the shape of ci.yml in small: a job in the gate, an informational one, the gate.
const valid = `
jobs:
  changes:
    name: changes (docs only?)
    runs-on: ubuntu-24.04
  web:
    name: web
    needs: changes
  emulator:
    name: emulator (informational)
    needs: changes
    continue-on-error: true
  ci:
    name: ci
    if: always()
    needs: [changes, web]
    steps:
      - name: Every job of this workflow must be green
        run: |
          check web           '${{ needs.web.result }}'
`

func check(t *testing.T, workflow string, informational ...string) []string {
	t.Helper()
	if informational == nil {
		informational = []string{"emulator (informational)"}
	}
	problems, err := Check([]byte(workflow), informational)
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestValidWorkflow(t *testing.T) {
	if p := check(t, valid); len(p) != 0 {
		t.Fatalf("want no problems, got %q", p)
	}
}

func TestBrokenRules(t *testing.T) {
	cases := []struct {
		name, from, to, want string
		informational        []string
	}{
		{"job missing from needs", "needs: [changes, web]", "needs: [changes]", `job "web" is not in the needs`, nil},
		{"job without a check line", "          check web           '${{ needs.web.result }}'\n", "", `job "web" needs exactly one line`, nil},
		{"check line for another job", "check web           '${{ needs.web.result }}'", "check web '${{ needs.changes.result }}'", "label and job differ", nil},
		{"check line twice", "          check web           '${{ needs.web.result }}'\n", "          check web '${{ needs.web.result }}'\n          check web '${{ needs.web.result }}'\n", "(found 2)", nil},
		{"gate without always", "    if: always()\n", "", "needs `if: always()`", nil},
		{"gate renamed", "    name: ci\n", "    name: gate\n", `is named "gate"`, nil},
		{"changes renamed", "name: changes (docs only?)", "name: changes", `must be named "changes (docs only?)"`, nil},
		{"informational without continue-on-error", "    continue-on-error: true\n", "", "needs `continue-on-error: true`", nil},
		{"informational job also in needs", "needs: [changes, web]", "needs: [changes, web, emulator]", "is informational and in the gate's needs", nil},
		{"unlisted job outside the gate", "", "", `job "emulator" is not in the needs`, []string{}},
		{"listed name that no job has", "", "", `"gone" (.github/ci-informational-jobs.txt) is no job`, []string{"emulator (informational)", "gone"}},
		{"gate job with continue-on-error", "  web:\n    name: web\n    needs: changes\n", "  web:\n    name: web\n    needs: changes\n    continue-on-error: true\n", `job "web" is in the gate's needs and has continue-on-error`, nil},
		{"gate job with continue-on-error from an expression", "  web:\n    name: web\n    needs: changes\n", "  web:\n    name: web\n    needs: changes\n    continue-on-error: ${{ matrix.experimental }}\n", `job "web" is in the gate's needs and has continue-on-error`, nil},
		{"gate checks a job outside its needs", "          check web           '${{ needs.web.result }}'\n", "          check web           '${{ needs.web.result }}'\n          check emulator '${{ needs.emulator.result }}'\n", `the gate checks "emulator", which is not in its needs`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wf := valid
			if c.from != "" {
				if !strings.Contains(wf, c.from) {
					t.Fatalf("fixture lacks %q", c.from)
				}
				wf = strings.Replace(wf, c.from, c.to, 1)
			}
			info := c.informational
			if info == nil {
				info = []string{"emulator (informational)"}
			}
			problems, err := Check([]byte(wf), info)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), c.want) {
				t.Fatalf("want a problem containing %q, got %q", c.want, problems)
			}
		})
	}
}

// grouped is valid with the change detection (M9): web runs only for its group.
const grouped = `
jobs:
  changes:
    name: changes (docs only?)
    outputs:
      run_web: ${{ steps.affected.outputs.run_web }}
      run_e2e: ${{ steps.affected.outputs.run_e2e }}
  web:
    name: web
    needs: changes
    if: needs.changes.outputs.docs_only != 'true' && needs.changes.outputs.run_web == 'true'
  lint:
    name: lint
    needs: changes
  ci:
    name: ci
    if: always()
    needs: [changes, web, lint]
    steps:
      - name: Every job of this workflow must be green
        env:
          RUN_WEB: ${{ needs.changes.outputs.run_web }}
          RUN_E2E: ${{ needs.changes.outputs.run_e2e }}
        run: |
          check web  '${{ needs.web.result }}' "$RUN_WEB"
          check lint '${{ needs.lint.result }}'
`

func TestGroupRules(t *testing.T) {
	if p := check(t, grouped, []string{}...); len(p) != 0 {
		t.Fatalf("want no problems, got %q", p)
	}
	cases := []struct{ name, from, to, want string }{
		{"gate line names another group", `"$RUN_WEB"`, `"$RUN_E2E"`, `job "web" runs only when run_web: its gate line must end in "$RUN_WEB" (found "RUN_E2E")`},
		{"gate line without the flag", `'${{ needs.web.result }}' "$RUN_WEB"`, `'${{ needs.web.result }}'`, `its gate line must end in "$RUN_WEB" (found "")`},
		{"flag on a job without a group", `check lint '${{ needs.lint.result }}'`, `check lint '${{ needs.lint.result }}' "$RUN_WEB"`, `job "lint" has no group condition`},
		{"two groups in one condition", `needs.changes.outputs.run_web == 'true'`, `needs.changes.outputs.run_web == 'true' && needs.changes.outputs.run_e2e == 'true'`, "the gate can accept its skip for one group only"},
		{"flag the gate step does not set", "          RUN_WEB: ${{ needs.changes.outputs.run_web }}\n", "", "names $RUN_WEB, which the gate step does not set"},
		{"flag read from another output", "RUN_WEB: ${{ needs.changes.outputs.run_web }}", "RUN_WEB: ${{ needs.changes.outputs.run_e2e }}", "RUN_WEB must be ${{ needs.changes.outputs.run_web }}"},
		{"group the changes job does not output", "      run_web: ${{ steps.affected.outputs.run_web }}\n", "", `which the job "changes" does not output`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(grouped, c.from) {
				t.Fatalf("fixture lacks %q", c.from)
			}
			problems, err := Check([]byte(strings.Replace(grouped, c.from, c.to, 1)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), c.want) {
				t.Fatalf("want a problem containing %q, got %q", c.want, problems)
			}
		})
	}
}

func TestParseInformational(t *testing.T) {
	got := ParseInformational([]byte("# comment\n\nandroid-emulator (flow, informational)\n  \n# end\n"))
	if len(got) != 1 || got[0] != "android-emulator (flow, informational)" {
		t.Fatalf("got %q", got)
	}
}

func TestUnparsableWorkflow(t *testing.T) {
	if _, err := Check([]byte("jobs: [unclosed"), nil); err == nil {
		t.Fatal("want a parse error")
	}
}
