/*
Copyright 2023 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"chainguard.dev/sdk/pkg/events"
	"chainguard.dev/sdk/pkg/events/policy"
	"chainguard.dev/sdk/pkg/events/receiver"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"

	jira "github.com/andygrunwald/go-jira"
)

type envConfig struct {
	Issuer    string `envconfig:"ISSUER_URL" required:"true"`
	Group     string `envconfig:"GROUP" required:"true"`
	Port      int    `envconfig:"PORT" default:"8080" required:"true"`
	User      string `envconfig:"JIRA_USER" required:"true"`
	Token     string `envconfig:"JIRA_TOKEN" required:"true"`
	BaseURL   string `envconfig:"JIRA_URL" required:"true"`
	Project   string `envconfig:"JIRA_PROJECT" required:"true"`
	IssueType string `envconfig:"ISSUE_TYPE" required:"true"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("failed to process env var: %s", err)
	}
	ctx := context.Background()

	transport := jira.BasicAuthTransport{
		Username: env.User,
		Password: env.Token,
	}

	client, err := jira.NewClient(transport.Client(), env.BaseURL)
	if err != nil {
		log.Fatalf("unable to auth to atlassian: %v", err)
	}

	receiver, err := receiver.New(ctx, env.Issuer, env.Group, func(ctx context.Context, event cloudevents.Event) error {
		// We are handling a specific event type, so filter the rest.
		if event.Type() != policy.ChangedEventType {
			return nil
		}

		data := events.Occurrence{Body: policy.ImagePolicyRecord{}}
		if err := event.DataAs(&data); err != nil {
			return cloudevents.NewHTTPResult(http.StatusBadRequest, "unable to unmarshal data: %w", err)
		}
		body := data.Body
		for name, pol := range body.Policies {
			if pol.Valid {
				// Not in violation of policy
				continue
			}
			switch pol.Change {
			case policy.ImprovedChange:
				// TODO: How is this an improvement?
				continue
			case policy.NewChange, policy.DegradedChange:
				// We want to fire on these events.
			}

			issue, _, err := client.Issue.Create(&jira.Issue{
				Fields: &jira.IssueFields{
					Description: strings.Join(
						[]string{
							fmt.Sprintf("Image:        `%s`", body.ImageID),
							fmt.Sprintf("Cluster       `%s`", body.ClusterID),
							fmt.Sprintf("Policy:       `%s`", name),
							fmt.Sprintf("Last Checked: `%v`", pol.LastChecked.Time),
							fmt.Sprintf("Diagnostic:   `%v`", pol.Diagnostic),
						}, "\n",
					),
					Type: jira.IssueType{
						Name: env.IssueType,
					},
					Project: jira.Project{
						Key: env.Project,
					},
					Summary: fmt.Sprintf("Policy %s failed", name),
				},
			})
			if err != nil {
				return cloudevents.NewHTTPResult(http.StatusInternalServerError, "unable to create Jira issue: %w", err)
			}
			log.Printf("Opened issue: %s", issue.Key)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("failed to create receiver: %v", err)
	}

	c, err := cloudevents.NewClientHTTP(cloudevents.WithPort(env.Port),
		// We need to infuse the request onto context, so we can
		// authenticate requests.
		cehttp.WithRequestDataAtContextMiddleware())
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	if err := c.StartReceiver(ctx, func(ctx context.Context, event cloudevents.Event) error {
		// This thunk simply wraps the main receiver in one that logs any errors
		// we encounter.
		err := receiver(ctx, event)
		if err != nil {
			log.Printf("SAW: %v", err)
		}
		return err
	}); err != nil {
		log.Fatal(err)
	}

}

func ptr(s string) *string {
	return &s
}
