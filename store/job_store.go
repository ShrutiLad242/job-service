package store

import (
	"errors"
	"log"
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
	log.Printf("[Store] Added job_id=%s, total_jobs=%d", job.ID, len(s.jobs))
}

func (s *JobStore) Get(id string) (*model.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[id]
	if !exists {
		log.Printf("[Store] Get job_id=%s: not found", id)
		return nil, errors.New("job not found")
	}
	log.Printf("[Store] Get job_id=%s: found, status=%s", id, job.Status)
	return job, nil
}

func (s *JobStore) GetAll() []*model.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	log.Printf("[Store] GetAll: returning %d jobs", len(jobs))
	return jobs
}

func (s *JobStore) UpdateStatus(job *model.Job, status model.JobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldStatus := job.Status
	job.Status = status
	job.UpdatedAt = time.Now()
	log.Printf("[Store] UpdateStatus job_id=%s: %s -> %s", job.ID, oldStatus, status)
}
