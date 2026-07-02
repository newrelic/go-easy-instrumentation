package main

import "github.com/sirupsen/logrus"

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.Info("Starting application")
	logger.WithField("key", "value").Warn("a warning")
}
