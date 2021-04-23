package gcp

import (
	"log"
	"sync"

	"google.golang.org/api/cloudresourcemanager/v1"
)

// Account represents a Google Cloud Platform account
type Account struct {
	mu sync.Mutex

	// Projects list that's account across Collectors
	Projects []*cloudresourcemanager.Project
}

// NewAccount creates a new Account
func NewAccount() *Account {
	projects := []*cloudresourcemanager.Project{}
	return &Account{
		Projects: projects,
	}
}

// Update is method that transactionally updates the list of GCP projects
func (x *Account) Update(projects []*cloudresourcemanager.Project) {
	log.Printf("[Update] replacing projects")
	x.mu.Lock()
	x.Projects = projects
	x.mu.Unlock()
}
