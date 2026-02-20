package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"job-handler/model"
	"job-handler/store"
	"job-handler/worker"
)

type JobHandler struct {
	store *store.JobStore
	workerPool *worker.WorkerPool
}

func NewJobHandler(store *store.JobStore, workerPool *worker.WorkerPool) *JobHandler {
	return &JobHandler{
		store:      store,
		workerPool: workerPool,
	}
}


type CreateJobRequest struct {
	Payload string `json:"payload" binding:"required"`
}

func (h *JobHandler) CreateJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler] CreateJob: json bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	job := &model.Job{
		ID:         uuid.NewString(),
		Payload:    req.Payload,
		Status:     model.StatusQueued,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Ctx:        ctx,
		CancelFunc: cancel,
	}

	h.store.Add(job)
	log.Printf("[Handler] CreateJob: job_id=%s added to store, payload_len=%d", job.ID, len(job.Payload))
	
	h.workerPool.Enqueue(job)
	log.Printf("[Handler] CreateJob: job_id=%s enqueued to worker pool", job.ID)
	
	c.JSON(http.StatusOK, gin.H{
		"id":     job.ID,
		"status": job.Status,
	})
}

func (h *JobHandler) GetJob(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[Handler] GetJob: job_id=%s", id)

	job, err := h.store.Get(id)
	if err != nil {
		log.Printf("[Handler] GetJob: job_id=%s not found", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	log.Printf("[Handler] GetJob: job_id=%s status=%s", id, job.Status)
	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) ListJobs(c *gin.Context) {
	jobs := h.store.GetAll()
	log.Printf("[Handler] ListJobs: returning %d jobs", len(jobs))
	c.JSON(http.StatusOK, jobs)
}

func (h *JobHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")
	log.Printf("[Handler] CancelJob: job_id=%s", id)

	job, err := h.store.Get(id)
	if err != nil {
		log.Printf("[Handler] CancelJob: job_id=%s not found", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if job.Status != model.StatusQueued && job.Status != model.StatusProcessing {
		log.Printf("[Handler] CancelJob: job_id=%s cannot cancel (status=%s)", id, job.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "job cannot be cancelled"})
		return
	}

	log.Printf("[Handler] CancelJob: triggering cancel for job_id=%s (current status=%s)", id, job.Status)
	job.CancelFunc()
	h.store.UpdateStatus(job, model.StatusCancelled)
	log.Printf("[Handler] CancelJob: job_id=%s marked as cancelled", id)

	c.JSON(http.StatusOK, gin.H{
		"id":     job.ID,
		"status": job.Status,
	})
}
