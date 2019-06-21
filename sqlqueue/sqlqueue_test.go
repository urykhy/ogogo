package sqlqueue

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gocraft/dbr"
	"github.com/gocraft/dbr/dialect"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockSession() (*dbr.Session, sqlmock.Sqlmock) {
	db, m, err := sqlmock.New()
	if err != nil {
		panic(err)
	}
	conn := dbr.Connection{DB: db, Dialect: dialect.MySQL, EventReceiver: &dbr.NullEventReceiver{}}
	return conn.NewSession(nil), m
}

func TestMock(t *testing.T) {
	assert := assert.New(t)
	db, mock := mockSession()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM DUAL")).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "foo").AddRow(2, "bar"))
	_, err := db.Query("SELECT id, name FROM DUAL")
	assert.NoError(err)
}

type nilHandler struct {
	state string
	t     *testing.T
}

func (h nilHandler) prepare(ctx context.Context, name string) (*string, error) {
	if h.state == "" {
		return nil, nil
	}
	if h.state == "with_tag" {
		s := "tagname"
		return &s, nil
	}
	if h.state == "fail" {
		return nil, nil
	}
	h.t.Fail()
	return nil, nil
}
func (h nilHandler) process(ctx context.Context, name string, tag string) error {
	if h.state == "" {
		return nil
	}
	if h.state == "with_tag" {
		if tag != "tagname" {
			return errors.New("invalid tag")
		}
		return nil
	}
	if h.state == "fail" {
		return errors.New("expected error")
	}
	h.t.Fail()
	return nil
}

func (h nilHandler) report(err error) {
	log.Println("sqlqueue errror: ", err)
}

func TestSuccess(t *testing.T) {
	db, mock := mockSession()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, tag FROM sqlqueue WHERE (`status` != 'ready') ORDER BY id ASC LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "tag"}).
			AddRow(1, "file1", ""))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'process' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'ready' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))

	log.SetFormatter(&log.JSONFormatter{})
	q := Open(db, time.Millisecond*300, log.StandardLogger(), nilHandler{state: "", t: t})
	time.Sleep(time.Millisecond * 500)
	q.Close()

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTag(t *testing.T) {
	db, mock := mockSession()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, tag FROM sqlqueue WHERE (`status` != 'ready') ORDER BY id ASC LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "tag"}).
			AddRow(1, "file1", ""))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'process', `tag` = 'tagname' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'ready' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))

	log.SetFormatter(&log.JSONFormatter{})
	q := Open(db, time.Millisecond*300, log.StandardLogger(), nilHandler{state: "with_tag", t: t})
	time.Sleep(time.Millisecond * 500)
	q.Close()

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestError(t *testing.T) {
	db, mock := mockSession()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, tag FROM sqlqueue WHERE (`status` != 'ready') ORDER BY id ASC LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "tag"}).
			AddRow(1, "file1", ""))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'process' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `sqlqueue` SET `status` = 'error' WHERE (`id` = 1)")).WillReturnResult(sqlmock.NewResult(0, 1))

	log.SetFormatter(&log.JSONFormatter{})
	q := Open(db, time.Millisecond*300, log.StandardLogger(), nilHandler{state: "fail", t: t})
	time.Sleep(time.Millisecond * 500)
	q.Close()

	require.NoError(t, mock.ExpectationsWereMet())
}

/*
func TestReal(t *testing.T) {
	assert := assert.New(t)
	conn, err := dbr.Open("mysql", "tcp(sql1.mysql.docker:3306)/test", nil)
	assert.NoError(err)
	db := conn.NewSession(nil)

	q := Open(db, time.Millisecond*500, log.StandardLogger(), nilHandler{})
	time.Sleep(time.Second)
	q.Close()
}
*/
