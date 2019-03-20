package main

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/dcos/dcos-ui-update-service/config"
	log "github.com/sirupsen/logrus"
)

type logWriterHook struct {
	Writer    io.Writer
	LogLevels []log.Level
}

func initLogging(config *config.Config) {
	setupSplitLogging()

	// Set logging level
	lvl, err := log.ParseLevel(config.LogLevel())
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(lvl)
	log.Infof("Logging set to: %s", config.LogLevel())
}

func setupSplitLogging() {
	log.SetOutput(ioutil.Discard)

	log.AddHook(&logWriterHook{ // Send logs with level higher than warning to stderr
		Writer: os.Stderr,
		LogLevels: []log.Level{
			log.PanicLevel,
			log.FatalLevel,
			log.ErrorLevel,
			log.WarnLevel,
		},
	})
	log.AddHook(&logWriterHook{ // Send info and debug logs to stdout
		Writer: os.Stdout,
		LogLevels: []log.Level{
			log.InfoLevel,
			log.DebugLevel,
			log.TraceLevel,
		},
	})
}

// Fire will be called when some logging function is called with current hook
// It will format log entry to string and write it to appropriate writer
func (hook *logWriterHook) Fire(entry *log.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = hook.Writer.Write([]byte(line))
	return err
}

// Levels define on which log levels this hook would trigger
func (hook *logWriterHook) Levels() []log.Level {
	return hook.LogLevels
}
