package chest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
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

//Chest main structure for chest
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
type fileInfo struct {
	chestName string
	time      int64
}

//Create Creates a chest
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
		chestName, _ = os.Hostname() //generateRandomName(10)
	}
	c := &Chest{RedisAddr: redisAddr, RedisPassword: redisPassword,
		Cluster: cluster, ChestName: chestName,
		Healthy: true, SecondsTillDead: 1}

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

	c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName, "State", ONLINE)
	c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName, "Status", ENABLED)

	if host {
		http.HandleFunc("/", c.handleEndpoint)
		go func() { http.ListenAndServe(":"+hostPort, nil) }()
	}

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

//LookUpFile looks up the file in redis to see has the newest copy
func (c *Chest) LookUpFile(path string) {

}

//RegisterFiles registers files in chest to redis
func (c *Chest) RegisterFiles() {
	// Iterate over files in the "/chest" folder
	// - In redis update Cluster:Chests:ChestName:Contents has with date file was updated and name/path.

	err := filepath.Walk("./contents",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == "./contents" {
				return nil
			}

			if c.Client.HGet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path).Val() == "" {
				//Brand new file
				log.Info("File needs added ", path)
				c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path, info.ModTime().UnixNano())
				c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path+"<Local>", info.ModTime().UnixNano())
			} else {
				//Get the local time I registered this file
				localTimeVal := c.Client.HGet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path+"<Local>").Val()
				localTime, err := strconv.ParseInt(localTimeVal, 10, 0)
				if err == nil {
					if localTime < info.ModTime().UnixNano() {
						log.Info("File needs updated! ", path)
						//If local time registered is older than the file then register this mod time to redis
						c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path, info.ModTime().UnixNano())
						c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path+"<Local>", info.ModTime().UnixNano())
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

//SyncFiles syncs files in chest with redis
func (c *Chest) SyncFiles() {
	/*
		Have a map of all files in the cluster and the chest with newest version of the file.
		Put my files in this map.
		Iterate over each chest in redis.
		- For each file update map date and chest if file either doesn't exist or is newer than one in map
		Iterate the map and copy the files from their respective chests to this chest.
		The date of the file remains the same as the origin chest, but the local time changes to the time the file finishes being added.
	*/
	keys := c.Client.Keys(ctx, c.Cluster+":Chests:*:Contents").Val()

	fileMap := make(map[string]*fileInfo)

	for i := range keys {
		key := keys[i]
		keySplit := strings.Split(key, ":")

		chestName := keySplit[2]
		chestHeartbeat, _ := c.Client.HGet(ctx, c.Cluster+":Chests:"+chestName, "Heartbeat").Int64()
		//If the heart isn't beating don't try to sync with it.
		if time.Now().UnixNano()-chestHeartbeat < int64(10*time.Second) {
			files := c.Client.HGetAll(ctx, key).Val()
			for fileName, fileDate := range files {
				if strings.Index(fileName, "<Local>") == -1 {
					t, err := strconv.ParseInt(fileDate, 10, 0)
					if err == nil {
						if fileMap[fileName] != nil {
							// If file is newer than in the map use it instead.
							if t > fileMap[fileName].time {
								fileMap[fileName].time = t
								fileMap[fileName].chestName = chestName
							} else if t == fileMap[fileName].time {
								//We don't need to pull this
								fileMap[fileName].chestName = c.ChestName
							}
						} else {
							fileMap[fileName] = &fileInfo{chestName: chestName, time: t}
						}
					} else {
						log.WithError(err).Error("Error getting local time from redis")
					}
				}
			}
		}

	}

	for path, info := range fileMap {
		if info.chestName != c.ChestName {
			log.Info("Pull the file ", path, " from ", info.chestName)
			c.Client.RPush(ctx, c.Cluster+":Chests:"+info.chestName+":FileQueue", path+":"+c.ChestName)
			tries := 0
			key := c.Cluster + ":Chests:" + c.ChestName + ":FileReturn:" + path
			for {
				val, _ := c.Client.Exists(ctx, key).Uint64()
				if val == 1 {
					break
				}
				if tries > 4 {
					break
				}
				fmt.Println(val, key)
				tries++

				time.Sleep(time.Second * 3)
			}
			if tries > 4 {
				log.Error("Gave up on " + key)
				continue
			}

			log.Info("Got a file from " + info.chestName)
			res, err := c.Client.HGetAll(ctx, c.Cluster+":Chests:"+c.ChestName+":FileReturn:"+path).Result()

			if err != nil {
				log.WithError(err).Error("Error getting file back")
				continue
			}
			file, err := os.Create(path)
			defer file.Close()
			if err != nil {
				log.WithError(err).Error("Error creating file")
				continue
			}
			_, err = file.Write([]byte(res["data"]))
			if err != nil {
				log.WithError(err).Error("Error writing the file")
				continue
			}
			fInfo, _ := file.Stat()
			c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path, res["originTime"])
			c.Client.HSet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path+"<Local>", fInfo.ModTime().UnixNano())

			c.Client.Del(ctx, c.Cluster+":Chests:"+c.ChestName+":FileReturn:"+path)
		}
	}
}

func (c *Chest) HandleFileRequests() {
	for {
		res, err := c.Client.BLPop(ctx, time.Second, c.Cluster+":Chests:"+c.ChestName+":FileQueue").Result()

		if err != nil {
			//log.WithError(err).Error("Error getting file back")
			continue
		}

		log.Info("Got a request to handle", res)
		split := strings.Split(res[1], ":")
		path := split[0]
		destinationChest := split[1]

		originTime, err := c.Client.HGet(ctx, c.Cluster+":Chests:"+c.ChestName+":Contents", path).Result()
		if err != nil {
			log.WithError(err).Error("Error getting origin time")
			continue
		}

		b, err := ioutil.ReadFile(path)
		if err != nil {
			log.WithError(err).Error("Failed to open file")
			continue
		}

		c.Client.HSet(ctx, c.Cluster+":Chests:"+destinationChest+":FileReturn:"+path, "originTime", originTime, "data", string(b))
	}
}

//Shutdown Shutsdown the chest by safely stopping threads
func (c *Chest) Shutdown() {
	c.shuttingDown = true
}

//IsEnabled Returns if the chest is enabled.
func IsEnabled(c *Chest) bool {
	status := c.Client.HGet(ctx, c.Cluster+":Chests:"+c.ChestName, "Status").Val()
	if c.shuttingDown || status == DISABLED {
		return false
	}
	return true
}

func (c *Chest) handleEndpoint(writer http.ResponseWriter, r *http.Request) {
	if c.Healthy {
		http.Error(writer, "Yar, nothing here", http.StatusNotFound)

		/*
			Look up list of healthy chests
				- iterate list and check date of requested file.
					- store date updated and node for determining which one to use later, replacing it if newer than current values
				- If the node that has the newest file isn't me then get the file copied over to me.
				- Respond with file.
		*/
	}
}
