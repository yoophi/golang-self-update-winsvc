package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"go.uber.org/zap"
	"golang.org/x/sys/windows/svc"

	"golang-self-update-winsvc/app/delivery/winsvc"
)

var (
	serviceName        = "POC_SelfUpdateEx"
	serviceDescription = "POC Self Update Example"
)

var (
	version  = "1.3.0"
	basePath string
)

func doSelfUpdate() {
	v := semver.MustParse(version)
	latest, err := selfupdate.UpdateSelf(v, "yoophi/golang-self-update-winsvc")
	fmt.Println("latest", latest)
	if err != nil {
		log.Println("Binary update failed:", err)
		return
	}
	if latest.Version.Equals(v) {
		// latest version is the same as current version. It means current binary is up-to-date.
		zap.L().Info("current binary is the latest version", zap.String("version", version))
	} else {
		zap.L().Info("successfully updated to version",
			zap.Any("latest-version", latest.Version),
			zap.String("current-version", version),
		)
		zap.L().Info("latest-version release note", zap.Any("release-note", latest.ReleaseNotes))
		os.Exit(2)
	}
}

func init() {
	basePath = filepath.Dir(os.Args[0])
	logFileDir := filepath.Join(basePath, "logs")
	err := os.MkdirAll(logFileDir, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create log directory: %s\n", err)
		os.Exit(1)
	}

	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.OutputPaths = []string{filepath.Join(logFileDir, "sample.log")}
	logger, err := loggerConfig.Build()
	if err != nil {
		fmt.Printf("build logger: %+v\n", err)
	}
	zap.ReplaceGlobals(logger)
}

func main() {
	zap.L().Info("process started ...", zap.Any("args", os.Args))

	doSelfUpdate()

	inService, err := svc.IsWindowsService()
	if err != nil {
		zap.L().Error("check process is in windows service", zap.Error(err))
		return
	}

	if inService && os.Args[1] == "is" {
		zap.L().Debug("process is running as windows service")
		winsvc.RunService(version, serviceName, false)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("command not found:", basePath)
	}

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		winsvc.RunService(version, serviceName, true)
		return
	case "install":
		err = winsvc.InstallService(serviceName, serviceDescription)
	case "remove":
		err = winsvc.RemoveService(serviceName)
	case "start":
		err = winsvc.StartService(serviceName)
	case "stop":
		err = winsvc.ControlService(serviceName, svc.Stop, svc.Stopped)
	case "pause":
		err = winsvc.ControlService(serviceName, svc.Pause, svc.Paused)
	case "continue":
		err = winsvc.ControlService(serviceName, svc.Continue, svc.Running)
	case "version":
		fmt.Printf("service: %s\nversion: %s\n", serviceName, version)
		return
	default:
		zap.L().Info(
			"invalid command",
			zap.String("command", cmd),
		)
		fmt.Printf("invalid command: `%s`\n", cmd)
		os.Exit(1)
		return
	}
	if err != nil {
		zap.L().Error(
			"handle command",
			zap.String("command", cmd),
			zap.Error(err),
		)
		fmt.Printf("service error: %+v\n", err)
		os.Exit(1)
	}

	fmt.Printf("command `%s` success", cmd)
}
