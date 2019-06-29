package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type config struct {
	Queue    string `yaml:"queue"`
	Etcd     string `yaml:"etcd"`
	Addr     string `yaml:"addr"`
	LogLevel string `yaml:"log-level"`
	Limit    int64  `yaml:"client-limit"`
}

func (c *config) getConf(filename string) error {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(f, c)
	if err != nil {
		return err
	}
	return nil
}

func getLogger() *log.Logger {
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalf("bad log level: %v", err)
	}
	logger := &log.Logger{
		Out:       os.Stderr,
		Formatter: new(log.JSONFormatter),
		/*Formatter: &prefixed.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},*/
		Level: level,
	}
	return logger
}

func getConfig() config {
	var c config
	err := c.getConf("queue.yml")
	if err != nil {
		log.Fatalf("can't read configuration: %v", err)
	}
	return c
}

var logger = log.New()
var cfg = getConfig()

func main() {
	logger = getLogger()
	logger.Infof("starting queue %s with %s backend", cfg.Queue, cfg.Etcd)

	err := openEtcd()
	if err != nil {
		logger.Fatalf("cant create connection to etcd: %v", err)
	}

	r := CreateRouter(logger)
	logger.Infof("start api at %v", cfg.Addr)
	server := &http.Server{
		Addr:         cfg.Addr,
		WriteTimeout: time.Second * 5,
		ReadTimeout:  time.Second * 5,
		IdleTimeout:  time.Second * 5,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error(err.Error())
	}
}
