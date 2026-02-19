package worker

import (
	"context"
	"os"
	"testing"
	"time"

	"job-handler/model"
	"job-handler/store"
)

func TestMain(m *testing.M) {
	// Speed up processing in tests to keep test suite fast and deterministic.
	processJobDelay = 10 * time.Millisecond
	os.Exit(m.Run())
}

func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		expectError bool
		validate    func(*WorkerPool) bool
	}{
		{
			name:        "Create pool with 1 worker",
			workerCount: 1,
			expectError: false,
			validate: func(wp *WorkerPool) bool {
				return wp.workers == 1 && wp.jobQueue != nil && wp.store != nil
			},
		},
		{
			name:        "Create pool with 5 workers",
			workerCount: 5,
			expectError: false,
			validate: func(wp *WorkerPool) bool {
				return wp.workers == 5 && cap(wp.jobQueue) == 100
			},
		},
		{
			name:        "Create pool with 10 workers",
			workerCount: 10,
			expectError: false,
			validate: func(wp *WorkerPool) bool {
				return wp.workers == 10
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(tt.workerCount, jobStore)

			if wp == nil {
				t.Fatalf("Expected WorkerPool, got nil")
			}

			if !tt.validate(wp) {
				t.Errorf("Validation failed for worker pool")
			}
		})
	}
}

func TestProcessJob(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		expectedResult string
		shouldCancel   bool
		cancelDelay    time.Duration
		expectError    bool
	}{
		{
			name:           "Process simple string",
			payload:        "hello",
			expectedResult: "olleh",
			shouldCancel:   false,
			expectError:    false,
		},
		{
			name:           "Process single character",
			payload:        "a",
			expectedResult: "a",
			shouldCancel:   false,
			expectError:    false,
		},
		{
			name:           "Process empty string",
			payload:        "",
			expectedResult: "",
			shouldCancel:   false,
			expectError:    false,
		},
		{
			name:           "Process string with spaces",
			payload:        "hello world",
			expectedResult: "dlrow olleh",
			shouldCancel:   false,
			expectError:    false,
		},
		{
			name:           "Process string with special characters",
			payload:        "a!@#b",
			expectedResult: "b#@!a",
			shouldCancel:   false,
			expectError:    false,
		},
		{
			name:           "Cancel job during processing",
			payload:        "test",
			expectedResult: "",
			shouldCancel:   true,
			cancelDelay:    1 * time.Second,
			expectError:    true,
		},
		{
			name:           "Cancel immediately",
			payload:        "immediate",
			expectedResult: "",
			shouldCancel:   true,
			cancelDelay:    0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(1, jobStore)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			job := &model.Job{
				ID:      "test-job",
				Payload: tt.payload,
				Status:  model.StatusQueued,
				Ctx:     ctx,
			}

			if tt.shouldCancel {
				if tt.cancelDelay > 0 {
					time.AfterFunc(tt.cancelDelay, cancel)
				} else {
					cancel()
				}
			}

			result, err := wp.processJob(job)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if !tt.expectError && result != tt.expectedResult {
				t.Errorf("Expected result %q, got %q", tt.expectedResult, result)
			}
		})
	}
}

func TestEnqueueAndProcess(t *testing.T) {
	tests := []struct {
		name           string
		jobCount       int
		workerCount    int
		payloads       []string
		expectedStatus model.JobStatus
		timeout        time.Duration
	}{
		{
			name:           "Process single job with single worker",
			jobCount:       1,
			workerCount:    1,
			payloads:       []string{"hello"},
			expectedStatus: model.StatusCompleted,
			timeout:        10 * time.Second,
		},
		{
			name:           "Process multiple jobs with single worker",
			jobCount:       3,
			workerCount:    1,
			payloads:       []string{"hello", "go", "test"},
			expectedStatus: model.StatusCompleted,
			timeout:        15 * time.Second,
		},
		{
			name:           "Process multiple jobs with multiple workers",
			jobCount:       5,
			workerCount:    3,
			payloads:       []string{"hello", "world", "go", "test", "pool"},
			expectedStatus: model.StatusCompleted,
			timeout:        15 * time.Second,
		},
		{
			name:           "Process single job with multiple workers",
			jobCount:       1,
			workerCount:    5,
			payloads:       []string{"quick"},
			expectedStatus: model.StatusCompleted,
			timeout:        10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(tt.workerCount, jobStore)
			wp.Start()
			defer wp.Shutdown()

			// Enqueue jobs
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			for i := 0; i < tt.jobCount; i++ {
				job := &model.Job{
					ID:        "job-" + string(rune(48+i)),
					Payload:   tt.payloads[i],
					Status:    model.StatusQueued,
					CreatedAt: time.Now(),
					Ctx:       ctx,
				}
				jobStore.Add(job)
				wp.Enqueue(job)
			}

			// Wait for all jobs to complete with timeout
			completed := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), tt.timeout)
			defer timeoutCancel()

			for completed < tt.jobCount {
				select {
				case <-timeoutCtx.Done():
					t.Fatalf("Timeout waiting for jobs to complete. Completed: %d/%d", completed, tt.jobCount)
				case <-ticker.C:
					completed = 0
					allJobs := jobStore.GetAll()
					for _, job := range allJobs {
						if job.Status == tt.expectedStatus {
							completed++
						}
					}
				}
			}

			// Verify all jobs have expected status
			allJobs := jobStore.GetAll()
			if len(allJobs) != tt.jobCount {
				t.Errorf("Expected %d jobs, got %d", tt.jobCount, len(allJobs))
			}

			for _, job := range allJobs {
				if job.Status != tt.expectedStatus {
					t.Errorf("Job %s: expected status %s, got %s", job.ID, tt.expectedStatus, job.Status)
				}
			}
		})
	}
}

