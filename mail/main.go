package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"path"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"gopkg.in/yaml.v1"
)

type mainConfig struct {
	Name     string `yaml:"name"`
	IMAPAddr string `yaml:"imap"`
	SMTPAddr string `yaml:"smtp"`
	LogLevel string `yaml:"log-level"`
	Store    string `yaml:"store"`
	DB       string `yaml:"db"`
}

type relayConfig struct {
	From     string `yaml:"from"`
	Via      string `yaml:"via"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type domainConfig struct {
	Name     string   `yaml:"name"`
	Aka      []string `yaml:"aka"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

type config struct {
	Main    mainConfig     `yaml:"main"`
	Domains []domainConfig `yaml:"domains"`
	Relay   []relayConfig  `yaml:"relay"`
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
	level, err := log.ParseLevel(cfg.Main.LogLevel)
	if err != nil {
		log.Fatalf("bad log level: %v", err)
	}
	logger := &log.Logger{
		Out: os.Stdout,
		//Formatter: new(log.JSONFormatter),
		Formatter: &prefixed.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},
		Level: level,
	}
	return logger
}
func getConfig() config {
	var c config
	err := c.getConf("mail.yml")
	if err != nil {
		log.Fatalf("can't read configuration: %v", err)
	}
	return c
}

var logger = log.New()
var cfg = getConfig()

func main() {
	logger = getLogger()

	logger.Info("small mail server")
	for _, r := range cfg.Relay {
		logger.Debugf("configured relay from %s via %s as %s", r.From, r.Via, r.Username)
	}
	for _, d := range cfg.Domains {
		os.MkdirAll(path.Join(cfg.Main.Store, d.Name, d.Username), 0755)
		logger.Debugf("configured domain %s for user %s", d.Name, d.Username)
	}

	logger.Infof("starting SMTP server at %s", cfg.Main.SMTPAddr)
	go SMTPRun(cfg.Main.SMTPAddr, cfg.Main.Name)

	logger.Infof("starting IMAP server at %s", cfg.Main.IMAPAddr)
	go IMAPRun(cfg.Main.IMAPAddr)

	logger.Infof("ready")
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan struct{})
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		close(cleanupDone)
	}()
	<-cleanupDone
	logger.Infof("stopping")
}
