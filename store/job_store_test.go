package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"job-handler/model"
)

func TestJobStore_AddAndGet(t *testing.T) {
	store := NewJobStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := &model.Job{
		ID:         "test-id",
		Payload:    "hello",
		Status:     model.StatusQueued,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Ctx:        ctx,
		CancelFunc: cancel,
	}

	store.Add(job)

	result, err := store.Get("test-id")

	if err != nil {
		t.Fatalf("expected job, got error %v", err)
	}

	if result.ID != "test-id" {
		t.Fatalf("expected id test-id, got %s", result.ID)
	}
}

func TestJobStore_UpdateStatus(t *testing.T) {
	store := NewJobStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := &model.Job{
		ID:         "test-id",
		Status:     model.StatusQueued,
		Ctx:        ctx,
		CancelFunc: cancel,
	}

	store.Add(job)

	store.UpdateStatus(job, model.StatusProcessing)

	if job.Status != model.StatusProcessing {
		t.Fatalf("expected processing, got %s", job.Status)
	}
}

func TestConcurrentAddGetUpdate(t *testing.T) {
	s := NewJobStore()
	const numJobs = 200

	// Create jobs concurrently
	var wg sync.WaitGroup
	wg.Add(numJobs)

	for i := 0; i < numJobs; i++ {
		go func(i int) {
			defer wg.Done()
			job := &model.Job{
				ID:        "job-" + string(rune(48+i%10)) + "-" + string(rune(i/10)),
				Payload:   "payload",
				Status:    model.StatusQueued,
				CreatedAt: time.Now(),
			}
			s.Add(job)
		}(i)
	}

	// Concurrently perform Gets and Updates
	const readers = 50
	wg.Wait() // wait until adds have been started

	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// iterate over snapshot
				all := s.GetAll()
				for _, job := range all {
					// Update status for some jobs
					s.UpdateStatus(job, model.StatusProcessing)
					// Read back
					if _, err := s.Get(job.ID); err != nil {
						t.Fatalf("expected job to exist: %v", err)
					}
				}
				// small sleep to yield
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	all := s.GetAll()
	if len(all) != numJobs {
		t.Fatalf("expected %d jobs, got %d", numJobs, len(all))
	}
}

func TestGetAllConcurrent(t *testing.T) {
	s := NewJobStore()
	const numJobs = 500

	var wg sync.WaitGroup
	wg.Add(numJobs)
	for i := 0; i < numJobs; i++ {
		go func(i int) {
			defer wg.Done()
			j := &model.Job{
				ID:        "gaj-" + string(rune(48+i%10)) + "-" + string(rune(i/10)),
				Payload:   "p",
				Status:    model.StatusQueued,
				CreatedAt: time.Now(),
			}
			s.Add(j)
		}(i)
	}

	// While adding, call GetAll concurrently
	var rwg sync.WaitGroup
	rwg.Add(1)
	go func() {
		defer rwg.Done()
		for i := 0; i < 100; i++ {
			_ = s.GetAll()
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()
	rwg.Wait()

	all := s.GetAll()
	if len(all) != numJobs {
		t.Fatalf("expected %d jobs after concurrent add, got %d", numJobs, len(all))
	}
}

func TestUpdateStatusConcurrent(t *testing.T) {
	s := NewJobStore()
	job := &model.Job{
		ID:        "up-1",
		Payload:   "p",
		Status:    model.StatusQueued,
		CreatedAt: time.Now(),
	}
	s.Add(job)

	var wg sync.WaitGroup
	workers := 100
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			// cycle through statuses
			statuses := []model.JobStatus{model.StatusProcessing, model.StatusFailed, model.StatusCompleted}
			for j := 0; j < 50; j++ {
				s.UpdateStatus(job, statuses[(i+j)%len(statuses)])
				// small read
				if got, err := s.Get(job.ID); err != nil || got == nil {
					t.Fatalf("Get failed during concurrent updates: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	got, err := s.Get(job.ID)
	if err != nil {
		t.Fatalf("expected job to exist: %v", err)
	}
	// final status should be one of allowed statuses
	switch got.Status {
	case model.StatusProcessing, model.StatusFailed, model.StatusCompleted:
		// ok
	default:
		t.Fatalf("unexpected final status: %v", got.Status)
	}
}
