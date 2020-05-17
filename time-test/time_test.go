package bench

// based on https://groups.google.com/forum/#!topic/golang-dev/xI83AG-QUbg
// go test -bench=. .

import (
 "syscall"
 "testing"
 "time"
 "sync"
)

const GOROUTINES=1000

func now() time.Time {
    var tv syscall.Timeval
    syscall.Gettimeofday(&tv)
    return time.Unix(0, syscall.TimevalToNsec(tv))
}

func BenchmarkTimeNow(b *testing.B) {
    b.StopTimer()
    var wg sync.WaitGroup
    for i := 0; i < GOROUTINES; i++ {
        wg.Add(1)
        go func() {
            b.StartTimer()
            for i := 0; i < b.N; i++ {
                time.Now()
            }
            b.StopTimer()
            wg.Done()
        }()
    }
    wg.Wait()
}

func BenchmarkNowGettimeofday(b *testing.B) {
    b.StopTimer()
    var wg sync.WaitGroup
    for i := 0; i < GOROUTINES; i++ {
        wg.Add(1)
        go func() {
            b.StartTimer()
            for i := 0; i < b.N; i++ {
                now()
            }
            b.StopTimer()
            wg.Done()
        }()
    }
    wg.Wait()
}
