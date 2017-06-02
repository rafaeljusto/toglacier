// +build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func manageSignals(cancel context.CancelFunc, cancelFunc func()) {
	// create a graceful shutdown when receiving a signal (SIGINT, SIGKILL,
	// SIGTERM, SIGSTOP)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGSTOP)

	go func() {
		<-sigs
		if cancelFunc != nil {
			cancelFunc()
		}

		cancel()
	}()
}
