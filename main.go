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
var host = flag.Bool("host", false, "Allow this wart to be an http host.")
var hostPort = flag.String("host-port", "9999", "HTTP port of wart.")
var healthPort = flag.String("health-port", "8787", "Port to run health metrics on")
var configFile = flag.String("config", "", "Config file with wart settings")

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
		log.Debug("Chest Opened")
		//handle creating new threads.
		for chest.IsEnabled(w) {
			w.Client.HSet(ctx, w.Cluster+":Chest:"+w.ChestName, "Heartbeat", time.Now().UnixNano())
			time.Sleep(time.Second)
		}
		log.Info("Shutting down.")
		if w.Client != nil {
			defer w.Client.HSet(ctx, w.Cluster+":Chests:"+w.ChestName, "State", "offline")
			defer log.Debug("Chest Closed")
		}
	} else {
		log.WithError(err).Error("Failed to start chest.")
	}
}