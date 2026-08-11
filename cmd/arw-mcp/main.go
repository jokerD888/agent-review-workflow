package main

import (
	"fmt"
	"os"

	"github.com/jokerD888/agent-review-workflow/internal/mcp"
)

func main() {
	if err := (mcp.Server{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "arw-mcp:", err)
		os.Exit(1)
	}
}
