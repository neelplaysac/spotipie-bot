package main

import (
	"flag"
	"os"

	"github.com/neelplaysac/spotipie-go/internal/app"
)

func main() {
	testMonitoring := flag.Bool("sentry-test", false, "send a test error to GlitchTip and exit without starting the bot")
	flag.Parse()
	os.Exit(app.Run(*testMonitoring))
}
