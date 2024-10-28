package main

import (
	"github.com/gurkengewuerz/sshcontainer/internal/server"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()

	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	log.SetLevel(logrus.DebugLevel)

	config, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.SetLevel(logrus.Level(config.LogLevel))

	srv, err := server.New(config, log)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.WithField("port", config.SSHPort).Info("Starting SSH server")
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
