// Command itemsaccess runs the itemsaccess analyzer: go run ./tools/itemsaccess/cmd/itemsaccess ./...
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/slouchcat/slouchcat/server/tools/itemsaccess"
)

func main() {
	singlechecker.Main(itemsaccess.Analyzer)
}
