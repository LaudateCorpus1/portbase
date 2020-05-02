package rng

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"github.com/aead/serpent"
	"github.com/seehuhn/fortuna"

	"github.com/safing/portbase/modules"
)

var (
	rng      *fortuna.Generator
	rngLock  sync.Mutex
	rngReady = false

	rngCipher = "aes"
	// possible values: aes, serpent

	module *modules.Module
)

func init() {
	module = modules.Register("rng", nil, start, nil)
}

func newCipher(key []byte) (cipher.Block, error) {
	switch rngCipher {
	case "aes":
		return aes.NewCipher(key)
	case "serpent":
		return serpent.NewCipher(key)
	default:
		return nil, fmt.Errorf("unknown or unsupported cipher: %s", rngCipher)
	}
}

func start() error {
	rngLock.Lock()
	defer rngLock.Unlock()

	rng = fortuna.NewGenerator(newCipher)
	if rng == nil {
		return errors.New("failed to initialize rng")
	}

	// explicitly add randomness
	osEntropy := make([]byte, minFeedEntropy/8)
	_, err := rand.Read(osEntropy)
	if err != nil {
		return fmt.Errorf("could not read entropy from os: %s", err)
	}
	rng.Reseed(osEntropy)

	rngReady = true

	// random source: OS
	module.StartServiceWorker("os rng feeder", 0, osFeeder)

	// random source: goroutine ticks
	module.StartServiceWorker("tick rng feeder", 0, tickFeeder)

	// full feeder
	module.StartServiceWorker("full feeder", 0, fullFeeder)

	return nil
}
