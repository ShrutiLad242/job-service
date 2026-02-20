package worker

import (
	"context"
	"errors"
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
	log.Printf("[WorkerPool] Started %d workers", wp.workers)
}

func (wp *WorkerPool) Enqueue(job *model.Job) {
	log.Printf("[JobQueue] Enqueued job_id=%s payload_len=%d", job.ID, len(job.Payload))
	wp.jobQueue <- job
}

func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()

	log.Printf("[Worker %d] Started and ready to process jobs", workerID)

	for job := range wp.jobQueue {
		log.Printf("[Worker %d] Received job_id=%s", workerID, job.ID)

		select {
		case <-job.Ctx.Done():
			log.Printf("[Worker %d] Job %s context already cancelled, marking as cancelled", workerID, job.ID)
			wp.store.UpdateStatus(job, model.StatusCancelled)
			continue
		default:
		}

		wp.store.UpdateStatus(job, model.StatusProcessing)
		log.Printf("[Worker %d] Job %s marked processing", workerID, job.ID)

		result, err := wp.processJob(job)

		if err != nil {
			job.Error = err.Error()
			log.Printf("[Worker %d] Job %s processing error: %v", workerID, job.ID, err)
			// Treat context cancellations as cancelled, not failed.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				log.Printf("[Worker %d] Job %s cancelled due to context", workerID, job.ID)
				wp.store.UpdateStatus(job, model.StatusCancelled)
			} else {
				log.Printf("[Worker %d] Job %s marked failed", workerID, job.ID)
				wp.store.UpdateStatus(job, model.StatusFailed)
			}
			continue
		}

		job.Result = result
		log.Printf("[Worker %d] Job %s processing completed, result_len=%d", workerID, job.ID, len(result))
		wp.store.UpdateStatus(job, model.StatusCompleted)
	}
	log.Printf("[Worker %d] Stopped (channel closed)", workerID)
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
	log.Printf("[WorkerPool] Shutdown initiated, closing job queue")
	close(wp.jobQueue) // stop workers
	log.Printf("[WorkerPool] Waiting for all workers to finish")
	wp.wg.Wait() // wait for all workers to finish
	log.Printf("[WorkerPool] All workers finished, pool shutdown complete")
}
