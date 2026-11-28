// Package ciworkflows checks the rules of .github/workflows/ci.yml that a reviewer is likely to
// miss and no linter knows (ADR 0013, standing rule; docs/perf/ci-optimierungsplan.md, M7):
//
//   - every job is watched: it is in the needs of the gate job `ci` and in the gate's list of
//     `check <job> '${{ needs.<job>.result }}'` lines, or it is an informational job named in
//     .github/ci-informational-jobs.txt with continue-on-error, never both;
//   - a job the gate needs has no continue-on-error: its result must be able to turn the gate red;
//   - the gate runs always (`if: always()`) and keeps its name `ci`, the status check a merge needs;
//   - the job `changes` keeps the name .github/scripts/already-tested.sh looks for;
//   - a job that runs only for its group of the change detection (M9,
//     `needs.changes.outputs.run_<group> == 'true'` in its `if:`) names that same group in its gate
//     line (`check <job> '...' "$RUN_<GROUP>"`), the gate step reads RUN_<GROUP> from that output
//     of `changes`, and a job without such a condition has no flag: the gate accepts a skip as
//     green only for the group whose answer made the job skip.
//
// A job missing from the gate would run without anyone looking at its result; that is the price
// ADR 0013 accepted for naming every job, and this check pays it.
package ciworkflows

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// GateJob is the id and the name of the gate job; ChangesName the name already-tested.sh needs.
const (
	GateJob     = "ci"
	ChangesJob  = "changes"
	ChangesName = "changes (docs only?)"
	gateStep    = "Every job of this workflow must be green"
)

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	Name            string            `yaml:"name"`
	Needs           yaml.Node         `yaml:"needs"`
	If              string            `yaml:"if"`
	ContinueOnError yaml.Node         `yaml:"continue-on-error"`
	Outputs         map[string]string `yaml:"outputs"`
	Steps           []step            `yaml:"steps"`
}

