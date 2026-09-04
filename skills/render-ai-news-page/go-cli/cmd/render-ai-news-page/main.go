package main

import (
	"os"

	"github.com/SherlockGy/ai-news-skills/skills/render-ai-news-page/go-cli/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
