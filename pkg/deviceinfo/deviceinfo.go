package deviceinfo

import (
	"reflect"
	"time"

	"github.com/pinpt/agent/pkg/sysinfo"
	"github.com/pinpt/go-common/datetime"
)

func SystemID() string {
	return sysinfo.GetID()
}

var started = time.Now()

type CommonInfo struct {
	CustomerID string
	DeviceID   string
	SystemID   string
	Root       string
}

func (o *CommonInfo) AppendCommonInfo(event interface{}) {

	systemInfo := sysinfo.GetSystemInfo(o.Root)
	t := reflect.ValueOf(event).Elem()
	typ := t.Type()
	ms := (reflect.New(typ).Elem()).Interface()
	st := reflect.TypeOf(ms)

	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		val := t.FieldByName(field.Name)
		if field.Name == "CustomerID" {
			val.Set(reflect.ValueOf(o.CustomerID))
		} else if field.Name == "UUID" {
			val.Set(reflect.ValueOf(o.DeviceID))
		} else if field.Name == "SystemID" {
			val.Set(reflect.ValueOf(o.SystemID))
		} else if field.Name == "OS" {
			os := systemInfo.OS
			val.Set(reflect.ValueOf(os))
		} else if field.Name == "Version" {
			version := systemInfo.AgentVersion
			val.Set(reflect.ValueOf(version))
		} else if field.Name == "Hostname" {
			hostName := systemInfo.Hostname
			val.Set(reflect.ValueOf(hostName))
		} else if field.Name == "Distro" {
			distro := systemInfo.Name + " " + systemInfo.Version
			val.Set(reflect.ValueOf(distro))
		} else if field.Name == "NumCPU" {
			numCPU := int64(systemInfo.NumCPU)
			val.Set(reflect.ValueOf(numCPU))
		} else if field.Name == "FreeSpace" {
			freeSpace := 0
			val.Set(reflect.ValueOf(freeSpace))
		} else if field.Name == "GoVersion" {
			goVersion := systemInfo.GoVersion
			val.Set(reflect.ValueOf(goVersion))
		} else if field.Name == "Architecture" {
			architecture := systemInfo.Architecture
			val.Set(reflect.ValueOf(architecture))
		} else if field.Name == "Memory" {
			memory := int64(systemInfo.Memory)
			val.Set(reflect.ValueOf(memory))
		} else if field.Name == "EventDate" {
			dt := datetime.NewDateNow()
			ms2 := (reflect.New(field.Type).Elem()).Interface()
			st2 := reflect.TypeOf(ms2)
			for e := 0; e < st2.NumField(); e++ {
				field2 := st2.Field(e)
				val2 := val.FieldByName(field2.Name)
				if field2.Name == "Epoch" {
					val2.Set(reflect.ValueOf(dt.Epoch))
				} else if field2.Name == "Rfc3339" {
					val2.Set(reflect.ValueOf(dt.Rfc3339))
				} else if field2.Name == "Offset" {
					val2.Set(reflect.ValueOf(dt.Offset))
				}
			}
		} else if field.Name == "UpdatedAt" {
			dt := datetime.EpochNow()
			val.Set(reflect.ValueOf(dt))
		} else if field.Name == "Uptime" {
			// TODO: it would be better to refactor this and have some info passed based on the app run, not depending on init
			ts := datetime.TimeToEpoch(time.Now()) - datetime.TimeToEpoch(started)
			val.Set(reflect.ValueOf(ts))
		}
	}
}
