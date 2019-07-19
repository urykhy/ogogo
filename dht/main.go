package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func getLogger() *log.Logger {
	level, err := log.ParseLevel("debug")
	if err != nil {
		log.Fatalf("bad log level: %v", err)
	}
	logger := &log.Logger{
		Out:       os.Stderr,
		Formatter: new(log.JSONFormatter),
		Level:     level,
	}
	return logger
}

var logger = getLogger()

func main() {
	logger.Info("started")

	// create one ring
	h := newDefaultHash()
	el := newDummyRing(h, 10)

	el.set("asd", "123")
	res, err := el.get("asd")
	if err != true {
		logger.Info("fail to get")
	} else {
		logger.Info("result ", *res)
	}
	el.dropNode(0x78629a0f5f3f164f)
	el.Replicate()

	logger.Info("finished")
}
