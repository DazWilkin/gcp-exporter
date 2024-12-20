package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/DazWilkin/gcp-exporter/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
)

// IAMCollector represents Identity and Access Management (IAM)
type IAMCollector struct {
	account *gcp.Account

	Up                 *prometheus.Desc
	ServiceAccounts    *prometheus.Desc
	ServiceAccountKeys *prometheus.Desc
}

// NewIAMCollector creates a new IAMCollector
func NewIAMCollector(account *gcp.Account) *IAMCollector {
	fqName := name("iam")

	return &IAMCollector{
		account: account,

		Up: prometheus.NewDesc(
			fqName("up"),
			"1 if the IAM service is up, 0 otherwise",
			nil,
			nil,
		),
		ServiceAccounts: prometheus.NewDesc(
			fqName("service_accounts"),
			"Number of Service Accounts",
			[]string{
				"project",
				"name",
				"disabled",
			},
			nil,
		),
		ServiceAccountKeys: prometheus.NewDesc(
			fqName("service_account_keys"),
			"Number of Service Account Keys",
			[]string{
				"project",
				"service_account_email",
				"key",
				"type",
				"disabled",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *IAMCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	iamService, err := iam.NewService(ctx)
	if err != nil {
		log.Println(err)
	}

	// Enumerate all of the projects
	var wg sync.WaitGroup
	for _, p := range c.account.Projects {
		wg.Add(1)
		go func(p *cloudresourcemanager.Project) {
			defer wg.Done()
			log.Printf("IAMCollector:go] Project: %s", p.ProjectId)
			parent := fmt.Sprintf("projects/%s", p.ProjectId)
			resp, err := iamService.Projects.ServiceAccounts.List(parent).Context(ctx).Do()
			if err != nil {
				if e, ok := err.(*googleapi.Error); ok {
					if e.Code == http.StatusForbidden {
						// Probably (!) IAM API has not been enabled for Project (p)
						return
					}

					log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
					return
				}

				log.Println(err)
				return
			}

			for _, account := range resp.Accounts {
				log.Printf("IAMCollector:go] ServiceAccount: %s", account.Name)

				// Record Service Account metrics
				ch <- prometheus.MustNewConstMetric(
					c.ServiceAccounts,
					prometheus.GaugeValue,
					1.0,
					[]string{
						p.ProjectId,
						account.Email,
						fmt.Sprintf("%t", account.Disabled),
					}...,
				)

				// Service Account Keys within Service Account
				name := fmt.Sprintf("projects/%s/serviceAccounts/%s", p.ProjectId, account.UniqueId)
				resp, err := iamService.Projects.ServiceAccounts.Keys.List(name).Context(ctx).Do()
				if err != nil {
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == http.StatusForbidden {
							// Probably (!) IAM API has not been enabled for Project (p)
							return
						}

						log.Printf("Google API Error: %d [%s]", e.Code, e.Message)
						return
					}

					log.Println(err)
					return
				}

				for _, key := range resp.Keys {
					log.Printf("[IAMCollector:go] ServiceAccountKey: %s", key.Name)

					// Name = projects/{PROJECT_ID}/serviceAccounts/{ACCOUNT}/keys/{key}
					keyID, err := func(name string) (string, error) {
						if name == "" {
							return "", fmt.Errorf("name is empty")
						}

						parts := strings.Split(name, "/")
						if len(parts) != 6 {
							return "", fmt.Errorf("expected 6 parts, got %d (%s)", len(parts), parts)
						}

						// Return the last part (key)
						key := parts[len(parts)-1]
						return key, nil
					}(key.Name)
					if err != nil {
						log.Printf("unable to extract {key} from %s", key.Name)
						continue
					}

					// Record Service Account Key metrics
					ch <- prometheus.MustNewConstMetric(
						c.ServiceAccountKeys,
						prometheus.GaugeValue,
						1.0,
						[]string{
							p.ProjectId,
							account.Email,
							keyID,
							key.KeyType,
							fmt.Sprintf("%t", key.Disabled),
						}...,
					)
				}
			}
		}(p)
	}
	wg.Wait()
}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *IAMCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Up
	ch <- c.ServiceAccounts
	ch <- c.ServiceAccountKeys
}
