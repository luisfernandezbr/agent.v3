package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"

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

		producer, err := kafka.NewProducer(kafka.Config{
			Brokers: []string{"localhost:9092"},
		})
		if err != nil {
			exitWithErr("error creating producer", "err", err)
		}
		defer producer.Close()

		pinpointCustomerID := "5500a5ba8135f296"
		pinpointTestUUID := "94589bfa76fc25e459521b6b33f8d25acb71e37adc9200e3e9c38c09ad17d24a"
		pinpointJobID := "abc1234567890fedcba"

		enrollResponseCh := make(chan datamodel.ModelSendEvent)
		enrollResponseEmpty := make(chan bool)
		agent.NewEnrollResponseProducer(ctx, producer, enrollResponseCh, errors, enrollResponseEmpty)

		enrollAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("enroll request received") //+instance.Object().Stringify())

			req := instance.Object().(*agent.EnrollRequest)
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "wrong uuid header, was: %v, expected: %v")
			assertEqual("1234", req.Code, "expected code to be %v but was %v")
			assertValidInt64(req.RequestDate.Epoch, "expected request date epoch to be valid but was %v")
			assertValidRfc3339(req.RequestDate.Rfc3339, "expected request date Rfc3339 to be valid but was %v")
			assertValidInt64(req.UpdatedAt, "expected updatedAt to be valid but was %v")
			date := datetime.NewDateNow()
			// we have to send this separate because of the customer_id isn't on the original at the point we created the
			// action
			enrollResponseCh <- datamodel.NewModelSendEventWithHeaders(
				&agent.EnrollResponse{
					Apikey:     "01234567890123456789012345678901",
					CustomerID: pinpointCustomerID,
					EventDate: agent.EnrollResponseEventDate{
						Epoch:   date.Epoch,
						Rfc3339: date.Rfc3339,
						Offset:  date.Offset,
					},
				},
				map[string]string{
					"uuid":        instance.Message().Headers["uuid"],
					"customer_id": pinpointCustomerID,
				},
			)
			return nil, nil
		})

		newConfig := func(topic string, customerID string) action.Config {
			cfg := action.Config{
				GroupID: "agent-integration-test",
				Channel: "dev",
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
			assertNotEqual("", req.Version, "expected version to not be %v but was %v")
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
			logger.Info("sending integration request")
			return datamodel.NewModelSendEventWithHeaders(integrationRequest, headers), nil
		})

		var auth *string

		integrationAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
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
			assertTrue(strings.Contains(req.Message, "GitHub API Key user"), "expected message to contain GitHub API Key user")
			assertNotEqual("", req.Authorization, "expected authorization to not be %v but was %v")
			assertEqual("8102bf8537e3a947", req.RequestID, "expected request_id to be %v but was %v")

			auth = &req.Authorization

			date := datetime.NewDateNow()
			userRequest := &agent.UserRequest{
				CustomerID: pinpointCustomerID,
				UUID:       pinpointTestUUID,
				RefType:    "github",
				RequestDate: agent.UserRequestRequestDate{
					Epoch:   date.Epoch,
					Rfc3339: date.Rfc3339,
					Offset:  date.Offset,
				},
				Integration: agent.UserRequestIntegration{
					Name: "github",
					Authorization: agent.UserRequestIntegrationAuthorization{
						Authorization: auth,
					},
				},
			}
			headers := map[string]string{
				"customer_id": pinpointCustomerID,
				"uuid":        pinpointTestUUID,
			}
			return datamodel.NewModelSendEventWithHeaders(userRequest, headers), nil
		})

		userAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
			logger.Info("agent users received: ") //+instance.Object().Stringify())
			assertEqual(pinpointCustomerID, instance.Message().Headers["customer_id"], "missing customer id, expected %v, was %v")
			assertEqual(pinpointTestUUID, instance.Message().Headers["uuid"], "missing uuid, expected %v, was %v")
			req := instance.Object().(*agent.UserResponse)
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
			assertTrue(len(req.Users) > 0, "expected users array to be >0 but was %v")
			assertEqual(agent.UserResponseTypeUser.String(), req.Type.String(), "expected type to be %v but was %v")

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

		repoAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
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
			var exclusions []string
			for _, r := range req.Repos {
				if r.Name != "pinpt/test_repo" {
					// filter out non test_repo for the exclusion list
					exclusions = append(exclusions, r.Name)
				}
			}
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

		exportAction := action.NewAction(func(instance datamodel.ModelReceiveEvent) (datamodel.ModelSendEvent, error) {
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
				assertValidInt64(req.StartDate.Epoch, "expected start date epoch to be valid but was %v")
				assertValidRfc3339(req.StartDate.Rfc3339, "expected start date Rfc3339 to be valid but was %v")
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

		agentEnabledSub, err := action.Register(ctx, agentEnabledAction, newConfig(agent.EnabledTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering enabled action", "err", err)
		}
		defer agentEnabledSub.Close()

		integrationSub, err := action.Register(ctx, integrationAction, newConfig(agent.IntegrationResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering integration action", "err", err)
		}
		defer integrationSub.Close()

		userSub, err := action.Register(ctx, userAction, newConfig(agent.UserResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering user action", "err", err)
		}
		defer userSub.Close()

		repoSub, err := action.Register(ctx, repoAction, newConfig(agent.RepoResponseTopic.String(), pinpointCustomerID))
		if err != nil {
			exitWithErr("error registering repo action", "err", err)
		}
		defer repoSub.Close()

		exportSub, err := action.Register(ctx, exportAction, newConfig(agent.ExportResponseTopic.String(), pinpointCustomerID))
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
		<-enrollResponseEmpty
	},
}

func init() {
	cmd := cmdTestBackendMock
	cmdRoot.AddCommand(cmd)
}
