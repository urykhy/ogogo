package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOk(t *testing.T) {
	assert := assert.New(t)

	data := make([]Task, 0)
	data = append(data, uint64(1))
	data = append(data, uint64(2))
	data = append(data, uint64(3))

	err := Process(context.Background(), data, func(task Task) error {
		u64, ok := task.(uint64)
		if ok {
			t.Log("process", u64)
			return nil
		}
		return fmt.Errorf("bad data: %v, %T", task, task)
	}, 1)
	assert.NoError(err)
}

func TestErr(t *testing.T) {
	assert := assert.New(t)

	data := make([]Task, 0)
	data = append(data, "foo")
	err := Process(context.Background(), data, func(task Task) error {
		_, ok := task.(uint64)
		if ok {
			return nil
		}
		return fmt.Errorf("unexpected %T data: %v", task, task)
	}, 1)
	assert.Error(err)
	assert.Equal("unexpected string data: foo", fmt.Sprintf("%s", err))
}

func TestCancel(t *testing.T) {
	assert := assert.New(t)

	data := make([]Task, 0)
	for i := 0; i < 10; i++ {
		data = append(data, i)
	}
	ctx, done := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer done()
	err := Process(ctx, data, func(task Task) error {
		time.Sleep(30 * time.Millisecond)
		return nil
	}, 2)
	assert.Error(err)
}
