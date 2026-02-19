package store

import (
	"errors"
	"sync"
	"time"

	"job-handler/model"
)

type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*model.Job
}

func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*model.Job),
	}
}

func (s *JobStore) Add(job *model.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *JobStore) Get(id string) (*model.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (s *JobStore) GetAll() []*model.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (s *JobStore) UpdateStatus(job *model.Job, status model.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job.Status = status
	job.UpdatedAt = time.Now()
}
