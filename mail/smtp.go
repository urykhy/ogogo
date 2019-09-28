package main

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/sirupsen/logrus"
)

// SMTPMain xxx
type SMTPMain struct{}

// Login handles a login command with username and password.
func (s *SMTPMain) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if username != "username" || password != "password" {
		return nil, errors.New("Invalid username or password")
	}
	logger.Debugf("Login %s from %v", username, state.RemoteAddr)
	return &Session{logger: logger.WithFields(logrus.Fields{
		"remote": state.RemoteAddr.String(),
		"auth":   username})}, nil
}

// AnonymousLogin requires clients to authenticate using SMTP AUTH before sending emails
func (s *SMTPMain) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	logger.Debugf("Anonymous login from %v", state.RemoteAddr)
	return &Session{logger: logger.WithFields(logrus.Fields{
		"remote": state.RemoteAddr.String(),
		"auth":   "anonymous"}),
		remote: state.RemoteAddr.String()}, nil
	//return nil, smtp.ErrAuthRequired
}

// Meta xxx
type Meta struct {
	from string
	to   string
}

// A Session is returned after successful login.
type Session struct {
	Message Meta
	logger  *logrus.Entry
	remote  string
}

// Mail xxx
func (s *Session) Mail(from string) error {
	s.logger.Debugf("Mail from: %s", from)
	s.Message.from = from
	return nil
}

// Rcpt xxx
func (s *Session) Rcpt(to string) error {
	s.logger.Debugf("Rcpt to: %s", to)
	s.Message.to = to
	return nil
}

// Data xxx
func (s *Session) Data(r io.Reader) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	s.logger.Debugf("Data: %s", string(b))

	err = delivery(s.Message, b, s.remote)
	if err != nil {
		s.logger.Errorf("Delivery failed: %s", err)
		return err
	}
	s.logger.Debugf("Delivered")
	return nil
}

// Reset xxx
func (s *Session) Reset() {
	s.logger.Debugf("Reset")
	s.Message = Meta{}
}

// Logout xxx
func (s *Session) Logout() error {
	s.logger.Debugf("Logout")
	return nil
}

// SMTPRun xxx
func SMTPRun(addr, name string) {
	be := &SMTPMain{}

	s := smtp.NewServer(be)

	s.Addr = addr
	s.Domain = name
	s.ReadTimeout = 600 * time.Second
	s.WriteTimeout = 600 * time.Second
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 1
	s.AllowInsecureAuth = true

	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
