package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	cloudevents "github.com/cloudevents/sdk-go"
	cloudeventsclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"
	keptnevents "github.com/keptn/go-utils/pkg/events"
	keptnutils "github.com/keptn/go-utils/pkg/utils"
)

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int    `envconfig:"RCV_PORT" default:"8080"`
	Path string `envconfig:"RCV_PATH" default:"/"`
}

type alertManagerEvent struct {
	Receiver string `json:"receiver"`
	Status   string `json:"status"`
	Alerts   []alert
}

type alert struct {
	Status      string `json:"status"`
	Labels      labels
	Annotations annotations
	//StartsAt time   `json:"startsAt"`
	//EndsAt   time   `json:"endsAt"`
}

type labels struct {
	AlertName string `json:"alertname"`
	Namespace string `json:"namespace,omitempty"`
	PodName   string `json:"pod_name,omitempty"`
	Severity  string `json:"severity"`
}

type annotations struct {
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
}

const eventBrokerURL = "http://event-broker.keptn.svc.cluster.local/keptn"

func main() {
	shkeptncontext := ""
	logger := keptnutils.NewLogger(shkeptncontext, "", "alertmanager-service")
	logger.Debug("starting handler")
	http.HandleFunc("/", Handler)
	http.ListenAndServe(":8080", nil)
}

// Handler takes the prometheus alert as input
func Handler(rw http.ResponseWriter, req *http.Request) {
	shkeptncontext := ""
	logger := keptnutils.NewLogger(shkeptncontext, "", "alertmanager-service")
	logger.Debug("receiving event from prometheus alertmanager")

	decoder := json.NewDecoder(req.Body)
	var event alertManagerEvent
	err := decoder.Decode(&event)
	if err != nil {
		logger.Error("Could not map received event to datastructure: " + err.Error())
	}

	problemState := ""
	if event.Status == "firing" {
		problemState = "OPEN"
	}

	newProblemData := keptnevents.ProblemEventData{
		State:          problemState,
		ProblemID:      "",
		ProblemTitle:   event.Alerts[0].Annotations.Summary,
		ProblemDetails: event.Alerts[0].Annotations.Description,
		ImpactedEntity: event.Alerts[0].Labels.PodName,
	}

	logger.Debug("sending event to eventbroker")
	err = createAndSendCE(shkeptncontext, newProblemData)
	if err != nil {
		logger.Error("could not send cloud event: " + err.Error())
		rw.WriteHeader(500)
	} else {
		logger.Debug("event successfully dispatched to eventbroker")
		rw.WriteHeader(201)
	}

}

func createAndSendCE(shkeptncontext string, problemData keptnevents.ProblemEventData) error {
	source, _ := url.Parse("prometheus")
	contentType := "application/json"

	ce := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:          uuid.New().String(),
			Type:        "sh.keptn.event.problem",
			Source:      types.URLRef{URL: *source},
			ContentType: &contentType,
			Extensions:  map[string]interface{}{"shkeptncontext": shkeptncontext},
		}.AsV02(),
		Data: problemData,
	}

	t, err := cloudeventshttp.New(
		cloudeventshttp.WithTarget(eventBrokerURL),
		cloudeventshttp.WithEncoding(cloudeventshttp.StructuredV02),
	)
	if err != nil {
		return errors.New("Failed to create transport:" + err.Error())
	}

	c, err := cloudeventsclient.New(t)
	if err != nil {
		return errors.New("Failed to create HTTP client:" + err.Error())
	}

	if _, err := c.Send(context.Background(), ce); err != nil {
		return errors.New("Failed to send cloudevent:, " + err.Error())
	}

	return nil
}
