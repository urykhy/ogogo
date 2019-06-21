package sqlqueue

import (
	"context"
	"sync"
	"time"

	"github.com/gocraft/dbr"
	log "github.com/sirupsen/logrus"
)

const tableName = "sqlqueue"

// SQLHandler is a task processor interface
type SQLHandler interface {
	// must allocate resource and return resource id
	prepare(ctx context.Context, name string) (*string, error)

	// called to process task, on retry - resource if will be reused
	process(ctx context.Context, name string, tag string) error

	// called on mysql error
	report(err error)
}

// SQLQueue XXX
type SQLQueue struct {
	db       *dbr.Session
	handler  SQLHandler
	interval time.Duration
	ctx      context.Context
	done     context.CancelFunc
	logger   *log.Logger
	wg       sync.WaitGroup
}

// Open creates new sql queue
func Open(db *dbr.Session, interval time.Duration, logger *log.Logger, handler SQLHandler) *SQLQueue {
	ctx, done := context.WithCancel(context.Background())
	s := &SQLQueue{db: db,
		handler:  handler,
		interval: interval,
		ctx:      ctx,
		done:     done,
		logger:   logger}
	s.wg.Add(1)
	go s.loop()
	return s
}

// Close terminates sql queue
func (s *SQLQueue) Close() {
	s.done()
	s.wg.Wait()
}

func (s *SQLQueue) loop() {
	s.logger.Info("started")

main:
	for {
		select {
		case _ = <-s.ctx.Done():
			break main
		case <-time.After(s.interval):
			s.once()
		}
	}

	s.logger.Info("stopped")
	s.wg.Done()
}

type queueEntry struct {
	ID   uint32 `db:"id"`
	Name string `db:"name"`
	Tag  string `db:"tag"`
}

func (s *SQLQueue) once() {
	var q queueEntry
	count, err := s.db.Select("id", "name", "tag").From(tableName).Where(dbr.Neq("status", "ready")).OrderAsc("id").Limit(1).Load(&q)
	if err != nil {
		s.logger.Warn("fail to get new task: ", err)
		s.handler.report(err)
		return
	}

	if count == 0 {
		s.logger.Debug("no tasks to process")
		return
	}

	logger := s.logger.WithField("task", q.ID)

	ctx, done := context.WithCancel(s.ctx)
	defer done()

	var tag *string
	// if no tag - ask for one
	if len(q.Tag) == 0 {
		tag, err = s.handler.prepare(ctx, q.Name)
		if err != nil {
			logger.Warn("fail to prepare: ", err)
			return
		}
	}

	query := s.db.Update(tableName).Set("status", "process")
	if tag != nil {
		logger.Info("set tag ", *tag)
		query = query.Set("tag", *tag)
		q.Tag = *tag
	}

	_, err = query.Where(dbr.Eq("id", q.ID)).Exec()
	if err != nil {
		logger.Warn("fail to update status : ", err)
		s.handler.report(err)
		return
	}

	logger.Info("started")
	err = s.handler.process(ctx, q.Name, q.Tag)
	if err != nil {
		logger.Warn("fail to process: ", err)

		_, err = s.db.Update(tableName).Set("status", "error").Where(dbr.Eq("id", q.ID)).Exec()
		if err != nil {
			logger.Warn("fail to update status: ", err)
			s.handler.report(err)
		}

		return
	}

	_, err = s.db.Update(tableName).Set("status", "ready").Where(dbr.Eq("id", q.ID)).Exec()
	if err != nil {
		logger.Error("fail to mark as done: ", err)
		s.handler.report(err)
	} else {
		logger.Info("done")
	}
}
