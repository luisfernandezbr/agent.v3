package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/datamodel"
	"github.com/pinpt/go-common/datetime"
	"github.com/pinpt/go-common/event/action"
	"github.com/pinpt/go-common/kafka"
	pos "github.com/pinpt/go-common/os"
	pstrings "github.com/pinpt/go-common/strings"
	sdk "github.com/pinpt/integration-sdk"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/spf13/cobra"
)

// TODO:
// - check progress states and make sure the end progress state has start/stopped and that the totals add up
// - run JIRA and other integrations (check env before running)
// - test logs
// - test crash
// - make a switch to allow it to run the agent and then after the test exit
// - test upgrade (mock out upgrade server)

type modelfactory struct {
}

func (f *modelfactory) New(name datamodel.ModelNameType) datamodel.Model {
	return sdk.New(name)
}

type ppLogger struct {
	l hclog.Logger
}

func (s ppLogger) Log(keyvals ...interface{}) error {
	msg := ""
	var kv []interface{}
	for i := 0; i < len(keyvals); i += 2 {
		k := keyvals[i]
		v := keyvals[i+1]
		switch k {
		case "msg":
			msg = v.(string)
		case "pkg":
		case "level":
		case "ts":
		default:
			kv = append(kv, k, fmt.Sprintf("%+v", v))
		}
	}
	s.l.Info(msg, kv...)
	return nil
}

func createdTempUUID() (string, error) {
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	fileName := os.TempDir() + "tempuuid.tmp"

	uuid := strconv.FormatInt(int64(seededRand.Int()), 10)

	fmt.Println("UUID", uuid)

	if err := ioutil.WriteFile(fileName, []byte(uuid), 0644); err != nil {
		return "", fmt.Errorf("error writting to tempfile %s", err)
	}

	return uuid, nil
}

