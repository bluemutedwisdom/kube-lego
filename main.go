package main

import (
	"flag"
	"os"

	"github.com/Shopify/kube-lego/pkg/kubelego"

	log "github.com/Sirupsen/logrus"
	bugsnag "github.com/bugsnag/bugsnag-go"
)

var AppVersion = "unknown"
var AppGitCommit = ""
var AppGitState = ""

func Version() string {
	version := AppVersion
	if len(AppGitCommit) > 0 {
		version += "-"
		version += AppGitCommit[0:8]
	}
	if len(AppGitState) > 0 && AppGitState != "clean" {
		version += "-"
		version += AppGitState
	}
	return version
}

func init() {
	apiKey := os.Getenv("LEGO_BUGSNAG_API_KEY")
	if apiKey == "" {
		log.Fatal("LEGO_BUGSNAG_API_KEY is required to setup Bugsnag")
	}

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:       apiKey,
		ReleaseStage: "production",
	})
}

func main() {
	// parse standard command line arguments
	flag.Parse()

	kl := kubelego.New(Version())
	kl.Init()
}
