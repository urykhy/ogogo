package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
)

var client *clientv3.Client

const timeout = time.Second * 5

func stateKey() string {
	return "__internal:" + cfg.Queue
}

func activeKey(task string) string {
	return "__active:" + cfg.Queue + ":" + task
}

func openEtcd() error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cfg.Etcd},
		DialTimeout: timeout,
	})
	if err != nil {
		return err
	}
	client = cli

	// FIXME: wrap with retry ?
	// create queue state if not exists
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(stateKey()), "=", 0)).
		Then(clientv3.OpPut(stateKey(), "")).
		Commit()
	cancel()
	if err != nil {
		return err
	}

	return nil
}

type kv struct {
	ID    string
	Value string
}

func handleDump(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := client.Get(ctx, cfg.Queue, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail dump queue")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	prefixLen := len(cfg.Queue) + 1 // to skip `:`
	result := make([]kv, 0, len(resp.Kvs))
	for _, ev := range resp.Kvs {
		t := kv{ID: string(ev.Key)[prefixLen:], Value: string(ev.Value)}
		result = append(result, t)
	}

	jsonString, err := json.Marshal(result)
	if err != nil {
		logger.WithError(err).Warnf("fail to format dump")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonString)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client")
	leaseTime, err := strconv.ParseInt(q.Get("timeout"), 10, 64)
	if err != nil {
		logger.WithError(err).Warnf("bad timeout")
		w.WriteHeader(http.StatusBadRequest)
	}
	f := log.Fields{"client": clientID}

	// fetch all running tasks
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := client.Get(ctx, "__active:"+cfg.Queue+":", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to get running tasks")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// ensure client have no running task
	prefixLen := len("__active:" + cfg.Queue + ":") // task id right after prefix
	running := make(map[string]struct{})
	for _, ev := range resp.Kvs {
		if string(ev.Value) == clientID {
			logger.WithFields(f).Warnf("client already have a task %s", ev)
			w.WriteHeader(http.StatusConflict)
			return
		}
		runningTaskID := string(ev.Key)[prefixLen:]
		running[runningTaskID] = struct{}{}
	}

	// get pending tasks
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	all, err := client.Get(ctx, cfg.Queue+":", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithLimit(cfg.Limit))
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to get tasks")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	prefixLen = len(cfg.Queue) + 1 // to skip `:`
	var pending kv
	for _, ev := range all.Kvs {
		t := kv{ID: string(ev.Key)[prefixLen:], Value: string(ev.Value)}
		_, ok := running[t.ID]
		if !ok && len(pending.ID) == 0 {
			pending = t // pick first not running task
		}
	}

	if len(pending.ID) == 0 {
		logger.WithFields(f).Debug("no tasks available")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	f = log.Fields{"client": clientID, "task": pending.ID, "value": pending.Value}

	// create lease
	lease, err := client.Grant(context.TODO(), leaseTime)
	if err != nil {
		logger.WithError(err).Warnf("fail to create a lease")
		w.WriteHeader(http.StatusInternalServerError)
	}

	// put with Lease in txn
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	putResp, err := client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(activeKey(pending.ID)), "=", 0)).
		Then(clientv3.OpPut(activeKey(pending.ID), clientID, clientv3.WithLease(lease.ID))).
		Commit()
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to get lease on task %s", pending.ID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !putResp.Succeeded {
		logger.WithFields(f).Debug("conflict")
		w.WriteHeader(http.StatusConflict)
		return
	}

	// return task id and data to user
	jsonString, err := json.Marshal(pending)
	if err != nil {
		logger.WithError(err).Warnf("fail to format reply")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.WithFields(f).Debug("got a task")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonString)
}

func handleRenew(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client")
	taskID := q.Get("task")
	f := log.Fields{"client": clientID, "task": taskID}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := client.Get(ctx, activeKey(taskID))
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to get running tasks")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	renewOk := false
	for _, ev := range resp.Kvs {
		if string(ev.Value) != clientID {
			logger.WithFields(f).Warnf("client do not own this task")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_, err = client.KeepAliveOnce(context.Background(), clientv3.LeaseID(ev.Lease))
		if err != nil {
			logger.WithFields(f).Warnf("fail to refresh")
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			renewOk = true
		}
	}
	if renewOk {
		logger.WithFields(f).Debugf("refresh ok")
		w.WriteHeader(http.StatusOK)
	} else {
		logger.WithFields(f).Warn("no task to refresh")
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleAck(w http.ResponseWriter, r *http.Request) {
	var err error
	q := r.URL.Query()
	clientID := q.Get("client")
	taskID := q.Get("task")
	f := log.Fields{"client": clientID, "task": taskID}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	var resp *clientv3.TxnResponse
	resp, err = client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(activeKey(taskID)), "=", clientID)).
		Then(clientv3.OpDelete(activeKey(taskID)),
			clientv3.OpDelete(cfg.Queue+":"+taskID)).
		Commit()
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to ack task %s", taskID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !resp.Succeeded {
		logger.WithFields(f).Debug("task not running")
		w.WriteHeader(http.StatusNotFound) // FIXME ???
		return
	}

	logger.WithFields(f).Debug("task complited")
	w.WriteHeader(http.StatusOK)
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	taskValue := q.Get("task")
	old := q.Get("old")
	state := q.Get("state")

	if len(taskValue) < 1 {
		logger.Warn("bad task in request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var err error
	dataKey := cfg.Queue + ":" + fmt.Sprintf("%v", time.Now().UnixNano())
	if len(state) < 1 {
		logger.Debug("no cas, just add a task")
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		_, err = client.Put(ctx, dataKey, taskValue)
		cancel()
		if err != nil {
			logger.WithError(err).Warnf("fail to add task %s", taskValue)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		logger.Debug("with cas, start transaction")
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		var resp *clientv3.TxnResponse
		resp, err = client.Txn(ctx).
			If(clientv3.Compare(clientv3.Value(stateKey()), "=", old)).
			Then(clientv3.OpPut(stateKey(), state), clientv3.OpPut(dataKey, taskValue)).
			Commit()
		cancel()
		if err != nil {
			logger.WithError(err).Warnf("fail to add task %s", taskValue)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !resp.Succeeded {
			logger.WithField("task-value", taskValue).Warn("conflict")
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func handleState(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := client.Get(ctx, stateKey())
	cancel()
	if err != nil {
		logger.WithError(err).Warnf("fail to get state")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jsonString, err := json.Marshal(struct{ State string }{string(resp.Kvs[0].Value)})
	if err != nil {
		logger.WithError(err).Warnf("fail to format state")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonString)
}
