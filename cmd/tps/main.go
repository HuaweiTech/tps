package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/tps/handler"
	"github.com/cloudfoundry-incubator/tps/heartbeat"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/diegonats"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var listenAddr = flag.String(
	"listenAddr",
	"0.0.0.0:1518", // p and s's offset in the alphabet, do not change
	"listening address of api server",
)

var diegoAPIURL = flag.String(
	"diegoAPIURL",
	"",
	"URL of diego API",
)

var natsAddresses = flag.String(
	"natsAddresses",
	"127.0.0.1:4222",
	"comma-separated list of NATS addresses (ip:port)",
)

var natsUsername = flag.String(
	"natsUsername",
	"nats",
	"Username to connect to nats",
)

var natsPassword = flag.String(
	"natsPassword",
	"nats",
	"Password for nats user",
)

var heartbeatInterval = flag.Duration(
	"heartbeatInterval",
	60*time.Second,
	"the interval, in seconds, between heartbeats for maintaining presence",
)

var maxInFlightRequests = flag.Int(
	"maxInFlightRequests",
	200,
	"number of requests to handle at a time; any more will receive 503",
)

var dropsondeOrigin = flag.String(
	"dropsondeOrigin",
	"tps",
	"Origin identifier for dropsonde-emitted metrics.",
)

var dropsondeDestination = flag.String(
	"dropsondeDestination",
	"localhost:3457",
	"Destination for dropsonde-emitted metrics.",
)

func main() {
	flag.Parse()

	logger := cf_lager.New("tps")
	initializeDropsonde(logger)
	diegoAPIClient := receptor.NewClient(*diegoAPIURL)
	apiHandler := initializeHandler(logger, *maxInFlightRequests, diegoAPIClient)

	natsClient := diegonats.NewClient()
	cf_debug_server.Run()

	heartbeatRunner := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		actual := heartbeat.New(
			natsClient,
			*heartbeatInterval,
			fmt.Sprintf("http://%s", *listenAddr),
			logger)
		return actual.Run(signals, ready)
	})

	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{"natsClient", diegonats.NewClientRunner(*natsAddresses, *natsUsername, *natsPassword, logger, natsClient)},
		{"heartbeat", heartbeatRunner},
		{"api", http_server.New(*listenAddr, apiHandler)},
	})

	monitor := ifrit.Envoke(sigmon.New(group))

	logger.Info("started")

	err := <-monitor.Wait()
	if err != nil {
		logger.Error("exited", err)
		os.Exit(1)
	}

	logger.Info("exited")
	os.Exit(0)
}

func initializeDropsonde(logger lager.Logger) {
	err := dropsonde.Initialize(*dropsondeDestination, *dropsondeOrigin)
	if err != nil {
		logger.Error("failed to initialize dropsonde: %v", err)
	}
}

func initializeHandler(logger lager.Logger, maxInFlight int, apiClient receptor.Client) http.Handler {
	apiHandler, err := handler.New(apiClient, maxInFlight, logger)
	if err != nil {
		logger.Fatal("initialize-handler.failed", err)
	}

	return dropsonde.InstrumentedHandler(apiHandler)
}
