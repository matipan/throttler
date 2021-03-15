package throttler

import (
	"fmt"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestT_StartNoThrottle(t *testing.T) {
	is := is.New(t)

	th := New(10, 2, 2*time.Millisecond, 250*time.Microsecond)
	th.cpuUsage = func() (float64, error) {
		return 0, nil
	}

	go th.Start()
	is.True(th.Allow())
	time.Sleep(2 * time.Millisecond)
	is.True(th.Allow())
	time.Sleep(2 * time.Millisecond)
	is.True(th.Allow())
}

func TestT_LinearThrottle(t *testing.T) {
	is := is.New(t)

	th := New(10, 2, 2*time.Millisecond, 250*time.Microsecond)
	th.cpuUsage = func() (float64, error) {
		return 20, nil
	}

	go th.Start()
	// first one should be allowe since we start with 100%
	is.True(th.Allow())
	time.Sleep(3 * time.Millisecond)
	fmt.Println(th.Allow())
	is.True(!th.Allow())
	time.Sleep(2 * time.Millisecond)
	is.True(!th.Allow())
}
