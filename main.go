package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/rakyll/portmidi"
)

var inputDevID, outputDevID portmidi.DeviceID

func run(quit chan bool) {
	in, _ := portmidi.NewInputStream(inputDevID, 1024)
	defer in.Close()
	ch := in.Listen()
	out, _ := portmidi.NewOutputStream(outputDevID, 1024, 0)
	defer out.Close()
	for {
		select {
		case event := <-ch:
			out.WriteShort(event.Status, event.Data1, event.Data2)
		case <-quit:
			return
		}
	}
}

func setup() error {
	if err := portmidi.Initialize(); err != nil {
		return err
	}
	devs := portmidi.CountDevices()
	if devs == 0 {
		return fmt.Errorf("No midi devices found, returning now")
	}

	key := regexp.MustCompile("(?i)key")
	device := regexp.MustCompile("(?i)MODEL\\sD\\s")

	for i := 0; i < devs; i++ {
		devID := portmidi.DeviceID(i)
		minfo := portmidi.Info(devID)
		fmt.Println(minfo)
		if device.MatchString(minfo.Name) && minfo.IsOutputAvailable {
			fmt.Printf("Setting model D: %s\n", minfo.Name)
			outputDevID = devID
		} else if key.MatchString(minfo.Name) && minfo.IsInputAvailable {
			fmt.Printf("Setting keyboard: %s\n", minfo.Name)
			inputDevID = devID
		}
	}

	if inputDevID == outputDevID {
		return fmt.Errorf("One or more devices missing; in:%d, out:%d",
			inputDevID, outputDevID)
	}
	return nil
}

func main() {
	if err := setup(); err != nil {
		//TODO, need to wait and then try again, over and over..
		log.Fatal(err)
		os.Exit(0)
	}
	defer portmidi.Terminate()
	quit := make(chan bool)
	go func() {
		run(quit)
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	// Block until a signal is received.
	s := <-sigs
	quit <- true
	var exitCode int
	switch s {
	case syscall.SIGINT:
		log.Println("Exiting due to interrupt signal")
	case syscall.SIGKILL:
		log.Println("Exiting due to termination signal")
	case syscall.SIGTERM:
		log.Println("Exiting due to kill signal")
		exitCode = 1
	}

	os.Exit(exitCode)
}
