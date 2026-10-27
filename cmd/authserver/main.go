package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/neelplaysac/spotipie-go/internal/app"
)

func main() {
	check := flag.Bool("healthcheck", false, "check local HTTP readiness and exit")
	flag.Parse()
	if *check {
		if err := app.CheckAuthServer(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	os.Exit(app.RunAuthServer())
}
