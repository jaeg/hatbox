package chest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"

	"github.com/go-redis/redis/v8"

	//This is how you import underscore
	_ "github.com/robertkrimen/otto/underscore"
)

//Status Constants

//DISABLED disabled
const DISABLED = "disabled"

//CRASHED crashed
const CRASHED = "crashed"

//ONLINE online
const ONLINE = "online"

//ENABLED enabled
const ENABLED = "enabled"

//STOPPED stopped
const STOPPED = "stopped"

//RUNNING running
const RUNNING = "running"

var ctx = context.Background()

//Chest main structure for wart
type Chest struct {
	RedisAddr       string
	RedisPassword   string
	Cluster         string
	ChestName       string
	Client          *redis.Client
	Healthy         bool
	SecondsTillDead int
	VMStopChan      chan func()
	shuttingDown    bool
}

//Create Creates a wart
func Create(configFile string, redisAddr string, redisPassword string, cluster string, chestName string, host bool, hostPort string, healthPort string) (*Chest, error) {
	if configFile != "" {
		fBytes, err := ioutil.ReadFile(configFile)
		if err == nil {
			var f interface{}
			err2 := json.Unmarshal(fBytes, &f)
			if err2 == nil {
				m := f.(map[string]interface{})
				redisAddr = m["redis-address"].(string)
				redisPassword = m["redis-password"].(string)
				cluster = m["cluster"].(string)
				chestName = m["name"].(string)
				host = m["host"].(bool)
			}
		}
	}

	if len(chestName) == 0 {
		chestName = generateRandomName(10)
	}
	w := &Chest{RedisAddr: redisAddr, RedisPassword: redisPassword,
		Cluster: cluster, ChestName: chestName,
		Healthy: true, SecondsTillDead: 1}

	if w.RedisAddr == "" {
		return nil, errors.New("no redis address provided")
	}

	w.Client = redis.NewClient(&redis.Options{
		Addr:     w.RedisAddr,
		Password: w.RedisPassword, // no password set
		DB:       0,               // use default DB
	})

	pong, pongErr := w.Client.Ping(ctx).Result()

	if pongErr != nil && pong != "PONG" {
		return nil, errors.New("redis failed ping")
	}

	w.Client.HSet(ctx, w.Cluster+":Chests:"+w.ChestName, "State", ONLINE)
	w.Client.HSet(ctx, w.Cluster+":Chests:"+w.ChestName, "Status", ENABLED)

	if host {
		http.HandleFunc("/", w.handleEndpoint)
		go func() { http.ListenAndServe(":"+hostPort, nil) }()
	}

	// create `ServerMux`
	mux := http.NewServeMux()

	// create a default route handler
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		pong, pongErr := w.Client.Ping(ctx).Result()

		if pongErr != nil && pong != "PONG" {
			http.Error(res, "Unhealthy", 500)
		} else {
			fmt.Fprint(res, "{}")
		}
	})

	// create new server
	healthServer := http.Server{
		Addr:    fmt.Sprintf(":%v", healthPort), // :{port}
		Handler: mux,
	}
	go func() { healthServer.ListenAndServe() }()

	return w, nil
}

func generateRandomName(length int) (out string) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	for i := 0; i < length; i++ {
		out += string(chars[rand.Intn(len(chars))])
	}

	return
}

//Shutdown Shutsdown the wart by safely stopping threads
func (w *Chest) Shutdown() {
	w.shuttingDown = true
}

//IsEnabled Returns if the wart is enabled.
func IsEnabled(w *Chest) bool {
	status := w.Client.HGet(ctx, w.Cluster+":Chests:"+w.ChestName, "Status").Val()
	if w.shuttingDown || status == DISABLED {
		return false
	}
	return true
}

func (w *Chest) handleEndpoint(writer http.ResponseWriter, r *http.Request) {
	if w.Healthy {
		http.Error(writer, "Yar, nothing here", http.StatusNotFound)
	}
}
