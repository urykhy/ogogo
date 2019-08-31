package main

import (
	"fmt"
	"net/mail"

	"path"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/luksen/maildir"
)

func delivery(msg Meta, data []byte) error {
	e, err := mail.ParseAddress(msg.to)
	if err != nil {
		return fmt.Errorf("bad rcpt: %s", msg.to)
	}
	split := strings.Split(e.Address, "@")
	if len(split) != 2 {
		return fmt.Errorf("bad rcpt: %s", msg.to)
	}
	rcptName, rcptDomain := split[0], split[1]

	if ld := findLocalDomain(rcptDomain); ld != nil {
		return localDelivery(ld, rcptName, data)
	}
	if sd := findSmartHost(msg.from); sd != nil {
		return smartDelivery(sd, msg.to, data)
	}
	return fmt.Errorf("no route: from %s to %s", msg.from, msg.to)
}

func findLocalDomain(domain string) *domainConfig {
	for _, d := range cfg.Domains {
		if domain == d.Name {
			return &d
		}
		for _, sd := range d.Aka {
			if domain == sd {
				return &d
			}
		}
	}
	return nil
}

func findSmartHost(from string) *relayConfig {
	for _, r := range cfg.Relay {
		if from == r.From {
			return &r
		}
	}
	return nil
}

func localDelivery(ld *domainConfig, localpart string, data []byte) error {
	// if localpart not as username - use it as folder
	d := maildir.Dir(path.Join(cfg.Main.Store, ld.Name, localpart))

	err := d.Create()
	if err != nil {
		return err
	}
	dv, err := d.NewDelivery()
	if err != nil {
		return err
	}
	_, err = dv.Write(data)
	if err != nil {
		dv.Abort()
		return err
	}
	dv.Close()

	return nil
}

func smartDelivery(sd *relayConfig, rcpt string, data []byte) error {
	auth := sasl.NewPlainClient("", sd.Username, sd.Password)
	return smtp.SendMail(sd.Via, auth, sd.From, []string{rcpt}, strings.NewReader(string(data)))
	//return errors.New("not supported")
}
