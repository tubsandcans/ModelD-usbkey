package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/rakyll/portmidi"
)

var inputDevID, outputDevID portmidi.DeviceID

var infoFile, errFile *os.File
var infolog, errlog *log.Logger

func run(quit, stop chan bool) {
	defer portmidi.Terminate()
	in, _ := portmidi.NewInputStream(inputDevID, 1024)
	defer in.Close()
	ch := in.Listen()
	out, _ := portmidi.NewOutputStream(outputDevID, 1024, 0)
	defer out.Close()
	for {
		select {
		case event := <-ch:
			if err := out.WriteShort(event.Status, event.Data1, event.Data2); err != nil {
				stop <- true
				return
			}
		case <-quit:
			return
		}
	}
}

func setup(done chan bool) {
	infolog.Println("Setup is being called")
	if err := portmidi.Initialize(); err != nil {
		errlog.Println(err)
		os.Exit(1)
	}
	ticker := time.NewTicker(time.Second * 2)
	if err := setDevices(); err != nil {
		errlog.Println(err.Error())
		portmidi.Terminate()
		time.Sleep(time.Second * 2)
		setup(done)
	} else {
		close(done)
		ticker.Stop()
	}
}

func setDevices() error {
	if err := portmidi.Initialize(); err != nil {
		return err
	}
	devs := portmidi.CountDevices()
	if devs == 0 {
		return fmt.Errorf("No midi devices found, returning now")
	}

	key := regexp.MustCompile("(?i)key")
	device := regexp.MustCompile("(?i)MODEL\\sD")
	foundModeld := false
	for i := 0; i < devs; i++ {
		devID := portmidi.DeviceID(i)
		minfo := portmidi.Info(devID)
		infolog.Printf("%d %v\n", i, minfo)
		if device.MatchString(minfo.Name) && minfo.IsOutputAvailable {
			infolog.Printf("Setting model D: %s\n", minfo.Name)
			outputDevID = devID
			foundModeld = true
		} else if key.MatchString(minfo.Name) && minfo.IsInputAvailable {
			infolog.Printf("Setting keyboard: %s\n", minfo.Name)
			inputDevID = devID
		}
	}

	if !foundModeld {
		return fmt.Errorf("Could not find MODEL D in midi devices")
	}
	return nil
}

func main() {
	if err := setupLogging(); err != nil {
		os.Exit(1)
	}
	doneSetup := make(chan bool)
	go setup(doneSetup)
	<-doneSetup

	quit := make(chan bool)
	stopRun := make(chan bool)
	go func() {
		run(quit, stopRun)
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	var exitCode int
	// Block until a signal is received.
	select {
	case s := <-sigs:
		quit <- true
		switch s {
		case syscall.SIGINT:
			infolog.Println("Exiting due to interrupt signal")
		case syscall.SIGKILL:
			infolog.Println("Exiting due to termination signal")
		case syscall.SIGTERM:
			infolog.Println("Exiting due to kill signal")
			exitCode = 1
		}
	case <-stopRun:
		exitCode = 1
	}
	close(quit)
	os.Exit(exitCode)
}

func setupLogging() (err error) {
	stdout := "/home/ubuntu/modeld_out.log"
	stderr := "/home/ubuntu/modeld_err.log"
	infoFile, err = os.OpenFile(stdout, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	errFile, err = os.OpenFile(stderr, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	infolog = log.New(infoFile, "INFO:", log.Ldate|log.Ltime)
	errlog = log.New(errFile, "ERROR:", log.Ldate|log.Ltime)
	return
}
