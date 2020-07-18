package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaeg/treasurechest/chest"

	_ "github.com/robertkrimen/otto/underscore"
	log "github.com/sirupsen/logrus"
)

var redisAddr = flag.String("redis-address", "", "the address for the main redis")
var redisPassword = flag.String("redis-password", "", "the password for redis")
var cluster = flag.String("cluster-name", "default", "name of cluster")
var chestName = flag.String("chest-name", "", "the unique name of this chest")
var healthInterval = flag.Duration("health-interval", 5, "Seconds delay for health check")
var host = flag.Bool("host", false, "Allow this chest to be an http host.")
var hostPort = flag.String("host-port", "80", "HTTP port of chest.")
var healthPort = flag.String("health-port", "8787", "Port to run health metrics on")
var configFile = flag.String("config", "", "Config file with chest settings")

func main() {
	rand.Seed(time.Now().UnixNano())
	var ctx = context.Background()
	log.SetLevel(log.InfoLevel)

	flag.Parse()
	w, err := chest.Create(*configFile, *redisAddr, *redisPassword, *cluster, *chestName, *host, *hostPort, *healthPort)

	//Capture sigterm
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		w.Shutdown()
	}()

	if err == nil {
		log.Info("Chest Name: ", w.ChestName)
		log.Info("Chest IP: ", w.IP)
		log.Debug("Chest Opened")
		for chest.IsEnabled(w) {
			//Heart beat
			w.Client.HSet(ctx, w.Cluster+":Chests:"+w.ChestName, "Heartbeat", time.Now().UnixNano())
			w.RegisterFiles()
			w.SyncFiles()
			time.Sleep(time.Second)
		}
		log.Info("Shutting down.")
		defer w.Shutdown()
	} else {
		log.WithError(err).Error("Failed to start chest.")
	}
}
