package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	f "github.com/jessevdk/go-flags"
	constants "github.com/locktopus-project/locktopus/internal/constants"
)

var port string
var hostname string
var statInterval = 0
var defaultAbandonTimeout = time.Millisecond * constants.DefaultAbandonTimeoutMs

var arguments struct {
	Help                 bool   `short:"h" long:"help" description:"Show help message and exit"`
	Host                 string `short:"H" long:"host" description:"Hostname for listening. Overrides env var LOCKTOPUS_HOST. Default: 0.0.0.0"`
	Port                 string `short:"p" long:"port" description:"Port to listen on. Overrides env var LOCKTOPUS_PORT. Default: 9009"`
	LogClients           string `long:"log-clients" description:"Log client sessions (true/false). Overrides env var LOCKTOPUS_LOG_CLIENTS. Default: false"`
	LogLocks             string `long:"log-locks" description:"Log locks caused by client sessions (true/false). Overrides env var LOCKTOPUS_LOG_LOCKS. Default: false"`
	StatisticsInterval   string `long:"stats-interval" description:"Log usage statistics every N>0 seconds. Overrides env var LOCKTOPUS_STATS_INTERVAL. Default: 0 (never)"`
	GlobalAbandonTimeout string `long:"default-abandon-timeout" description:"Default abandon timeout (ms) used for releasing closed connections not released by clients. Overrides env var LOCKTOPUS_DEFAULT_ABANDON_TIMEOUT. Default: 60000"`
}

func parseArguments() {
	p := f.NewParser(&arguments, f.PassDoubleDash)

	_, err := p.Parse()
	if err != nil {
		fmt.Println(fmt.Errorf("cannot parse arguments: %w", err))
		fmt.Println()
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if arguments.Help {
		p.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	port = resolveStringParameter(arguments.Port, "PORT", constants.DefaultServerPort)
	hostname = resolveStringParameter(arguments.Host, "HOST", constants.DefaultServerHost)

	if resolveBoolParameter(arguments.LogClients, "LOG_CLIENTS", false) {
		apiLogger.Disable()
	}

	if resolveBoolParameter(arguments.LogLocks, "LOG_LOCKS", false) {
		lockLogger.Disable()
	}

	if v := resolveStringParameter(arguments.StatisticsInterval, "STATS_INTERVAL", ""); v != "" {
		interval, err := strconv.Atoi(v)

		if err != nil {
			mainLogger.Errorf("Cannot parse stats-interval value: %s", err)
			os.Exit(1)
			return
		}

		statInterval = interval
	}

	if v := resolveStringParameter(arguments.GlobalAbandonTimeout, "DEFAULT_ABANDON_TIMEOUT", ""); v != "" {
		timeoutMs, err := strconv.Atoi(v)

		if err != nil {
			mainLogger.Errorf("Cannot parse default-abandon-timeout value: %s", err)
			os.Exit(1)
			return
		}

		defaultAbandonTimeout = time.Millisecond * time.Duration(timeoutMs)
	}
}

func getEnvVar(name string) string {
	if e := os.Getenv(fmt.Sprintf("%v%s", constants.EnvPrefix, name)); e != "" {
		return e
	}

	return ""
}

func resolveStringParameter(argValue, envName, def string) string {
	if argValue != "" {
		return argValue
	}

	if e := getEnvVar(envName); e != "" {
		return e
	}

	return def
}

var trueStrings = []string{"true", "1", "yes", "y"}

func resolveBoolParameter(argValue, envName string, def bool) bool {
	if argValue != "" {
		return contains(trueStrings, argValue)
	}

	if e := getEnvVar(envName); e != "" {
		return contains(trueStrings, e)
	}

	return def
}

func contains[T comparable](arr []T, s T) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}

	return false
}
