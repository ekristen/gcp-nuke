package global

import (
	"fmt"
	"path"
	"runtime"

	"github.com/ekristen/libnuke/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Flags() []cli.Flag {
	globalFlags := []cli.Flag{
		&cli.StringFlag{
			Name:    "log-level",
			Usage:   "Log Level",
			Aliases: []string{"l"},
			EnvVars: []string{"LOGLEVEL"},
			Value:   "info",
		},
		&cli.BoolFlag{
			Name:  "log-caller",
			Usage: "log the caller (aka line number and file)",
		},
		&cli.BoolFlag{
			Name:  "log-disable-color",
			Usage: "disable log coloring",
		},
		&cli.BoolFlag{
			Name:  "log-full-timestamp",
			Usage: "force log output to always show full timestamp",
		},
		&cli.StringFlag{
			Name:    "log-format",
			Usage:   "log format",
			Value:   "standard",
			EnvVars: []string{"GCP_NUKE_LOG_FORMAT"},
		},
		&cli.BoolFlag{
			Name:    "json",
			Usage:   "output as json, shorthand for --log-format=json",
			EnvVars: []string{"GCP_NUKE_LOG_FORMAT_JSON"},
		},
	}

	return globalFlags
}

func Before(c *cli.Context) error {
	formatter := &logrus.TextFormatter{
		DisableColors: c.Bool("log-disable-color"),
		FullTimestamp: c.Bool("log-full-timestamp"),
	}
	if c.Bool("log-caller") {
		logrus.SetReportCaller(true)

		formatter.CallerPrettyfier = func(f *runtime.Frame) (string, string) {
			return "", fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
		}
	}

	logFormatter := &log.CustomFormatter{
		FallbackFormatter: formatter,
	}

	if c.Bool("json") {
		_ = c.Set("log-format", "json")
	}

	switch c.String("log-format") {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{
			DisableHTMLEscape: true,
		})
		// note: this is a hack to remove the _handler key from the log output
		logrus.AddHook(&StructuredHook{})
	case "kv":
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		})
	default:
		logrus.SetFormatter(logFormatter)
	}

	switch c.String("log-level") {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	}

	return nil
}

type StructuredHook struct {
}

func (h *StructuredHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *StructuredHook) Fire(e *logrus.Entry) error {
	if e.Data == nil {
		return nil
	}

	delete(e.Data, "_handler")

	return nil
}
