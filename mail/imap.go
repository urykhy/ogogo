package main

import (
	"errors"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/server"
	"github.com/luksen/maildir"
	"github.com/sirupsen/logrus"
)

// IMAPMain xxx
type IMAPMain struct{}

// Login XXX
func (x *IMAPMain) Login(state *imap.ConnInfo, login, password string) (backend.User, error) {
	split := strings.Split(login, "@")
	if len(split) != 2 {
		return nil, errors.New("username must be in form user@domain")
	}
	username, domain := split[0], split[1]

	ld := findLocalDomain(domain)
	if ld == nil {
		return nil, errors.New("unknown domain")
	}

	if username != ld.Username || password != ld.Password {
		return nil, errors.New("Invalid username or password")
	}
	logger.Debugf("Login %s from %v", username, state.RemoteAddr)

	return &user{name: username,
		path: path.Join(cfg.Main.Store, ld.Name),
		logger: logger.WithFields(logrus.Fields{
			"remote": state.RemoteAddr.String(),
			"auth":   username})}, nil
}

type user struct {
	name   string
	path   string
	logger *logrus.Entry
}

func (u *user) Username() string {
	return u.name
}
func (u *user) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	u.logger.Debug("list mailboxes")
	files, err := ioutil.ReadDir(u.path)
	if err != nil {
		return nil, err
	}
	boxes := []backend.Mailbox{}
	for _, file := range files {
		// FIXME: check if folder
		u.logger.Debug("found ", file.Name())
		boxes = append(boxes, &mailbox{
			name:   file.Name(),
			md:     maildir.Dir(path.Join(u.path, file.Name())),
			logger: u.logger.WithField("mailbox", file.Name()),
		})
	}
	return boxes, nil
}
func (u *user) GetMailbox(name string) (backend.Mailbox, error) {
	// FIXME: check if folder exists
	u.logger.Debugf("get mailbox %s(%s)", name, path.Join(u.path, name))
	return &mailbox{
		name:   name,
		md:     maildir.Dir(path.Join(u.path, name)),
		logger: u.logger.WithField("mailbox", name),
	}, nil
}
func (u *user) CreateMailbox(name string) error {
	return errors.New("not supported / CreateMailbox")
}
func (u *user) DeleteMailbox(name string) error {
	return errors.New("not supported / DeleteMailbox")
}
func (u *user) RenameMailbox(existingName, newName string) error {
	return errors.New("not supported / RenameMailbox")
}
func (u *user) Logout() error {
	return nil
}

type mailbox struct {
	name   string
	md     maildir.Dir
	logger *logrus.Entry
}

func (m *mailbox) Name() string {
	return m.name
}
func (m *mailbox) Info() (*imap.MailboxInfo, error) {
	m.logger.Debug("Info")
	return &imap.MailboxInfo{Delimiter: "/", Name: m.Name()}, nil
}
func (m *mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	m.logger.Debug("Status")
	status := imap.NewMailboxStatus(m.Name(), make([]imap.StatusItem, 0))

	for _, i := range items {
		m.logger.Debug("run Status for ", i)
		if i == imap.StatusMessages {
			status.Items[i] = struct{}{}
			keys, err := m.md.Keys()
			if err != nil {
				return nil, err
			}
			status.Messages = uint32(len(keys))
			m.logger.Debugf("found %v messages", status.Messages)
		}
		if i == imap.StatusUnseen {
			status.Items[i] = struct{}{}
			count, err := m.md.UnseenCount()
			if err != nil {
				return nil, err
			}
			status.Unseen = uint32(count)
			m.logger.Debugf("found %v unseen messages", status.Unseen)
		}
	}
	return status, nil
}
func (m *mailbox) SetSubscribed(subscribed bool) error {
	m.logger.Debug("SetSubscribed")
	return errors.New("not supported / SetSubscribed")
}
func (m *mailbox) Check() error {
	m.logger.Debug("Check")
	return errors.New("not supported / Check")
}
func (m *mailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	m.logger.Debug("ListMessages")
	return errors.New("not supported / ListMessages")
}
func (m *mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	m.logger.Debug("SearchMessages")
	return nil, errors.New("not supported / SearchMessages")
}
func (m *mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	return errors.New("not supported / CreateMessage")
}
func (m *mailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error {
	return errors.New("not supported / UpdateMessagesFlags")
}
func (m *mailbox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	return errors.New("not supported / CopyMessages")
}
func (m *mailbox) Expunge() error {
	return errors.New("not supported / Expunge")
}

// IMAPRun xxx
func IMAPRun(addr string) {
	s := server.New(&IMAPMain{})
	s.Addr = addr
	s.AllowInsecureAuth = true
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
