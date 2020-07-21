package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaeg/hatbox/hatbox"

	_ "github.com/robertkrimen/otto/underscore"
	log "github.com/sirupsen/logrus"
)

var redisAddr = flag.String("redis-address", "", "the address for the main redis")
var redisPassword = flag.String("redis-password", "", "the password for redis")
var cluster = flag.String("cluster-name", "default", "name of cluster")
var hatboxName = flag.String("hatbox-name", "", "the unique name of this hatbox")
var healthInterval = flag.Duration("health-interval", 5, "Seconds delay for health check")
var hostPort = flag.String("host-port", "80", "HTTP port of hatbox.")
var healthPort = flag.String("health-port", "8787", "Port to run health metrics on")
var configFile = flag.String("config", "", "Config file with hatbox settings")

func main() {
	rand.Seed(time.Now().UnixNano())
	var ctx = context.Background()
	log.SetLevel(log.InfoLevel)

	flag.Parse()
	w, err := hatbox.Create(*configFile, *redisAddr, *redisPassword, *cluster, *hatboxName, *hostPort, *healthPort)

	//Capture sigterm
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		w.Shutdown()
	}()

	if err == nil {
		log.Info("Hatbox Name: ", w.HatboxName)
		log.Info("Hatbox IP: ", w.IP)
		log.Debug("Hatbox Opened")
		for hatbox.IsEnabled(w) {
			//Heart beat
			w.Client.HSet(ctx, w.Cluster+":Hatboxes:"+w.HatboxName, "Heartbeat", time.Now().UnixNano())
			w.RegisterFiles()
			w.SyncFiles()
			time.Sleep(time.Second)
		}
		log.Info("Shutting down.")
		defer w.Shutdown()
	} else {
		log.WithError(err).Error("Failed to start hatbox.")
	}
}
