package cmdserviceinstall

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/kardianos/service"
	kservice "github.com/kardianos/service"
)

func Run() {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	conf := KServiceConfig()
	conf.Executable = execPath
	//conf.WorkingDirectory = agentDir

	serv, err := kservice.New(nil, conf)
	if err != nil {
		panic(err)
		//log.Fatal(logger, "error creating service", "err", err)
	}
	err = serv.Install()
	if err != nil {
		// if we get this error on windows, it's because the user needs Administrator rights to install a Windows service
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "Access is denied") {
			panic(err)
			//log.Fatal(logger, "Access was defined installing the agent as a Windows Service. You will need to re-run this command as Administrator so that the agent can be installed with the right permissions.")
		}
		if strings.Contains(err.Error(), "already exists") {
			fmt.Println("already installed")
			//log.Debug(logger, "agent already installed, will attempt to start it")
		} else {
			// other error
			panic(err)
		}
	}
	err = serv.Start()
	if err != nil {
		panic(err)
		//log.Fatal(logger, "error starting service", "err", err)
	}
	fmt.Println("ok")
	//log.Info(logger, "service was started")
}

func KServiceConfig() *kservice.Config {
	res := &kservice.Config{
		Name:        "io.pinpt.agent",
		DisplayName: "Pinpoint Agent",
		Description: "The Pinpoint Agent will process data and send to Pinpoint",
		Option: service.KeyValue{
			"RunAtLoad": true,
		},
	}
	if runtime.GOOS != "linux" {
		res.Option["UserService"] = true
	}
	return res
}
