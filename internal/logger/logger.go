package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
)

var levelColors = map[Level]string{
	DEBUG: dim,
	INFO:  green,
	WARN:  yellow,
	ERROR: red,
	FATAL: bold + red,
}

var levelLabels = map[Level]string{
	DEBUG: "DEBU",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERRO",
	FATAL: "FATL",
}

type Module struct {
	name  string
	color string
}

var modules = map[string]*Module{}
var modulesMu sync.RWMutex

func registerModule(name, color string) *Module {
	m := &Module{name: name, color: color}
	modulesMu.Lock()
	modules[name] = m
	modulesMu.Unlock()
	return m
}

var (
	ModMirror     = registerModule("mirror", cyan)
	ModServer     = registerModule("server", green)
	ModScan       = registerModule("scan", blue)
	ModBlacklist  = registerModule("blacklist", yellow)
	ModFirewall   = registerModule("firewall", magenta)
	ModStats      = registerModule("stats", magenta)
	ModDB         = registerModule("db", white)
	ModDownload   = registerModule("download", cyan)
	ModSelfUpdate = registerModule("selfupdate", green)
	ModAuth       = registerModule("auth", red)
	ModSecurity   = registerModule("security", bold+red)
	ModCaptcha    = registerModule("captcha", yellow)
	ModConfig     = registerModule("config", yellow)
	ModURLCheck   = registerModule("urlcheck", blue)
)

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func colorize(s, color string) string {
	return color + s + reset
}

func formatMessage(level Level, mod *Module, format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	levelLabel := colorize(levelLabels[level], levelColors[level])
	modLabel := colorize(fmt.Sprintf("[%s]", mod.name), mod.color)
	return fmt.Sprintf("%s %s %s %s",
		log.Default().Prefix(),
		levelLabel,
		modLabel,
		msg,
	)
}

func output(level Level, mod *Module, format string, args ...interface{}) {
	msg := formatMessage(level, mod, format, args...)
	if level >= ERROR {
		fmt.Fprintln(stderr, msg)
	} else {
		fmt.Fprintln(stdout, msg)
	}
	if level == FATAL {
		os.Exit(1)
	}
}

type Logger struct {
	mod *Module
}

func New(mod *Module) *Logger {
	return &Logger{mod: mod}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	output(DEBUG, l.mod, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	output(INFO, l.mod, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	output(WARN, l.mod, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	output(ERROR, l.mod, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	output(FATAL, l.mod, format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	output(DEBUG, l.mod, format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	output(INFO, l.mod, format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	output(WARN, l.mod, format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	output(ERROR, l.mod, format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	output(FATAL, l.mod, format, args...)
}

func Debug(mod *Module, format string, args ...interface{}) {
	output(DEBUG, mod, format, args...)
}

func Info(mod *Module, format string, args ...interface{}) {
	output(INFO, mod, format, args...)
}

func Warn(mod *Module, format string, args ...interface{}) {
	output(WARN, mod, format, args...)
}

func Error(mod *Module, format string, args ...interface{}) {
	output(ERROR, mod, format, args...)
}

func Fatal(mod *Module, format string, args ...interface{}) {
	output(FATAL, mod, format, args...)
}

func Init() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("")
	log.SetOutput(io.Discard)
}
