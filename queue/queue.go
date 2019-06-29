package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
)

var client *clientv3.Client

const etcdTimeout = time.Second * 5

func stateKey() string {
	return "__internal:" + cfg.Queue
}

func activeKey(task string) string {
	return "__active:" + cfg.Queue + ":" + task
}

func openEtcd() error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cfg.Etcd},
		DialTimeout: etcdTimeout,
	})
	if err != nil {
		return err
	}
	client = cli

	// FIXME: wrap with retry ?
	// create queue state if not exists
	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
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

// KV XXX
type KV struct {
	ID    string
	Value string
}

// State XXX
type State struct {
	State string
}

func dump() (int, *[]KV, error) {

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	resp, err := client.Get(ctx, cfg.Queue, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	cancel()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	prefixLen := len(cfg.Queue) + 1 // to skip `:`
	result := make([]KV, 0, len(resp.Kvs))
	for _, ev := range resp.Kvs {
		t := KV{ID: string(ev.Key)[prefixLen:], Value: string(ev.Value)}
		result = append(result, t)
	}
	return http.StatusOK, &result, nil
}

func getTask(clientID *string, timeout *int64) (int, *KV, error) {
	f := log.Fields{"client": clientID}

	// fetch all running tasks
	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	resp, err := client.Get(ctx, "__active:"+cfg.Queue+":", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	cancel()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	// ensure client have no running task
	prefixLen := len("__active:" + cfg.Queue + ":") // task id right after prefix
	running := make(map[string]struct{})
	for _, ev := range resp.Kvs {
		if string(ev.Value) == *clientID {
			return http.StatusConflict, nil, fmt.Errorf("client already have a task %s", ev)
		}
		runningTaskID := string(ev.Key)[prefixLen:]
		running[runningTaskID] = struct{}{}
	}

	// get pending tasks
	ctx, cancel = context.WithTimeout(context.Background(), etcdTimeout)
	all, err := client.Get(ctx, cfg.Queue+":", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithLimit(cfg.Limit))
	cancel()
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("fail to get tasks")
	}

	prefixLen = len(cfg.Queue) + 1 // to skip `:`
	var pending KV
	for _, ev := range all.Kvs {
		t := KV{ID: string(ev.Key)[prefixLen:], Value: string(ev.Value)}
		_, ok := running[t.ID]
		if !ok && len(pending.ID) == 0 {
			pending = t // pick first not running task
		}
	}

	if len(pending.ID) == 0 {
		return http.StatusNoContent, nil, nil
	}
	f = log.Fields{"client": clientID, "task": pending.ID, "value": pending.Value}

	// create lease
	lease, err := client.Grant(context.TODO(), *timeout)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("fail to create a lease")
	}

	// put with Lease in txn
	ctx, cancel = context.WithTimeout(context.Background(), etcdTimeout)
	putResp, err := client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(activeKey(pending.ID)), "=", 0)).
		Then(clientv3.OpPut(activeKey(pending.ID), *clientID, clientv3.WithLease(lease.ID))).
		Commit()
	cancel()
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("fail to get lease on task %s", pending.ID)
	}
	if !putResp.Succeeded {
		return http.StatusConflict, nil, nil
	}
	logger.WithFields(f).Debug("got a task")
	return http.StatusOK, &pending, nil
}

func renewTask(clientID *string, taskID *string) (int, error) {
	f := log.Fields{"client": *clientID, "task": *taskID}

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	resp, err := client.Get(ctx, activeKey(*taskID))
	cancel()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("fail to get running tasks")
	}

	renewOk := false
	for _, ev := range resp.Kvs {
		if string(ev.Value) != *clientID {
			return http.StatusConflict, fmt.Errorf("client do not own this task")
		}
		_, err = client.KeepAliveOnce(context.Background(), clientv3.LeaseID(ev.Lease))
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("fail to refresh")
		}
		renewOk = true
	}
	if renewOk {
		logger.WithFields(f).Debugf("refresh ok")
		return http.StatusOK, nil
	}
	return http.StatusNotFound, fmt.Errorf("no task to refresh")
}

func ackTask(clientID *string, taskID *string) (int, error) {
	var err error
	f := log.Fields{"client": *clientID, "task": *taskID}

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	var resp *clientv3.TxnResponse
	resp, err = client.Txn(ctx).
		If(clientv3.Compare(clientv3.Value(activeKey(*taskID)), "=", *clientID)).
		Then(clientv3.OpDelete(activeKey(*taskID)),
			clientv3.OpDelete(cfg.Queue+":"+*taskID)).
		Commit()
	cancel()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("fail to ack task %v", *taskID)
	}
	if !resp.Succeeded {
		return http.StatusNotFound, fmt.Errorf("task %v not running", *taskID)
	}

	logger.WithFields(f).Debug("task completed")
	return http.StatusOK, nil
}

func putTask(data *string, old *string, state *string) (int, error) {
	var err error
	dataKey := cfg.Queue + ":" + fmt.Sprintf("%v", time.Now().UnixNano())
	if state == nil {
		logger.Debug("no cas, just add a task")
		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
		_, err = client.Put(ctx, dataKey, *data)
		cancel()
		if err != nil {
			return http.StatusInternalServerError, errors.Wrap(err, "fail to add task")
		}
	} else {
		logger.Debug("with cas, start transaction")
		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
		var resp *clientv3.TxnResponse
		resp, err = client.Txn(ctx).
			If(clientv3.Compare(clientv3.Value(stateKey()), "=", *old)).
			Then(clientv3.OpPut(stateKey(), *state), clientv3.OpPut(dataKey, *data)).
			Commit()
		cancel()
		if err != nil {
			return http.StatusInternalServerError, errors.Wrap(err, "fail to add task")
		}
		if !resp.Succeeded {
			return http.StatusConflict, errors.Wrap(err, "fail to add task")
		}
	}
	return http.StatusOK, nil
}

func getState() (int, *State, error) {
	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	resp, err := client.Get(ctx, stateKey())
	cancel()
	if err != nil {
		return http.StatusInternalServerError, nil, errors.Wrap(err, "fail to get state")
	}

	return http.StatusOK, &State{string(resp.Kvs[0].Value)}, nil
}
