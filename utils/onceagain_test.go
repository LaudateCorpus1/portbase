package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tevino/abool"
)

func TestOnceAgain(t *testing.T) {
	t.Parallel()

	oa := OnceAgain{}
	executed := abool.New()
	var testWg sync.WaitGroup

	// basic
	for i := 0; i < 10; i++ {
		testWg.Add(100)
		for i := 0; i < 100; i++ {
			go func() {
				oa.Do(func() {
					if !executed.SetToIf(false, true) {
						t.Errorf("concurrent execution!")
					}
					time.Sleep(10 * time.Millisecond)
				})
				testWg.Done()
			}()
		}
		testWg.Wait()
		executed.UnSet() // reset check
	}

	// streaming
	var execs uint32
	testWg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			oa.Do(func() {
				atomic.AddUint32(&execs, 1)
				time.Sleep(10 * time.Millisecond)
			})
			testWg.Done()
		}()

		time.Sleep(1 * time.Millisecond)
	}

	testWg.Wait()
	if execs >= 20 {
		t.Errorf("unexpected high exec count: %d", execs)
	}
}
