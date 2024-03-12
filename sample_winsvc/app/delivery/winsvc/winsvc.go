package winsvc

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

const interrogateReportDelay = 100 * time.Second

var (
	elog           debug.Log
	done           chan struct{}
	ticker         *time.Ticker
	tickerDuration = 3 * time.Second
)

type SelfUpdateService struct {
	version string
}

func (s *SelfUpdateService) Execute(
	args []string,
	r <-chan svc.ChangeRequest,
	changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue

	changes <- svc.Status{State: svc.StartPending}

	ticker = time.NewTicker(tickerDuration)
	done = make(chan struct{})
	go func() {
		defer func() {
			zap.L().Debug("closing ticker (start) ...")
			zap.L().Debug("closing goroutine (start) ...")
			ticker.Stop()
		}()
		zap.L().Debug("starting goroutine (start) ...")
		for {
			select {
			case <-done:
				zap.L().Debug("stopping service ...")
				return
			case <-ticker.C:
				zap.L().Info(
					"tick",
					zap.String("version", s.version),
					zap.Any("datetime", time.Now()),
				)
			}
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case c := <-r:
			zap.L().Debug("received ChangeRequest", zap.Any("Cmd", c.Cmd))
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				time.Sleep(interrogateReportDelay)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				output := fmt.Sprintf("%s shutdown - %d", strings.Join(args, "-"), c.Context)
				elog.Info(1, output)
				select {
				case _, ok := <-done:
					if ok {
						close(done)
						zap.L().Debug("done channel closed", zap.Any("ChangeRequest", svc.Stop))
					} else {
						zap.L().Debug("done channel is already closed", zap.Any("ChangeRequest", svc.Stop))
					}
				default:
					close(done)
					zap.L().Debug("done channel closed", zap.Any("ChangeRequest", svc.Stop))
				}
				break loop
			case svc.Pause:
				select {
				case _, ok := <-done:
					if ok {
						close(done)
						zap.L().Debug("done channel closed", zap.Any("ChangeRequest", svc.Pause))
					} else {
						zap.L().Debug("done channel is already closed", zap.Any("ChangeRequest", svc.Pause))
					}
				default:
					close(done)
					zap.L().Debug("done channel closed", zap.Any("ChangeRequest", svc.Pause))
				}
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				zap.L().Debug("emit current status", zap.Any("status", c.CurrentStatus))
			case svc.Continue:
				ticker = time.NewTicker(tickerDuration)
				done = make(chan struct{})
				go func() {
					defer func() {
						zap.L().Debug("closing ticker (continue) ...")
						zap.L().Debug("closing goroutine (continue) ...")
						ticker.Stop()
					}()
					zap.L().Debug("starting goroutine (continue) ...")
					for {
						select {
						case <-done:
							zap.L().Debug("stopping service ...")
							return
						case <-ticker.C:
							zap.L().Info(
								"tick",
								zap.String("version", s.version),
								zap.Any("datetime", time.Now()),
							)
						}
					}
				}()
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				zap.L().Debug("emit current status", zap.Any("status", c.CurrentStatus))
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request: #%d", c))
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	zap.L().Debug("service stopped")

	return false, 0
}

func RunService(version string, name string, isDebug bool) {
	var err error

	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			zap.L().Error("open eventlog", zap.Error(err))
		}
	}
	defer elog.Close()

	elog.Info(0, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	service := SelfUpdateService{version: version}
	err = run(name, &service)
	if err != nil {
		elog.Error(0, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	elog.Info(0, fmt.Sprintf("stopped %s service", name))
}