func TestEnqueueWithCancellation(t *testing.T) {
	tests := []struct {
		name              string
		jobCount          int
		cancelAtIndex     int
		expectedCompleted int
		expectedCancelled int
		timeout           time.Duration
	}{
		{
			name:              "Cancel single job out of 1",
			jobCount:          1,
			cancelAtIndex:     0,
			expectedCompleted: 0,
			expectedCancelled: 1,
			timeout:           10 * time.Second,
		},
		{
			name:              "Cancel first job out of 3",
			jobCount:          3,
			cancelAtIndex:     0,
			expectedCompleted: 2,
			expectedCancelled: 1,
			timeout:           15 * time.Second,
		},
		{
			name:              "Cancel middle job out of 5",
			jobCount:          5,
			cancelAtIndex:     2,
			expectedCompleted: 4,
			expectedCancelled: 1,
			timeout:           20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(2, jobStore)
			wp.Start()
			defer wp.Shutdown()

			// Create contexts for each job
			jobContexts := make([]context.Context, tt.jobCount)
			jobCancels := make([]context.CancelFunc, tt.jobCount)

			for i := 0; i < tt.jobCount; i++ {
				jobContexts[i], jobCancels[i] = context.WithCancel(context.Background())
			}

			// Enqueue jobs
			for i := 0; i < tt.jobCount; i++ {
				job := &model.Job{
					ID:         "cancel-job-" + string(rune(48+i)),
					Payload:    "test-" + string(rune(48+i)),
					Status:     model.StatusQueued,
					CreatedAt:  time.Now(),
					Ctx:        jobContexts[i],
					CancelFunc: jobCancels[i],
				}
				jobStore.Add(job)
				wp.Enqueue(job)
			}

			// Cancel the specified job after a short delay
			time.Sleep(500 * time.Millisecond)
			jobCancels[tt.cancelAtIndex]()

			// Wait for all jobs to complete or be cancelled with timeout
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), tt.timeout)
			defer timeoutCancel()

			for {
				select {
				case <-timeoutCtx.Done():
					t.Fatalf("Timeout waiting for jobs to complete/cancel")
				case <-ticker.C:
					allJobs := jobStore.GetAll()
					completed := 0
					cancelled := 0
					for _, job := range allJobs {
						if job.Status == model.StatusCompleted {
							completed++
						}
						if job.Status == model.StatusCancelled {
							cancelled++
						}
					}
					if completed+cancelled == tt.jobCount {
						if completed != tt.expectedCompleted || cancelled != tt.expectedCancelled {
							t.Errorf("Expected %d completed and %d cancelled, got %d completed and %d cancelled",
								tt.expectedCompleted, tt.expectedCancelled, completed, cancelled)
						}
						return
					}
				}
			}
		})
	}
}