type step struct {
	Name string            `yaml:"name"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

var (
	// check <job> '${{ needs.<job>.result }}' ["$RUN_<GROUP>"]
	checkLine = regexp.MustCompile(`(?m)^\s*check\s+(\S+)\s+'\$\{\{\s*needs\.([A-Za-z0-9_-]+)\.result\s*\}\}'(?:[ \t]+"\$([A-Z0-9_]+)")?`)
	groupIf   = regexp.MustCompile(`needs\.` + ChangesJob + `\.outputs\.(run_[a-z0-9_]+)`)
	runEnv    = regexp.MustCompile(`^\$\{\{\s*needs\.` + ChangesJob + `\.outputs\.(run_[a-z0-9_]+)\s*\}\}$`)
)

// ParseInformational reads the job names of .github/ci-informational-jobs.txt.
func ParseInformational(data []byte) []string {
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	return names
}

// Check returns one message per broken rule, sorted; none means the workflow keeps every rule.
func Check(data []byte, informational []string) ([]string, error) {
	var wf workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	gate, ok := wf.Jobs[GateJob]
	if !ok {
		return []string{fmt.Sprintf("no gate job %q", GateJob)}, nil
	}
	if gate.Name != GateJob {
		add("the gate job %q is named %q; the merge waits for the status check %q", GateJob, gate.Name, GateJob)
	}
	if strings.TrimSpace(gate.If) != "always()" {
		add("the gate job %q needs `if: always()`, or a failed job turns it skipped instead of red", GateJob)
	}
	if ch, ok := wf.Jobs[ChangesJob]; !ok || ch.Name != ChangesName {
		add("the job %q must be named %q (.github/scripts/already-tested.sh looks for that name)", ChangesJob, ChangesName)
	}

	needs := strings_(gate.Needs)
	checked := map[string]int{}
	flag := map[string]string{}   // job -> the RUN_<GROUP> its gate line names, "" for none
	flagOf := map[string]string{} // RUN_<GROUP> -> the output of `changes` the gate step reads
	for _, s := range gate.Steps {
		if s.Name != gateStep {
			continue
		}
		for name, value := range s.Env {
			if !strings.HasPrefix(name, "RUN_") {
				continue
			}
			m := runEnv.FindStringSubmatch(strings.TrimSpace(value))
			if m == nil || strings.ToUpper(m[1]) != name {
				add("the gate step's %s must be ${{ needs.%s.outputs.%s }} (found %q)", name, ChangesJob, strings.ToLower(name), value)
				continue
			}
			flagOf[name] = m[1]
		}
		for _, m := range checkLine.FindAllStringSubmatch(s.Run, -1) {
			if m[1] != m[2] {
				add("the gate's line `check %s` reads needs.%s.result: label and job differ", m[1], m[2])
			}
			checked[m[2]]++
			flag[m[2]] = m[3]
		}
	}
	changesOutputs := wf.Jobs[ChangesJob].Outputs

	byName := map[string][]string{}
	for id, j := range wf.Jobs {
		byName[j.Name] = append(byName[j.Name], id)
	}
	info := map[string]bool{}
	for _, name := range informational {
		ids := byName[name]
		switch {
		case len(ids) == 0:
			add("informational job %q (.github/ci-informational-jobs.txt) is no job of the workflow", name)
			continue
		case len(ids) > 1:
			add("informational job name %q belongs to %d jobs", name, len(ids))
		}
		for _, id := range ids {
			info[id] = true
			if v := strings.TrimSpace(wf.Jobs[id].ContinueOnError.Value); v != "true" {
				add("informational job %q needs `continue-on-error: true`", id)
			}
		}
	}

	ids := make([]string, 0, len(wf.Jobs))
	for id := range wf.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if id == GateJob {
			continue
		}
		inNeeds := slices.Contains(needs, id)
		switch {
		case info[id] && inNeeds:
			add("job %q is informational and in the gate's needs; it is one or the other", id)
		case !info[id] && !inNeeds:
			add("job %q is not in the needs of the gate %q: nobody would see its result (or list it in .github/ci-informational-jobs.txt)", id, GateJob)
		case inNeeds && strings.TrimSpace(wf.Jobs[id].ContinueOnError.Value) != "" && strings.TrimSpace(wf.Jobs[id].ContinueOnError.Value) != "false":
			add("job %q is in the gate's needs and has continue-on-error: its failure could not turn the gate red", id)
		case inNeeds && id != ChangesJob && checked[id] != 1:
			add("job %q needs exactly one line `check %s '${{ needs.%s.result }}'` in the gate step %q (found %d)", id, id, id, gateStep, checked[id])
		}
		if !inNeeds || id == ChangesJob {
			continue
		}
		var groups []string
		for _, m := range groupIf.FindAllStringSubmatch(wf.Jobs[id].If, -1) {
			if !slices.Contains(groups, m[1]) {
				groups = append(groups, m[1])
			}
		}
		switch {
		case len(groups) > 1:
			add("job %q runs only for the groups %s; the gate can accept its skip for one group only", id, strings.Join(groups, ", "))
		case len(groups) == 1:
			want := strings.ToUpper(groups[0])
			if _, ok := changesOutputs[groups[0]]; !ok {
				add("job %q runs only when needs.%s.outputs.%s, which the job %q does not output", id, ChangesJob, groups[0], ChangesJob)
			}
			if flag[id] != want {
				add("job %q runs only when %s: its gate line must end in \"$%s\" (found %q)", id, groups[0], want, flag[id])
			} else if _, ok := flagOf[want]; !ok {
				add("the gate line of job %q names $%s, which the gate step does not set", id, want)
			}
		case flag[id] != "":
			add("job %q has no group condition, but its gate line accepts a skip by $%s", id, flag[id])
		}
	}
	for id := range checked {
		if !slices.Contains(needs, id) {
			add("the gate checks %q, which is not in its needs", id)
		}
	}
	sort.Strings(problems)
	return problems, nil
}

// strings_ reads `needs:` as a list, whether it is written as one name or as a list.
func strings_(n yaml.Node) []string {
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}
	case yaml.SequenceNode:
		out := make([]string, 0, len(n.Content))
		for _, c := range n.Content {
			out = append(out, c.Value)
		}
		return out
	}
	return nil
}
