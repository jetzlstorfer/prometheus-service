package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	cloudevents "github.com/cloudevents/sdk-go"
	cloudeventsclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"
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

type problem struct {
	State          string `json:"state"`
	ProblemID      string `json:"problemID"`
	PID            string `json:"pid"`
	ProblemTitle   string `json:"problemtitle"`
	ProblemDetails string `json:"problemdetails"`
	ImpactedEntity string `json:"impactedEntity"`
}

const eventBrokerURL = "http://event-broker.keptn.svc.cluster.local/keptn"

func main() {
	http.HandleFunc("/", Handler)
	http.ListenAndServe(":8080", nil)
}

// Handler
func Handler(rw http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var event alertManagerEvent
	err := decoder.Decode(&event)
	if err != nil {
		panic(err)
	}

	newProblemData := problem{
		State:          "OPEN",
		ProblemID:      "",
		PID:            "",
		ProblemTitle:   event.Alerts[0].Annotations.Summary,
		ProblemDetails: event.Alerts[0].Annotations.Description,
		ImpactedEntity: event.Alerts[0].Labels.PodName,
	}

	err = createAndSendCE(newProblemData)
	if err != nil {
		fmt.Println(err.Error())
		rw.WriteHeader(500)
	} else {
		rw.WriteHeader(201)
	}

}

func createAndSendCE(problemData problem) error {

	shkeptncontext := "test"

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

	fmt.Println(ce.String())

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