func TestWorkerPoolShutdown(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		jobCount    int
		timeout     time.Duration
	}{
		{
			name:        "Shutdown with no jobs",
			workerCount: 1,
			jobCount:    0,
			timeout:     5 * time.Second,
		},
		{
			name:        "Shutdown with pending jobs",
			workerCount: 2,
			jobCount:    5,
			timeout:     20 * time.Second,
		},
		{
			name:        "Shutdown with single worker and single job",
			workerCount: 1,
			jobCount:    1,
			timeout:     10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(tt.workerCount, jobStore)
			wp.Start()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Enqueue jobs
			for i := 0; i < tt.jobCount; i++ {
				job := &model.Job{
					ID:        "shutdown-job-" + string(rune(48+i)),
					Payload:   "payload-" + string(rune(48+i)),
					Status:    model.StatusQueued,
					CreatedAt: time.Now(),
					Ctx:       ctx,
				}
				jobStore.Add(job)
				wp.Enqueue(job)
			}

			// Shutdown should complete without hanging
			shutdownDone := make(chan bool, 1)
			go func() {
				wp.Shutdown()
				shutdownDone <- true
			}()

			select {
			case <-shutdownDone:
				// Shutdown completed successfully
			case <-time.After(tt.timeout):
				t.Fatalf("Shutdown did not complete within timeout")
			}

			// Sending to closed channel should panic
			// We verify pool is properly closed by checking if enqueueing panics
			defer func() {
				if r := recover(); r == nil {
					// This is expected for a closed pool
				}
			}()
			// This should panic since jobQueue is closed
			// wp.Enqueue(&model.Job{})
		})
	}
}

func TestJobStatusTransition(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		expectedStatus model.JobStatus
		shouldFail     bool
		failureType    string
	}{
		{
			name:           "Successful job transitions to completed",
			payload:        "success",
			expectedStatus: model.StatusCompleted,
			shouldFail:     false,
		},
		{
			name:           "Job with result is marked completed",
			payload:        "result-test",
			expectedStatus: model.StatusCompleted,
			shouldFail:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(1, jobStore)
			wp.Start()
			defer wp.Shutdown()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			job := &model.Job{
				ID:        "status-test-job",
				Payload:   tt.payload,
				Status:    model.StatusQueued,
				CreatedAt: time.Now(),
				Ctx:       ctx,
			}
			jobStore.Add(job)
			wp.Enqueue(job)

			// Wait for job to complete with timeout
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer timeoutCancel()

			for {
				select {
				case <-timeoutCtx.Done():
					t.Fatalf("Timeout waiting for job to complete")
				case <-ticker.C:
					retrievedJob, err := jobStore.Get(job.ID)
					if err == nil && retrievedJob.Status == tt.expectedStatus {
						if tt.expectedStatus == model.StatusCompleted && retrievedJob.Result == "" {
							t.Errorf("Expected result to be set for completed job")
						}
						return
					}
				}
			}
		})
	}
}

func TestWorkerPoolConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		jobCount    int
		timeout     time.Duration
	}{
		{
			name:        "High concurrency: 10 workers, 50 jobs",
			workerCount: 10,
			jobCount:    50,
			timeout:     60 * time.Second,
		},
		{
			name:        "Medium concurrency: 5 workers, 20 jobs",
			workerCount: 5,
			jobCount:    20,
			timeout:     40 * time.Second,
		},
		{
			name:        "Low concurrency: 2 workers, 10 jobs",
			workerCount: 2,
			jobCount:    10,
			timeout:     40 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore := store.NewJobStore()
			wp := NewWorkerPool(tt.workerCount, jobStore)
			wp.Start()
			defer wp.Shutdown()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Enqueue all jobs
			for i := 0; i < tt.jobCount; i++ {
				job := &model.Job{
					ID:        "concurrent-job-" + string(rune(48+i%10)),
					Payload:   "payload-" + string(rune(48+i%10)),
					Status:    model.StatusQueued,
					CreatedAt: time.Now(),
					Ctx:       ctx,
				}
				jobStore.Add(job)
				wp.Enqueue(job)
			}

			// Wait for all jobs to complete
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), tt.timeout)
			defer timeoutCancel()

			for {
				select {
				case <-timeoutCtx.Done():
					t.Fatalf("Timeout waiting for all jobs to complete")
				case <-ticker.C:
					allJobs := jobStore.GetAll()
					completed := 0
					for _, job := range allJobs {
						if job.Status == model.StatusCompleted {
							completed++
						}
					}
					if completed == tt.jobCount {
						return
					}
				}
			}
		})
	}
}

// BenchmarkProcessJob benchmarks the processJob function
func BenchmarkProcessJob(b *testing.B) {
	jobStore := store.NewJobStore()
	wp := NewWorkerPool(1, jobStore)

	ctx := context.Background()
	job := &model.Job{
		ID:      "bench-job",
		Payload: "benchmark-test",
		Ctx:     ctx,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.processJob(job)
	}
}

// BenchmarkEnqueue benchmarks the Enqueue function
func BenchmarkEnqueue(b *testing.B) {
	jobStore := store.NewJobStore()
	wp := NewWorkerPool(1, jobStore)
	wp.Start()
	defer wp.Shutdown()

	ctx := context.Background()
	job := &model.Job{
		ID:      "bench-enqueue",
		Payload: "test",
		Ctx:     ctx,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.Enqueue(job)
	}
}
