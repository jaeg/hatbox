package hatbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-redis/redis/v8"
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

//Hatbox main structure for hatbox
type Hatbox struct {
	RedisAddr       string
	RedisPassword   string
	Cluster         string
	HatboxName       string
	Client          *redis.Client
	Healthy         bool
	SecondsTillDead int
	VMStopChan      chan func()
	shuttingDown    bool
	IP              string
}
type fileInfo struct {
	hatboxName string
	time      int64
}

//Create Creates a hatbox
func Create(configFile string, redisAddr string, redisPassword string, cluster string, hatboxName string, hostPort string, healthPort string) (*Hatbox, error) {
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
				if m["name"] != nil {
					hatboxName = m["name"].(string)
				}
			}
		}
	}

	if len(hatboxName) == 0 {
		hatboxName, _ = os.Hostname() //generateRandomName(10)
	}
	c := &Hatbox{RedisAddr: redisAddr, RedisPassword: redisPassword,
		Cluster: cluster, HatboxName: hatboxName,
		Healthy: true, SecondsTillDead: 1, IP: getIPAddress()}

	if c.RedisAddr == "" {
		return nil, errors.New("no redis address provided")
	}

	c.Client = redis.NewClient(&redis.Options{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword, // no password set
		DB:       0,               // use default DB
	})

	pong, pongErr := c.Client.Ping(ctx).Result()

	if pongErr != nil && pong != "PONG" {
		return nil, errors.New("redis failed ping")
	}
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName, "IP", c.IP)
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName, "Port", hostPort)
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName, "State", ONLINE)
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName, "Status", ENABLED)

	http.HandleFunc("/", c.handleEndpoint)
	go func() { http.ListenAndServe(":"+hostPort, nil) }()

	// create `ServerMux`
	mux := http.NewServeMux()

	// create a default route handler
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		pong, pongErr := c.Client.Ping(ctx).Result()

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

	return c, nil
}

func generateRandomName(length int) (out string) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	for i := 0; i < length; i++ {
		out += string(chars[rand.Intn(len(chars))])
	}

	return
}

func getIPAddress() (ip string) {
	tt, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, t := range tt {
		aa, err := t.Addrs()
		if err != nil {
			panic(err)
		}
		for _, a := range aa {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || v4[0] == 127 { // loopback address
				continue
			}
			ip = v4.To4().String()
			fmt.Printf("%v\n", v4)
		}
	}

	return
}

//LookUpFile looks up the file in redis to see has the newest copy
func (c *Hatbox) LookUpFile(path string) {

}

//RegisterFiles registers files in hatbox to redis
func (c *Hatbox) RegisterFiles() {
	// Iterate over files in the "/hatbox" folder
	// - In redis update Cluster:Hatboxes:HatboxName:Contents has with date file was updated and name/path.

	err := filepath.Walk("./contents",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == "./contents" {
				return nil
			}

			if c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path).Val() == "" {
				//Brand new file
				log.Info("File needs added ", path)
				c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path, info.ModTime().UnixNano())
				c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path+"<Local>", info.ModTime().UnixNano())
			} else {
				//Get the local time I registered this file
				localTimeVal := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path+"<Local>").Val()
				localTime, err := strconv.ParseInt(localTimeVal, 10, 0)
				if err == nil {
					if localTime < info.ModTime().UnixNano() {
						log.Info("File needs updated! ", path)
						//If local time registered is older than the file then register this mod time to redis
						c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path, info.ModTime().UnixNano())
						c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path+"<Local>", info.ModTime().UnixNano())
					}
				} else {
					log.WithError(err).Error("Error getting local time from redis")
				}
			}

			return nil
		})
	if err != nil {
		log.Println(err)
	}
}

//SyncFiles syncs files in hatbox with redis
func (c *Hatbox) SyncFiles() {
	keys := c.Client.Keys(ctx, c.Cluster+":Hatboxes:*:Contents").Val()

	fileMap := make(map[string]*fileInfo)

	for i := range keys {
		key := keys[i]
		keySplit := strings.Split(key, ":")

		hatboxName := keySplit[2]
		hatboxHeartbeat, _ := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+hatboxName, "Heartbeat").Int64()
		//If the heart isn't beating don't try to sync with it.
		if time.Now().UnixNano()-hatboxHeartbeat < int64(10*time.Second) {
			files := c.Client.HGetAll(ctx, key).Val()
			for fileName, fileDate := range files {
				if strings.Index(fileName, "<Local>") == -1 {
					t, err := strconv.ParseInt(fileDate, 10, 0)
					if err == nil {
						if fileMap[fileName] != nil {
							// If file is newer than in the map use it instead.
							if t > fileMap[fileName].time {
								fileMap[fileName].time = t
								fileMap[fileName].hatboxName = hatboxName
							}
						} else {
							fileMap[fileName] = &fileInfo{hatboxName: hatboxName, time: t}
						}
					} else {
						log.WithError(err).Error("Error getting local time from redis")
					}
				}
			}
		}

	}

	for path, info := range fileMap {
		originTime, _ := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path).Int64()

		if info.hatboxName != c.HatboxName && info.time != originTime {
			log.Info("Pull the file ", path, " from ", info.hatboxName)
			//HTTP request to the hatbox that has the file
			c.pullFile(path, info)
		}
	}
}

