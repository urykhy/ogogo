package main

// prometheus ipsec exporter
// using ip -s xfrm state

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	"github.com/VictoriaMetrics/metrics"
	"github.com/jasonlvhit/gocron"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var (
	logger        = &log.Logger{Out: os.Stderr, Formatter: new(log.JSONFormatter), Level: log.DebugLevel}
	exporterAlive = metrics.NewCounter("ipsec_exporter_alive")
	rePeer        = regexp.MustCompile(`^src (?P<src>.*) dst (?P<dst>.*)$`)
	reTraffic     = regexp.MustCompile(`\s+(?P<bytes>\d*)\(bytes\), (?P<packets>\d*)\(packets\)`)
)

func updateMetrics() {
	defer logger.Debug("update done")
	logger.Debug("update started...")

	cmd := exec.Command("ip", "-s", "xfrm", "state")
	stream, _ := cmd.StdoutPipe()

	err := cmd.Start()
	defer cmd.Wait()
	if err != nil {
		logger.Error("`ip xfrm state` failed with ", err)
		return
	}

	scanner := bufio.NewScanner(stream)
	var currentPeer string
	for scanner.Scan() {
		s := scanner.Text()
		sm := rePeer.FindStringSubmatch(s)
		if len(sm) > 0 {
			logger.Debug("state : found peer ", sm[1], " to ", sm[2])
			currentPeer = fmt.Sprintf(`src="%s", dst="%s"`, sm[1], sm[2])
			continue
		}
		sm = reTraffic.FindStringSubmatch(s)
		if len(sm) > 0 {
			v, _ := strconv.ParseUint(sm[1], 10, 64)
			metrics.GetOrCreateCounter(fmt.Sprintf(`ipsec_bytes{%s}`, currentPeer)).Set(v)
			v, _ = strconv.ParseUint(sm[2], 10, 64)
			metrics.GetOrCreateCounter(fmt.Sprintf(`ipsec_packets{%s}`, currentPeer)).Set(v)
		}
	}

	exporterAlive.Inc()
}

func main() {
	listenAddress := flag.String("web.listen-address", ":9168", "Address to listen on for web interface")
	logLevel := flag.String("log.level", "debug", "Sets the loglevel. Valid levels are debug, info")
	logFormat := flag.String("log.format", "json", "Sets the log format. Valid formats are json and text")
	flag.Parse()

	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("bad log level: %v", err)
	}
	logger.Level = level

	if *logFormat == "text" {
		logger.Formatter = &prefixed.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true}
	}

	logger.Info("ipsec exporter starting on ", *listenAddress)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	gocron.Every(15).Seconds().From(gocron.NextTick()).DoSafely(updateMetrics)
	gocron.Start()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	go func() {
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}()
	<-done

	logger.Info("exited")
}
