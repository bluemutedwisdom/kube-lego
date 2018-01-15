package main

import (
	"flag"
	"os"

	"github.com/Shopify/kube-lego/pkg/kubelego"

	"github.com/Shopify/logrus-bugsnag"
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

func setupBugsnag() {
	apiKey := os.Getenv("LEGO_BUGSNAG_API_KEY")
	if apiKey == "" {
		log.Fatal("LEGO_BUGSNAG_API_KEY is required to setup Bugsnag")
	}

	bugsnag.Configure(bugsnag.Configuration{
		APIKey:       apiKey,
		ReleaseStage: "production",
		Synchronous:  true,
	})

	hook, err := logrus_bugsnag.NewBugsnagHook()
	if err != nil {
		log.Fatal("error happened while seting up logrus Bugsnag hook", err)
	}
	log.AddHook(hook)
}

func main() {
	setupBugsnag()

	// parse standard command line arguments
	flag.Parse()

	kl := kubelego.New(Version())
	kl.Init()
}