//SyncFile syncs file in hatbox with redis
func (c *Hatbox) SyncFile(path string) (found bool) {
	keys := c.Client.Keys(ctx, c.Cluster+":Hatboxes:*:Contents").Val()
	found = false
	fileMap := make(map[string]*fileInfo)
	originTime, _ := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path).Int64()
	for i := range keys {
		key := keys[i]
		keySplit := strings.Split(key, ":")

		hatboxName := keySplit[2]
		hatboxHeartbeat, _ := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+hatboxName, "Heartbeat").Int64()
		//If the heart isn't beating don't try to sync with it.
		if time.Now().UnixNano()-hatboxHeartbeat < int64(10*time.Second) {
			fileDate, err := c.Client.HGet(ctx, key, path).Result()
			if err != nil {
				log.WithError(err).Error("Error getting file information from redis", hatboxName, path)
				continue
			}

			t, err := strconv.ParseInt(fileDate, 10, 0)
			if err == nil {
				found = true
				if fileMap[path] != nil {
					// If file is newer than in the map use it instead.
					if t > fileMap[path].time {
						fileMap[path].time = t
						fileMap[path].hatboxName = hatboxName
					}
				} else {
					fileMap[path] = &fileInfo{hatboxName: hatboxName, time: t}
				}
			} else {
				log.WithError(err).Error("Error getting local time from redis")
			}
		}
	}

	for path, info := range fileMap {
		if info.hatboxName != c.HatboxName && info.time != originTime {
			log.Info("Pull the file ", path, " from ", info.hatboxName)
			//HTTP request to the hatbox that has the file
			c.pullFile(path, info)
		}
	}

	return
}

func (c *Hatbox) pullFile(path string, info *fileInfo) error {
	ip, err := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+info.hatboxName, "IP").Result()
	if err != nil {
		log.WithError(err).Error("Error getting ip of hatbox")
		return err
	}
	port, err := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+info.hatboxName, "Port").Result()
	if err != nil {
		log.WithError(err).Error("Error getting port of hatbox")
		return err
	}
	resp, err := http.Get("http://" + ip + ":" + port + "/" + path)

	if err != nil {
		log.WithError(err).Error("Error getting file from hatbox")
		return err
	}
	defer resp.Body.Close()
	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		log.WithError(err).Error("Error creating file")
		return err
	}
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.WithError(err).Error("Error writing file to disk")
		return err
	}

	fInfo, _ := file.Stat()
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path, info.time)
	c.Client.HSet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents", path+"<Local>", fInfo.ModTime().UnixNano())

	c.Client.Del(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":FileReturn:"+path)

	return nil
}

//Shutdown Shutsdown the hatbox by safely stopping threads
func (c *Hatbox) Shutdown() {
	log.Info("Shutting down requested")
	c.shuttingDown = true
	c.Client.Del(ctx, c.Cluster+":Hatboxes:"+c.HatboxName)
	c.Client.Del(ctx, c.Cluster+":Hatboxes:"+c.HatboxName+":Contents")

}

//IsEnabled Returns if the hatbox is enabled.
func IsEnabled(c *Hatbox) bool {
	status := c.Client.HGet(ctx, c.Cluster+":Hatboxes:"+c.HatboxName, "Status").Val()
	if c.shuttingDown || status == DISABLED {
		return false
	}
	return true
}

func (c *Hatbox) handleEndpoint(w http.ResponseWriter, r *http.Request) {
	if c.Healthy {
		if r.Method == http.MethodGet {
			path := strings.Replace(r.URL.Path, "/", "", 1)
			if !c.SyncFile(path) {
				http.Error(w, "Yar, nothing here", http.StatusNotFound)
				return
			}

			file, err := os.Open(path)
			if err != nil {
				log.WithError(err).Error("Error opening file")
				http.Error(w, "Yar, nothing here", http.StatusNotFound)
				return
			}
			defer file.Close()
			io.Copy(w, file)
		}
	}
}
