// Command ciworkflows checks that every job of the CI workflow is watched by the gate (task
// ci:workflows): go run ./tools/ciworkflows/cmd/ciworkflows <ci.yml> <ci-informational-jobs.txt>
package main

import (
	"fmt"
	"os"

	"github.com/slouchcat/slouchcat/server/tools/ciworkflows"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: ciworkflows <ci.yml> <ci-informational-jobs.txt>")
		os.Exit(2)
	}
	workflow, err := os.ReadFile(os.Args[1]) //nolint:gosec // the repository's own workflow file, named on the command line by task ci:workflows
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	list, err := os.ReadFile(os.Args[2]) //nolint:gosec // the repository's own list, named on the command line by task ci:workflows
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	problems, err := ciworkflows.Check(workflow, ciworkflows.ParseInformational(list))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, os.Args[1]+": "+p)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
	fmt.Println("ciworkflows: every job of the workflow is watched by the gate")
}
