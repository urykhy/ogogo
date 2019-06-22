package worker

import (
	"context"
	"sync"
)

// Task XXX
type Task interface{}

// Handler XXX
type Handler = func(task Task) error

// Process XXX
func Process(parentCtx context.Context, data []Task, handler Handler, threadCount int) error {
	tasks := make(chan Task, threadCount)
	defer close(tasks)
	results := make(chan error, len(data))
	defer close(results)
	ctx, done := context.WithCancel(parentCtx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer done()

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-tasks:
					if !ok {
						return
					}
					err := handler(msg)
					if err != nil {
						done()
					}
					results <- err
				}
			}
		}()
	}

	for _, d := range data {
		select {
		case <-ctx.Done():
			break
		case tasks <- d:
		}
	}

	for i := 0; i < len(data); i++ {
		select {
		case <-parentCtx.Done():
			return parentCtx.Err()
		case err := <-results:
			if err != nil {
				return err
			}
		}
	}

	return nil
}
