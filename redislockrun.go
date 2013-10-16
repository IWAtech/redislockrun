package main

import (
	"fmt"
	"github.com/hoisie/redis"
	"github.com/golang/glog"
	"os"
	"os/exec"
	"strconv"
	"time"
	"errors"
	"flag"
)

var redisClient redis.Client
var config *Config

type Config struct {
	Key           string
	RedisDb       int
	RedisAddr     string
	RedisPassword string
	LockTimeout   time.Duration
}

func (conf *Config) ParseFromEnvironment() {
	conf.RedisAddr = os.Getenv("REDISLOCKRUN_ADDR")
	conf.RedisPassword = os.Getenv("REDISLOCKRUN_PASSWORD")

	if db, err := strconv.ParseInt(os.Getenv("REDISLOCKRUN_DB"), 10, 8); err != nil {
		conf.RedisDb = int(db)
	}
	
	if key := os.Getenv("REDISLOCKRUN_KEY"); len(key) > 0 {
		conf.Key = key
	}
}

// Run the command and catch any panics
func safelyRun(name string, args []string) {
	defer func() {
		if err := recover(); err != nil {
			glog.Errorln(err)
			unlock()
		}
	}()

	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		panic(fmt.Sprintf("Error running %s %v: %s", name, args, err))
	}
}

func makeExpiryTime() time.Time {
	expire := time.Now()
	expire = expire.Add(config.LockTimeout)
	expire = expire.Add(1*time.Second)

	return expire
}

func unlock() {
	expire, err := getLockExpire()
	now := time.Now()

	// Only unlock if the lock is not expired
	if err == nil && now.Before(expire) {
		glog.Infoln("Deleting lock")
		redisClient.Del(config.Key)
	}
}

func unlockAndLock(expire time.Time) error {
	resp, err := redisClient.Getset(config.Key, []byte(strconv.FormatInt(expire.Unix(), 10)))

	if err != nil {
		return err
	}

	unixTimeStamp, _ := strconv.ParseInt(string(resp), 10, 32)
	timestamp := time.Unix(unixTimeStamp, 0)
	now := time.Now()

	if glog.V(2) {
		glog.Infoln("My lock expire is", expire.Unix())
		glog.Infoln("Now", now.Unix())
		glog.Infoln("Current lock expire in Redis", unixTimeStamp)
	}

	// If the timestamp is not expired, then another process aquired the lock faster
	if now.Before(timestamp) {
		return errors.New("Locked")
	}

	return nil
}

func getLockExpire() (time.Time, error) {
	resp, err := redisClient.Get(config.Key)

	if err == nil {
		val, _ := strconv.ParseInt(string(resp), 10, 32)
		return time.Unix(val, 0), nil
	} else {
		goto error
	}

error:
	return time.Now(), err
}

func init() {
	config = &Config{Key: "lock"}
	config.ParseFromEnvironment()

	redisClient = redis.Client{Addr: config.RedisAddr, Db: config.RedisDb}
	redisClient.Auth(config.RedisPassword)

	flag.DurationVar(&config.LockTimeout, "timeout", 30*time.Minute, "Lock timeout")
	flag.Parse()
}

func main() {
	var cmdArgs = flag.Args()
	var expire = makeExpiryTime()

	glog.V(2).Infof("Config: %+v\n", config)

	if ok, _ := redisClient.Setnx(config.Key, []byte(strconv.FormatInt(expire.Unix(), 10))); !ok {
		if timeout, err := getLockExpire(); err == nil {
			if time.Now().After(timeout) {
				glog.Infoln("Lock is expired. Trying to aquire lock")
				// Lock expired, try to aquire a new one
				if err := unlockAndLock(expire); err == nil {
					glog.Infoln("Aquired lock")
				} else {
					// Failed aquiring the lock, exit
					glog.Infoln("Failed: lock is already aquired by other process")
					os.Exit(1)
				}
			} else {
				// Lock not expired
				glog.Infoln("Locked")
				os.Exit(1)
			}
		}
	}

	defer unlock()
	defer glog.Flush()

	glog.Infof("Running %s %v\n", cmdArgs[0], cmdArgs[1:])

	safelyRun(cmdArgs[0], cmdArgs[1:])

	glog.Infof("Finished running %s %v", cmdArgs[0], cmdArgs[1:])
}
