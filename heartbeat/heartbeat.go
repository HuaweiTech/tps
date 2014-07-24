package heartbeat

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/lager"
)

const HeatbeatSubject = "service.announce.tps"

type HeartbeatMessage struct {
	Addr string `json:"addr"`
	TTL  uint   `json:"ttl"`
}

type HeartbeatRunner struct {
	natsAddresses     string
	natsUsername      string
	natsPassword      string
	heartbeatInterval time.Duration
	serviceAddress    string
	logger            lager.Logger
}

func New(natsAddresses, natsUsername, natsPassword string, heartbeatInterval time.Duration, serviceAddress string, logger lager.Logger) *HeartbeatRunner {
	heartbeatLogger := logger.Session("heartbeater")
	return &HeartbeatRunner{
		natsAddresses:     natsAddresses,
		natsUsername:      natsUsername,
		natsPassword:      natsPassword,
		heartbeatInterval: heartbeatInterval,
		serviceAddress:    serviceAddress,
		logger:            heartbeatLogger,
	}
}

func (hr *HeartbeatRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	natsClient, err := initializeNatsClient(hr.natsAddresses, hr.natsUsername, hr.natsPassword)
	if err != nil {
		hr.logger.Error("init-failure-connecting-to-nats", err)
		return err
	}

	ticker := time.NewTicker(hr.heartbeatInterval)
	heartbeatChan := make(chan error)

	close(ready)

	inflight := true
	go hr.heartbeat(natsClient, heartbeatChan)

	for {
		select {
		case <-signals:
			return nil

		case err := <-heartbeatChan:
			inflight = false
			if err != nil {
				hr.logger.Error("failed", err)
				return err
			}

		case <-ticker.C:
			if inflight == true {
				continue
			}
			inflight = true
			go hr.heartbeat(natsClient, heartbeatChan)
		}
	}
}

func (hr *HeartbeatRunner) heartbeat(natsClient yagnats.NATSClient, heartbeatChan chan<- error) {
	msg := HeartbeatMessage{
		Addr: hr.serviceAddress,
		TTL:  ttlFromHeartbeatInterval(hr.heartbeatInterval),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		heartbeatChan <- fmt.Errorf("could not marshal HeartbeatMessage: %s", err)
		return
	}

	hr.logger.Info("will-heartbeat")
	err = natsClient.Publish(HeatbeatSubject, payload)
	if err != nil {
		heartbeatChan <- err
		return
	}

	hr.logger.Info("heartbeat")

	heartbeatChan <- nil
}

func ttlFromHeartbeatInterval(heartbeatInterval time.Duration) uint {
	heartbeatSecs := uint(heartbeatInterval / time.Second)
	if heartbeatSecs == 0 {
		heartbeatSecs = 1
	}
	return heartbeatSecs * 3
}

func initializeNatsClient(natsAddresses, natsUsername, natsPassword string) (yagnats.NATSClient, error) {
	natsClient := yagnats.NewClient()

	natsMembers := []yagnats.ConnectionProvider{}
	for _, addr := range strings.Split(natsAddresses, ",") {
		natsMembers = append(
			natsMembers,
			&yagnats.ConnectionInfo{
				Addr:     addr,
				Username: natsUsername,
				Password: natsPassword,
			},
		)
	}

	err := natsClient.Connect(&yagnats.ConnectionCluster{
		Members: natsMembers,
	})

	return natsClient, err
}
