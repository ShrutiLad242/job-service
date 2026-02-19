package worker

import (
	"log"
	"sync"
	"time"

	"job-handler/model"
	"job-handler/store"
)

// processJobDelay controls the simulated processing duration in tests.
// It defaults to 3 seconds but tests can override it.
var processJobDelay = 3 * time.Second

type WorkerPool struct {
	jobQueue chan *model.Job
	store    *store.JobStore
	workers  int
	wg       sync.WaitGroup
}

func NewWorkerPool(workerCount int, store *store.JobStore) *WorkerPool {
	return &WorkerPool{
		jobQueue: make(chan *model.Job, 100), //buffered channel for 100 jobs
		store:    store,
		workers:  workerCount,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) Enqueue(job *model.Job) {
	wp.jobQueue <- job
}

func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()

	log.Printf("Worker %d started\n", workerID)

	for job := range wp.jobQueue {

		select {
		case <-job.Ctx.Done():
			wp.store.UpdateStatus(job, model.StatusCancelled)
			continue
		default:
		}

		wp.store.UpdateStatus(job, model.StatusProcessing)

		result, err := wp.processJob(job)

		if err != nil {
			job.Error = err.Error()
			wp.store.UpdateStatus(job, model.StatusFailed)
			continue
		}

		job.Result = result
		wp.store.UpdateStatus(job, model.StatusCompleted)
	}
}

func (wp *WorkerPool) processJob(job *model.Job) (string, error) {

	// Simulate processing using configurable delay
	select {
	case <-time.After(processJobDelay):

		// Example processing: reverse string
		runes := []rune(job.Payload)

		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}

		return string(runes), nil

	case <-job.Ctx.Done():
		return "", job.Ctx.Err()
	}
}

func (wp *WorkerPool) Shutdown() {
	close(wp.jobQueue) // stop workers
	wp.wg.Wait()       // wait for all workers to finish
}
