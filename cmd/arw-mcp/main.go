package main

import (
	"fmt"
	"os"

	"github.com/jokerD888/agent-review-workflow/internal/mcp"
)

var version = "0.2.0-dev"

func main() {
	if err := (mcp.Server{Version: version}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "arw-mcp:", err)
		os.Exit(1)
	}
}
