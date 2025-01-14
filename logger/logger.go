// Copyright 2019 ThoughtWorks, Inc.

// This file is part of Gauge.

// Gauge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Gauge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with Gauge.  If not, see <http://www.gnu.org/licenses/>.

package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/getgauge/common"
	"github.com/getgauge/gauge/config"
	"github.com/getgauge/gauge/plugin/pluginInfo"
	"github.com/getgauge/gauge/version"
	"github.com/natefinch/lumberjack"
	"github.com/op/go-logging"
)

const (
	gaugeModuleID    = "Gauge"
	logsDirectory    = "logs_directory"
	logs             = "logs"
	gaugeLogFileName = "gauge.log"
	apiLogFileName   = "api.log"
	lspLogFileName   = "lsp.log"
	// CLI indicates gauge is used as a CLI.
	CLI = iota
	// API indicates gauge is in daemon mode. Used in IDEs.
	API
	// LSP indicates that gauge is acting as an LSP server.
	LSP
)

var level logging.Level
var initialized bool
var loggersMap map[string]*logging.Logger
var fatalErrors []string
var fileLogFormat = logging.MustStringFormatter("%{time:02-01-2006 15:04:05.000} [%{module}] [%{level}] %{message}")

// ActiveLogFile log file represents the file which will be used for the backend logging
var ActiveLogFile string
var machineReadable bool
var isLSP bool

// Initialize logger with given level
func Initialize(mr bool, logLevel string, c int) {
	machineReadable = mr
	level = loggingLevel(logLevel)
	switch c {
	case CLI:
		ActiveLogFile = getLogFile(gaugeLogFileName)
	case API:
		ActiveLogFile = getLogFile(apiLogFileName)
	case LSP:
		isLSP = true
		ActiveLogFile = getLogFile(lspLogFileName)
	}
	addLogger(gaugeModuleID)
	initialized = true
}

// GetLogger gets logger for given modules. It creates a new logger for the module if not exists
func GetLogger(module string) *logging.Logger {
	if module == "" {
		return loggersMap[gaugeModuleID]
	}
	if _, ok := loggersMap[module]; !ok {
		addLogger(module)
	}
	return loggersMap[module]

}

func logInfo(logger *logging.Logger, stdout bool, msg string) {
	if level >= logging.INFO {
		write(stdout, msg)
	}
	if !initialized {
		return
	}
	logger.Infof(msg)
}

func logError(logger *logging.Logger, stdout bool, msg string) {
	if level >= logging.ERROR {
		write(stdout, msg)
	}
	if !initialized {
		fmt.Fprintf(os.Stderr, msg)
		return
	}
	logger.Errorf(msg)
}

func logWarning(logger *logging.Logger, stdout bool, msg string) {
	if level >= logging.WARNING {
		write(stdout, msg)
	}
	if !initialized {
		return
	}
	logger.Warningf(msg)
}

func logDebug(logger *logging.Logger, stdout bool, msg string) {
	if level >= logging.DEBUG {
		write(stdout, msg)
	}
	if !initialized {
		return
	}
	logger.Debugf(msg)
}

func logCritical(logger *logging.Logger, msg string) {
	if !initialized {
		fmt.Fprintf(os.Stderr, msg)
		return
	}
	logger.Criticalf(msg)

}

func write(stdout bool, msg string) {
	if !isLSP && stdout {
		if machineReadable {
			machineReadableLog(msg)
		} else {
			fmt.Println(msg)
		}
	}
}

// OutMessage contains information for output log
type OutMessage struct {
	MessageType string `json:"type"`
	Message     string `json:"message"`
}

// ToJSON converts OutMessage into JSON
func (out *OutMessage) ToJSON() (string, error) {
	json, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(json), nil
}

func machineReadableLog(msg string) {
	strs := strings.Split(msg, "\n")
	for _, m := range strs {
		outMessage := &OutMessage{MessageType: "out", Message: m}
		m, _ = outMessage.ToJSON()
		fmt.Println(m)
	}
}

func addLogger(module string) {
	l := logging.MustGetLogger(module)
	if loggersMap == nil {
		loggersMap = make(map[string]*logging.Logger)
	}
	loggersMap[module] = l
	initFileLogger(ActiveLogFile, module, l)
}

func initFileLogger(logFileName string, module string, fileLogger *logging.Logger) {
	var backend logging.Backend
	backend = createFileLogger(logFileName, 10)
	fileFormatter := logging.NewBackendFormatter(backend, fileLogFormat)
	fileLoggerLeveled := logging.AddModuleLevel(fileFormatter)
	fileLoggerLeveled.SetLevel(logging.DEBUG, "")
	fileLogger.SetBackend(fileLoggerLeveled)
}

func createFileLogger(name string, size int) logging.Backend {
	return logging.NewLogBackend(&lumberjack.Logger{
		Filename:   name,
		MaxSize:    size, // megabytes
		MaxBackups: 3,
		MaxAge:     28, //days
	}, "", 0)
}

func addLogsDirPath(logFileName string) string {
	customLogsDir := os.Getenv(logsDirectory)
	if customLogsDir == "" {
		return filepath.Join(logs, logFileName)
	}
	return filepath.Join(customLogsDir, logFileName)
}

func getLogFile(logFileName string) string {
	logDirPath := addLogsDirPath(logFileName)
	if filepath.IsAbs(logDirPath) {
		return logDirPath
	}
	if config.ProjectRoot != "" {
		return filepath.Join(config.ProjectRoot, logDirPath)
	}
	gaugeHome, err := common.GetGaugeHomeDirectory()
	if err != nil {
		return logDirPath
	}
	return filepath.Join(gaugeHome, logDirPath)
}

func loggingLevel(logLevel string) logging.Level {
	if logLevel != "" {
		switch strings.ToLower(logLevel) {
		case "debug":
			return logging.DEBUG
		case "info":
			return logging.INFO
		case "warning":
			return logging.WARNING
		case "error":
			return logging.ERROR
		}
	}
	return logging.INFO
}

func getFatalErrorMsg() string {
	env := []string{runtime.GOOS, version.FullVersion()}
	if version.GetCommitHash() != "" {
		env = append(env, version.GetCommitHash())
	}
	envText := strings.Join(env, ", ")

	return fmt.Sprintf(`Error ----------------------------------

%s

Get Support ----------------------------
	Docs:          https://docs.gauge.org
	Bugs:          https://github.com/getgauge/gauge/issues
	Chat:          https://gitter.im/getgauge/chat

Your Environment Information -----------
	%s
	%s`, strings.Join(fatalErrors, "\n\n"),
		envText,
		getPluginVersions())
}

func addFatalError(module, msg string) {
	msg = strings.TrimSpace(msg)
	fatalErrors = append(fatalErrors, fmt.Sprintf("[%s]\n%s", module, msg))
}

func getPluginVersions() string {
	pis, err := pluginInfo.GetAllInstalledPluginsWithVersion()
	if err != nil {
		return fmt.Sprintf("Could not retrieve plugin information.")
	}
	pluginVersions := make([]string, 0, 0)
	for _, pi := range pis {
		pluginVersions = append(pluginVersions, fmt.Sprintf(`%s (%s)`, pi.Name, pi.Version))
	}
	return strings.Join(pluginVersions, ", ")
}
