package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/khoi/kuhhandel/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	address := flag.String("addr", ":8080", "listen address")
	databasePath := flag.String("db", "kuhhandel.db", "SQLite database path")
	flag.Parse()

	application, err := server.New(*databasePath)
	if err != nil {
		return err
	}
	defer application.Close()

	httpServer := &http.Server{
		Addr:              *address,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
		close(errorsChannel)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errorsChannel:
		return err
	case <-signals:
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownContext)
	}
}
