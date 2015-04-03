package tpsrunner

import (
	"os/exec"

	"github.com/tedsuo/ifrit/ginkgomon"
)

func NewListener(bin string, listenAddr string, diegoAPIURL string) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name: "tps-listener",
		Command: exec.Command(
			bin,
			"-diegoAPIURL", diegoAPIURL,
			"-listenAddr", listenAddr,
		),
		StartCheck: "tps-listener.started",
	})
}

func NewWatcher(bin string, diegoAPIURL string, ccBaseURL string) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name: "tps-watcher",
		Command: exec.Command(
			bin,
			"-diegoAPIURL", diegoAPIURL,
			"-ccBaseURL", ccBaseURL,
		),
		StartCheck: "tps-watcher.started",
	})
}
