package log

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"log/slog"

	screenLog "log"
)

type Logger struct {
	logToScreen       bool
	log               *slog.Logger
	currentLoggerFile string
	mtx               sync.RWMutex
}

var logger *Logger = &Logger{}

func getTodayDate() string {
	now := time.Now()
	return fmt.Sprintf("%d%s%d", now.Day(), now.Month().String(), now.Year())
}

func getIANAName() (string, error) {
	linkPath := "/etc/localtime"
	targetPath, err := os.Readlink(linkPath)
	if err != nil {
		return "", err
	}

	tzParts := strings.Split(targetPath, "/")
	if len(tzParts) < 3 {
		return "", errors.New("invalid timezone format")
	}

	continent, country := tzParts[len(tzParts)-2], tzParts[len(tzParts)-1]
	timezone := fmt.Sprintf("%s/%s", continent, country)

	// Load the location using the timezone value
	// Ensure valid IANA name
	_, err = time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}

	return timezone, nil
}

func runFuncAtTime(tm string, freq time.Duration, fn func()) error {
	timezone, _ := getIANAName()
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}

	tm_, _ := time.ParseInLocation("00:00:00", tm, loc)
	stm := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), tm_.Hour(), tm_.Minute(), tm_.Second(), 0, tm_.Location())

	duration := stm.Sub(time.Now().In(loc))
	var timer *time.Timer

	timer = time.AfterFunc(duration, func() {
		timer.Reset(freq)
		fn()
	})

	select {}
}

// foolish
func checkAndUpdateLogger() {
	todayLogName := fmt.Sprintf("log/%s.log", getTodayDate())
	if todayLogName != logger.currentLoggerFile {
		err := initLogger(todayLogName)
		if err != nil {
			LogError("failed to update logger", err)
			return
		}
	}
}

func NewLogger() error {
	todayLogName := fmt.Sprintf("log/%s.log", getTodayDate())
	err := initLogger(todayLogName)
	if err != nil {
		return err
	}

	go func() {
		err = runFuncAtTime("00:00:00", time.Duration(time.Hour*24), func() {
			// opportunity for a data race
			checkAndUpdateLogger()
		})

		if err != nil {
			LogError("error stating log updater goroutine", err)
		}
	}()

	return nil
}

func initLogger(filename string) error {
	if os.Getenv("TUNWG_RUN_SERVER") == "true" {
		logger.mtx.Lock()
		defer logger.mtx.Unlock()

		_ = os.Mkdir("log", 0755)

		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		_logger := slog.New(slog.NewTextHandler(f, nil))
		if _logger == nil {
			fmt.Println("could not create new logger")
			return errors.New("could not create new logger")
		}

		logger.currentLoggerFile = filename
		logger.log = _logger
	} else {
		logger.logToScreen = true
	}

	return nil
}

func LogInfo(msg string) {
	if logger.logToScreen {
		screenLog.Printf("%s\n", msg)
	} else {
		logger.log.Info(msg)
	}
}

func LogWarn(msg string) {
	if logger.logToScreen {
		screenLog.Printf("%s\n", msg)
	} else {
		logger.log.Warn(msg)
	}
}

func LogError(msg string, err error) {
	msgStr := fmt.Sprintf("%s: %v", msg, err)

	if logger.logToScreen {
		screenLog.Printf("%s\n", msgStr)
	} else {
		logger.log.Error(msgStr)
	}

}

func LogFatal(msg string) {
	LogWarn(msg)
	os.Exit(1)
}
