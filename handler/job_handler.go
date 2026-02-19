package handler

import (
	"context"
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
	h.workerPool.Enqueue(job)
	
	c.JSON(http.StatusOK, gin.H{
		"id":     job.ID,
		"status": job.Status,
	})
}

func (h *JobHandler) GetJob(c *gin.Context) {
	id := c.Param("id")

	job, err := h.store.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) ListJobs(c *gin.Context) {
	jobs := h.store.GetAll()
	c.JSON(http.StatusOK, jobs)
}

func (h *JobHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")

	job, err := h.store.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if job.Status != model.StatusQueued && job.Status != model.StatusProcessing {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job cannot be cancelled"})
		return
	}

	job.CancelFunc()
	h.store.UpdateStatus(job, model.StatusCancelled)

	c.JSON(http.StatusOK, gin.H{
		"id":     job.ID,
		"status": job.Status,
	})
}
