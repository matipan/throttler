// Package throttler implements a simple algorithm to do throttling based on CPU usage.
package throttler

import (
	"errors"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/cpu"
)

// ErrAlreadyStarted is the error returned when a user calls
// t.Start twice.
var ErrAlreadyStarted = errors.New("throttler has already been started")

// T is a request throttler that reduces the percentage of allowed events (typically requests)
// according to a target CPU usage.
// Within the throttler we define the following parameters:
// 	* L=Limit CPU Usage
//	* X=CPU Usage
//	* R=% of allowed requests
//	* K=multiplier for the step difference
// 	* S=K*(L-X) -> step to increase/decrease
// 	* T=interval
// 	* ST=step interval
// Every ST we will collect CPU usage information and store it. After T ends we compute the average
// CPU usage (X) and evaluate what action is necessary:
// 	* IF X >= L 	-> reduce R by substracting S, rounding at 0
// 	* IF X < L 	-> increase R by adding S, rounding at 100
// A user of T will simply call `t.Start` so that the throttler starts
// collecting CPU statistics. Every request/event the user will call `t.Allow`
// to ask if the request is allowed to go through or if it needs to be throttled.
//
// T is safe for concurrent use.
type T struct {
	L float64
	R float64
	K float64

	r unsafe.Pointer

	cpuUsage               func() (float64, error)
	rand                   *rand.Rand
	interval, intervalStep time.Duration
	done                   chan struct{}
	mu                     sync.Mutex
	started                bool
}

// New creates a new throttler with the specified parameters.
func New(cpuLimit, k float64, interval, intervalStep time.Duration) *T {
	t := &T{
		L:            cpuLimit,
		K:            k,
		interval:     interval,
		intervalStep: intervalStep,
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		cpuUsage:     getCpuUsage,
	}
	var r float64 = 100.0
	atomic.StorePointer(&t.r, unsafe.Pointer(&r))
	return t
}

func getCpuUsage() (float64, error) {
	cpuStats, err := cpu.Times(false)
	if err != nil || len(cpuStats) != 1 {
		return 0, err
	}

	st := cpuStats[0]
	total := st.Idle + st.Guest + st.GuestNice + st.Iowait + st.Irq + st.Nice + st.Softirq + st.Steal + st.System + st.User
	return 100 - (st.Idle*100.0)/total, nil
}

// Allow returns whether the request is allowed to go through or if it is throttled.
func (t *T) Allow() bool {
	return (t.rand.Float64() * 100.0) < *(*float64)(atomic.LoadPointer(&t.r))
}

// Start starts the control loop that collects CPU information every ST and computes
// the average every T, adjusting R accordingly.
// After a T is stopped it can be re-started by calling Start again.
func (t *T) Start() error {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return ErrAlreadyStarted
	}
	t.started = true
	t.mu.Unlock()

	// we start by allowing all requests to go through
	var r float64 = 100.0
	atomic.StorePointer(&t.r, unsafe.Pointer(&r))

	var (
		itk   = time.NewTicker(t.interval)
		istk  = time.NewTicker(t.intervalStep)
		stats = []float64{}
	)
	defer func() {
		t.mu.Lock()
		t.started = false
		t.mu.Unlock()
	}()
	for {
		select {
		case <-t.done:
			istk.Stop()
			itk.Stop()
			return nil
		case <-itk.C:
			// end of the current interval, now we need to collect
			// the stats, compute the average and make the adjustment if
			// necessary
			if len(stats) == 0 {
				log.Println("could not collect any stats during the interval")
				continue
			}

			var sum, avg float64
			for _, stat := range stats {
				sum += stat
			}
			avg = sum / float64(len(stats))

			r := *(*float64)(atomic.LoadPointer(&t.r))
			step := t.K * (t.L - avg)
			newR := r + step
			switch {
			case avg >= t.L:
				// if the average CPU usage was above or equal to the
				// limit we allow less requests to go in
				if newR < 0 {
					newR = 0
				}
				atomic.StorePointer(&t.r, unsafe.Pointer(&newR))
			case avg < t.L:
				// if the average CPU usage was below the limit
				// then we can allow more requests to go in
				if newR > 100 {
					newR = 100
				}
				atomic.StorePointer(&t.r, unsafe.Pointer(&newR))
			}

			// reset the stats for the next interval
			stats = []float64{}
		case <-istk.C:
			// step within the current interval, get a CPU usage sample and add
			// to the stats
			cpuUsage, err := t.cpuUsage()
			if err != nil {
				log.Printf("could not collect CPU stats: %s", err)
				continue
			}
			stats = append(stats, cpuUsage)
		}
	}
}

// Stop stops the throttler. A user needs to call Start again to resume operations.
func (t *T) Stop() {
	t.done <- struct{}{}
}