var cmdTestBackendMock = &cobra.Command{
	Use:   "test-backend-mock",
	Short: "Run test backend mock and check the received messages",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		ppLogger := ppLogger{l: logger}

		exitWithErr := func(msg string, args ...interface{}) {
			logger.Error(msg, args...)
			os.Exit(1)
		}

		pinpointTestUUID, err := createdTempUUID()
		if err != nil {
			exitWithErr("error creating temp file", "err", err)
		}

		if os.Getenv("PP_GITHUB_TOKEN") == "" {
			exitWithErr("set env PP_GITHUB_TOKEN and re-run")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan bool, 1)

		pos.OnExit(func(_ int) {
			cancel()
			done <- true
		})

		f := &modelfactory{}

		errors := make(chan error)
		go func() {
			for err := range errors {
				logger.Error(err.Error())
			}
		}()

		assertEqual := func(expected, val interface{}, msg string) {
			if reflect.DeepEqual(expected, val) {
				return
			}
			if expected == nil {
				if val == nil {
					return
				}
				if v, ok := val.(*interface{}); ok {
					if v == nil {
						return
					}
				}
				return
			}
			errors <- fmt.Errorf(msg, expected, val)
		}
		assertNotEqual := func(expected, val interface{}, msg string) {
			if !reflect.DeepEqual(expected, val) {
				return
			}
			errors <- fmt.Errorf(msg, expected, val)
		}
		assertValidInt64 := func(val int64, msg string) {
			if val > 0 {
				return
			}
			errors <- fmt.Errorf(msg, val)
		}
		assertValidRfc3339 := func(val string, msg string) {
			if val != "" {
				_, err := datetime.ISODateOffsetToTime(val)
				if err == nil {
					return
				}
			}
			errors <- fmt.Errorf(msg, val)
		}
		assertTrue := func(val bool, msg string) {
			if val {
				return
			}
			errors <- fmt.Errorf(msg)
		}
		assertNoError := func(err error, msg string) {
			if err == nil {
				return
			}
			errors <- fmt.Errorf(msg)
		}

		producer, err := kafka.NewProducer(kafka.Config{
			Brokers: []string{"localhost:9092"},
		})
		if err != nil {
			exitWithErr("error creating producer", "err", err)
		}
		defer producer.Close()

		pinpointCustomerID := "5500a5ba8135f296"
		pinpointJobID := "abc1234567890fedcba"
		pinpointCODE := "1234"
		pinpointChannel := "dev"
		pinpointAPIKey := "PQ2EUHYfb69KY2O6Gpl/6NvdUt5CA8OKexW36OTttSwpLEzrdUOcH7G+jfCdnqvHevW/Bu22+0nRVoqxBlDfnyeFO78wKgXoztMzhdAFvLKhPWmLdT3wfOYvto3nAPxd8QEqLpS/cliJrgiUjQw+tPaoA1sR5lRHJHAF0E5V+6nR9Hcjrwo3r38GKK4leM0P7vEpyyX1P9v4WE7iCIy5N3umKY8UUtaEPyPWq5bX16dDKJJGmQ=="

		assertNoError(err, "err expected to not be error")

		enrollAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("enroll request received") //+instance.Object().Stringify())

			req := instance.Object().(*agent.EnrollRequest)
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "wrong uuid header, was: %v, expected: %v")
			assertEqual(pinpointCODE, req.Code, "expected code to be %v but was %v")
			assertValidInt64(req.RequestDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.RequestDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			date := datetime.NewDateNow()

			enrollResponse := &agent.EnrollResponse{
				Apikey:     pinpointAPIKey,
				CustomerID: pinpointCustomerID,
				EventDate: agent.EnrollResponseEventDate{
					Epoch:   date.Epoch,
					Rfc3339: date.Rfc3339,
					Offset:  date.Offset,
				},
			}

			headers := map[string]string{
				"uuid": pinpointTestUUID,
			}
			defer logger.Info("enroll response sent")
			return datamodel.NewModelSendEventWithHeaders(enrollResponse, headers), nil

		})

		newConfig := func(topic string, customerID string) action.Config {
			cfg := action.Config{
				GroupID: "agent-integration-test",
				Channel: pinpointChannel,
				Errors:  errors,
				Topic:   topic,
				Factory: f,
				Offset:  "earliest",
				Logger:  ppLogger,
				Headers: map[string]string{"uuid": pinpointTestUUID},
			}
			if customerID != "" {
				cfg.Headers["customer_id"] = customerID
			}
			return cfg
		}

		enrollActionSub, err := action.Register(ctx, enrollAction, newConfig(agent.EnrollRequestTopic.String(), ""))
		if err != nil {
			exitWithErr("error registering enroll action", "err", err)
		}
		defer enrollActionSub.Close()

		agentEnabledAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent enabled received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.Enabled)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual("", req.Message, "expected message to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertEqual("", req.RefType, "expected ref_type to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be [%v] but was [%v]")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			assertEqual(agent.EventTypeEnroll.String(), req.Type.String(), "expected type to be %v but was %v")

			date := datetime.NewDateNow()
			integrationRequest := &agent.IntegrationRequest{
				CustomerID: pinpointCustomerID,
				UUID:       pinpointTestUUID,
				RequestDate: agent.IntegrationRequestRequestDate{
					Epoch:   date.Epoch,
					Rfc3339: date.Rfc3339,
					Offset:  date.Offset,
				},
				Integration: agent.IntegrationRequestIntegration{
					Name: "github",
					Authorization: agent.IntegrationRequestIntegrationAuthorization{
						URL:      pstrings.Pointer("https://api.github.com"),
						APIToken: pstrings.Pointer(os.Getenv("PP_GITHUB_TOKEN")),
					},
				},
			}
			headers := map[string]string{
				"customer_id": pinpointCustomerID,
				"uuid":        pinpointTestUUID,
			}
			defer logger.Info("integration request sent")
			time.Sleep(time.Second * 20)
			return datamodel.NewModelSendEventWithHeaders(integrationRequest, headers), nil
		})

		var auth *string

		integrationResponseAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent integration received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.IntegrationResponse)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertEqual("github", req.RefType, "expected ref_type to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			assertEqual(agent.IntegrationResponseTypeIntegration.String(), req.Type.String(), "expected type to be %v but was %v")
			assertTrue(strings.Contains(req.Message, "Success. Integration validated."), "expected message to contain Success. Integration validated.")
			assertNotEqual("", req.Authorization, "expected authorization to not be %v but was %v")
			assertEqual("8102bf8537e3a947", req.RequestID, "expected request_id to be %v but was %v")

			auth = &req.Authorization

			date := datetime.NewDateNow()
			repoRequest := &agent.RepoRequest{
				CustomerID: pinpointCustomerID,
				UUID:       pinpointTestUUID,
				RefType:    "github",
				RequestDate: agent.RepoRequestRequestDate{
					Epoch:   date.Epoch,
					Rfc3339: date.Rfc3339,
					Offset:  date.Offset,
				},
				Integration: agent.RepoRequestIntegration{
					Name: "github",
					Authorization: agent.RepoRequestIntegrationAuthorization{
						Authorization: auth,
					},
				},
			}
			headers := map[string]string{
				"customer_id": pinpointCustomerID,
				"uuid":        pinpointTestUUID,
			}
			defer logger.Info("repo request sent")
			return datamodel.NewModelSendEventWithHeaders(repoRequest, headers), nil
		})

		httpserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger.Info("incoming HTTP request", "method", req.Method)
			if req.Method != http.MethodPut {
				exitWithErr("incoming HTTP request should have been a PUT but was " + req.Method)
			}
			buf, _ := ioutil.ReadAll(req.Body)
			req.Body.Close()
			logger.Info("incoming HTTP body", "len", len(buf))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			assertValidInt64(int64(len(buf)), "expected HTTP body to be greater than 0 but was %v")
		}))
		defer httpserver.Close()

		repoResponseAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent repos received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.RepoResponse)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertEqual("github", req.RefType, "expected ref_type to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			assertTrue(len(req.Repos) > 0, "expected repos array to be >0 but was %v")
			assertEqual(agent.RepoResponseTypeRepo.String(), req.Type.String(), "expected type to be %v but was %v")

			exclusions := []string{
				"MDEwOlJlcG9zaXRvcnkxNjM4MDQwOTQ=",
				"MDEwOlJlcG9zaXRvcnkxODUyMjY1MDk=",
				"MDEwOlJlcG9zaXRvcnk5ODY4NjQxMQ==",
				"MDEwOlJlcG9zaXRvcnkxMjI0MTk3MDg=",
				"MDEwOlJlcG9zaXRvcnkxNDc0Mjg3MTc=",
				"MDEwOlJlcG9zaXRvcnkxNTQ3NzUyNjA=",
				"MDEwOlJlcG9zaXRvcnkxNTU5MzUwMzc=",
				"MDEwOlJlcG9zaXRvcnkxODY3MzgzODQ=",
				"MDEwOlJlcG9zaXRvcnkyMDEwOTI0Njk=",
				"MDEwOlJlcG9zaXRvcnkyMTQ0ODY5MTk=",
				// "MDEwOlJlcG9zaXRvcnk3OTYxMDg3OQ==", test_repo
				"MDEwOlJlcG9zaXRvcnkxMjg4MTEzODU=",
				"MDEwOlJlcG9zaXRvcnkxNDAwMDQ3MzM=",
				"MDEwOlJlcG9zaXRvcnkxNDAwMTAwOTM=",
				"MDEwOlJlcG9zaXRvcnkxNDA0NjE5MDg=",
				"MDEwOlJlcG9zaXRvcnkxNjcyNjIyMTE=",
				"MDEwOlJlcG9zaXRvcnkxNDU5ODk3NzM=",
				"MDEwOlJlcG9zaXRvcnkxNDY2MjQ3MTI=",
				"MDEwOlJlcG9zaXRvcnkxNTEyMTAxODY=",
				"MDEwOlJlcG9zaXRvcnkxNjM0NjMxNjg=",
				"MDEwOlJlcG9zaXRvcnkxOTA5MTg3OTM=",
				"MDEwOlJlcG9zaXRvcnkxMjQ2MzEzNTA=",
				"MDEwOlJlcG9zaXRvcnkxMzQ3NTYxODc=",
				"MDEwOlJlcG9zaXRvcnkxMzgzNDc1MjU=",
				"MDEwOlJlcG9zaXRvcnkxNDY1Mzc0MDI=",
				"MDEwOlJlcG9zaXRvcnkxNzc1ODA4MTE=",
				"MDEwOlJlcG9zaXRvcnkxNzgwOTk4Mjk=",
				"MDEwOlJlcG9zaXRvcnkxODIxNjY5MjM=",
				"MDEwOlJlcG9zaXRvcnk3NzExODU4OQ==",
				"MDEwOlJlcG9zaXRvcnkxMjE2NzQ0Nzg=",
				"MDEwOlJlcG9zaXRvcnkxMjI0MjAwNDk=",
				"MDEwOlJlcG9zaXRvcnkxNDkxNjI4ODQ=",
				"MDEwOlJlcG9zaXRvcnkxNTkwNTc2Njg=",
				"MDEwOlJlcG9zaXRvcnkxODUyMjkyMDQ=",
				"MDEwOlJlcG9zaXRvcnkxODc1NTQ3ODk=",
				"MDEwOlJlcG9zaXRvcnkxODc5Mjg1NDc=",
				"MDEwOlJlcG9zaXRvcnk5MTc2ODc2NQ==",
				"MDEwOlJlcG9zaXRvcnk5NDgyMjY1Mw==",
				"MDEwOlJlcG9zaXRvcnkxMTkwMDc2MTI=",
				"MDEwOlJlcG9zaXRvcnkxNDY1NDIwMDg=",
				"MDEwOlJlcG9zaXRvcnkxNDc4ODMwMDY=",
				"MDEwOlJlcG9zaXRvcnkxNTQzNjUzNTU=",
				"MDEwOlJlcG9zaXRvcnkxNTY2NjE1NTM=",
				"MDEwOlJlcG9zaXRvcnkxNjMyMDE3MzE=",
				"MDEwOlJlcG9zaXRvcnk5MjU0NzU5Mw==",
				"MDEwOlJlcG9zaXRvcnkxMTcxMzc3NjA=",
				"MDEwOlJlcG9zaXRvcnkxNDA1MDY4NzU=",
				"MDEwOlJlcG9zaXRvcnkxODMwNzcyMzQ=",
				"MDEwOlJlcG9zaXRvcnkyMTQyMzk5MTU=",
				"MDEwOlJlcG9zaXRvcnkxMjkwNTk3ODk=",
				"MDEwOlJlcG9zaXRvcnkxNTI4MDk4ODI=",
				"MDEwOlJlcG9zaXRvcnkxODI1ODcwMzA=",
				"MDEwOlJlcG9zaXRvcnkxODU0NDA1MDA=",
				"MDEwOlJlcG9zaXRvcnkxOTI0MTU3MzE=",
				"MDEwOlJlcG9zaXRvcnk5MTg0NTM4Nw==",
				"MDEwOlJlcG9zaXRvcnkxMDY2NDI1ODQ=",
				"MDEwOlJlcG9zaXRvcnkxMjI0MzQ0NTc=",
				"MDEwOlJlcG9zaXRvcnkxNjY5NzI1OTU=",
				"MDEwOlJlcG9zaXRvcnkxMTc0MDAzODc=",
				"MDEwOlJlcG9zaXRvcnkxMTc3ODE3MjQ=",
				"MDEwOlJlcG9zaXRvcnkxMzYxMTc0ODU=",
				"MDEwOlJlcG9zaXRvcnkxNjExMTY0Nzk=",
				"MDEwOlJlcG9zaXRvcnkxODU3NzcyMDU=",
				"MDEwOlJlcG9zaXRvcnkxODY4NDYxOTg=",
				"MDEwOlJlcG9zaXRvcnkxNjQ0NTgxOTM=",
				"MDEwOlJlcG9zaXRvcnkxNjcyNjI0OTQ=",
				"MDEwOlJlcG9zaXRvcnkxNzYzNjM1ODg=",
				"MDEwOlJlcG9zaXRvcnkxMzkyOTk4MDc=",
				"MDEwOlJlcG9zaXRvcnkxNjM0NTEzNTc=",
				"MDEwOlJlcG9zaXRvcnkxNjM4NTYyOTE=",
				"MDEwOlJlcG9zaXRvcnkxNjc0NjIyNjQ=",
				"MDEwOlJlcG9zaXRvcnkxODU0Mjc3NTM=",
				"MDEwOlJlcG9zaXRvcnkxODY0ODE1MTk=",
				"MDEwOlJlcG9zaXRvcnkxMzMzOTg5MDY=",
				"MDEwOlJlcG9zaXRvcnkxMzkyODg2NTY=",
				"MDEwOlJlcG9zaXRvcnkxMzk2MjYxNjY=",
				"MDEwOlJlcG9zaXRvcnkxMjczMTcyNDA=",
				"MDEwOlJlcG9zaXRvcnkxNTUyNjc4MTc=",
				"MDEwOlJlcG9zaXRvcnkxNzU0NzUwNDc=",
				"MDEwOlJlcG9zaXRvcnkxNzYzOTQzMzA=",
				"MDEwOlJlcG9zaXRvcnkxODYwNDg4OTU=",
				"MDEwOlJlcG9zaXRvcnk3MDA3NTYxNA==",
				"MDEwOlJlcG9zaXRvcnk4MzkxMDY2Mw==",
				"MDEwOlJlcG9zaXRvcnkxMTUzNzc1NDA=",
				"MDEwOlJlcG9zaXRvcnkxOTEwNzg0ODU=",
				"MDEwOlJlcG9zaXRvcnkyMDM5MjUyMDk=",
				"MDEwOlJlcG9zaXRvcnkyMTE1NDI1NTg=",
				"MDEwOlJlcG9zaXRvcnkxNTg4MDA3MzY=",
				"MDEwOlJlcG9zaXRvcnkxNjMyMTY4MjY=",
				"MDEwOlJlcG9zaXRvcnkxOTk5OTY3OTc=",
				"MDEwOlJlcG9zaXRvcnkyMDI5NDAxNTg=",
				"MDEwOlJlcG9zaXRvcnk5ODIxOTU0OA==",
				"MDEwOlJlcG9zaXRvcnkxMzI4OTc2ODQ=",
				"MDEwOlJlcG9zaXRvcnkxNTc3NTkwNzc=",
				"MDEwOlJlcG9zaXRvcnkxMTc1OTg4Nzg=",
				"MDEwOlJlcG9zaXRvcnkxNDUwNTcwNjA=",
				"MDEwOlJlcG9zaXRvcnkxNjMyMDA0NDQ=",
				"MDEwOlJlcG9zaXRvcnkyMTA5NjEwMDE=",
				"MDEwOlJlcG9zaXRvcnk4NTAyNjAzNA==",
				"MDEwOlJlcG9zaXRvcnk5MDc2MjI5Mw==",
				"MDEwOlJlcG9zaXRvcnk5NDkzNjA4NQ==",
				"MDEwOlJlcG9zaXRvcnk5MTA5OTM5NQ==",
				"MDEwOlJlcG9zaXRvcnkxNTM3MTgyNzA="}
			date := datetime.NewDateNow()
			exportReq := &agent.ExportRequest{
				CustomerID:          pinpointCustomerID,
				UUID:                pinpointTestUUID,
				JobID:               pinpointJobID,
				UploadURL:           pstrings.Pointer(httpserver.URL),
				ReprocessHistorical: true,
				RequestDate: agent.ExportRequestRequestDate{
					Epoch:   date.Epoch,
					Rfc3339: date.Rfc3339,
					Offset:  date.Offset,
				},
				Integrations: []agent.ExportRequestIntegrations{
					agent.ExportRequestIntegrations{
						Exclusions: exclusions,
						Authorization: agent.ExportRequestIntegrationsAuthorization{
							Authorization: auth,
						},
						Name: "github",
					},
				},
			}
			headers := map[string]string{
				"customer_id": pinpointCustomerID,
				"uuid":        pinpointTestUUID,
			}
			return datamodel.NewModelSendEventWithHeaders(exportReq, headers), nil
		})

		exportRequestAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent export received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.ExportResponse)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			if req.RefType == "progress" {
				assertNotEqual(nil, req.Data, "expected data to be not %v but was %v")
			} else {
				assertEqual(nil, req.Data, "expected data to be %v but was %v")
				if req.RefType == "finish" || req.RefType == "error" {
					assertValidInt64(req.EndDate.Epoch, "expected end date epoch to be valid but was %v")
					assertValidRfc3339(req.EndDate.Rfc3339, "expected end date Rfc3339 to be valid but was %v")
				}
			}
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertEqual(pinpointJobID, req.JobID, "expected job_id to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			return nil, nil
		})

		pingAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent ping received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.Ping)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual("", req.Message, "expected message to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertEqual("", req.RefType, "expected ref_type to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to be %v but was %v")
			assertNotEqual("", req.ID, "expected id to be %v but was %v")
			assertNotEqual("", req.OS, "expected os to be %v but was %v")
			assertNotEqual("", req.Version, "expected version to be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			assertEqual(agent.EventTypePing.String(), req.Type.String(), "expected type to be %v but was %v")
			return nil, nil
		})

		startAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent start received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.Start)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			return nil, nil
		})

		stopAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent stop received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.Stop)
			assertEqual(pinpointCustomerID, req.CustomerID, "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, req.UUID, "missing uuid, expected %v, was %v")
			assertValidInt64(req.EventDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.EventDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			assertEqual(true, req.Success, "expected success to be %v but was %v")
			assertEqual(nil, req.Error, "expected error to be %v but was %v")
			assertEqual(nil, req.Data, "expected data to be %v but was %v")
			assertEqual("", req.RefID, "expected ref_id to be %v but was %v")
			assertNotEqual("", req.Architecture, "expected architecture to not be %v but was %v")
			assertNotEqual("", req.Distro, "expected distro to not be %v but was %v")
			assertNotEqual("", req.GoVersion, "expected go_version to not be %v but was %v")
			assertNotEqual("", req.Hostname, "expected hostname to not be %v but was %v")
			assertNotEqual("", req.ID, "expected id to not be %v but was %v")
			assertNotEqual("", req.OS, "expected os to not be %v but was %v")
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
			assertValidInt64(req.Memory, "expected memory to be valid but was %v")
			assertValidInt64(req.FreeSpace, "expected memory to be valid but was %v")
			assertValidInt64(req.NumCPU, "expected num_cpu to be valid but was %v")
			return nil, nil
		})

		agentEnabledSub, err := action.Register(ctx, agentEnabledAction, newConfig(agent.EnabledTopic.String(), ""))
		if err != nil {
			exitWithErr("error registering enabled action", "err", err)
		}
		defer agentEnabledSub.Close()

		integrationSub, err := action.Register(ctx, integrationResponseAction, newConfig(agent.IntegrationResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering integration action", "err", err)
		}
		defer integrationSub.Close()

		repoSub, err := action.Register(ctx, repoResponseAction, newConfig(agent.RepoResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering repo action", "err", err)
		}
		defer repoSub.Close()

		exportSub, err := action.Register(ctx, exportRequestAction, newConfig(agent.ExportResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering export action", "err", err)
		}
		defer exportSub.Close()

		pingSub, err := action.Register(ctx, pingAction, newConfig(agent.PingTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering ping action", "err", err)
		}
		defer pingSub.Close()

		startSub, err := action.Register(ctx, startAction, newConfig(agent.StartTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering start action", "err", err)
		}
		defer startSub.Close()

		stopSub, err := action.Register(ctx, stopAction, newConfig(agent.StopTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering stop action", "err", err)
		}
		defer stopSub.Close()

		<-done
	},
}

func init() {
	cmd := cmdTestBackendMock
	cmdRoot.AddCommand(cmd)
}
