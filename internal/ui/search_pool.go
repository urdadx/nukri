package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const searchBatchSize = 512

type searchCandidate struct {
	path, name, nameKey, relative, relativeKey string
	directory                                  bool
}

type searchEvent struct {
	token      uint64
	candidates []searchCandidate
	scanned    int
	done       bool
	err        error
}

type searchRequest struct {
	token uint64
	root  string
}

type searchJob struct {
	request searchRequest
	results chan searchEvent
	cancel  atomic.Bool
	stopped chan struct{}
	stop    sync.Once
}

// SearchPool owns one filesystem worker. New requests replace pending work and
// cancel the active traversal so recursive scans never compete for disk I/O.
type SearchPool struct {
	mu      sync.Mutex
	ready   *sync.Cond
	pending *searchJob
	active  *searchJob
}

func NewSearchPool() *SearchPool {
	p := &SearchPool{}
	p.ready = sync.NewCond(&p.mu)
	go p.worker()
	return p
}

func (p *SearchPool) Submit(request searchRequest) <-chan searchEvent {
	job := &searchJob{request: request, results: make(chan searchEvent, 8), stopped: make(chan struct{})}
	p.mu.Lock()
	if p.pending != nil {
		p.pending.cancelJob()
		close(p.pending.results)
	}
	if p.active != nil {
		p.active.cancelJob()
	}
	p.pending = job
	p.ready.Signal()
	p.mu.Unlock()
	return job.results
}

func (p *SearchPool) Cancel() {
	p.mu.Lock()
	if p.pending != nil {
		p.pending.cancelJob()
		close(p.pending.results)
		p.pending = nil
	}
	if p.active != nil {
		p.active.cancelJob()
	}
	p.mu.Unlock()
}

func (p *SearchPool) worker() {
	for {
		p.mu.Lock()
		for p.pending == nil {
			p.ready.Wait()
		}
		job := p.pending
		p.pending = nil
		p.active = job
		p.mu.Unlock()

		p.scan(job)
		close(job.results)

		p.mu.Lock()
		if p.active == job {
			p.active = nil
		}
		p.mu.Unlock()
	}
}

func (p *SearchPool) scan(job *searchJob) {
	queue := []string{job.request.root}
	batch := make([]searchCandidate, 0, searchBatchSize)
	scanned := 0
	for len(queue) > 0 {
		if job.cancel.Load() {
			return
		}
		directory := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(directory)
		if err != nil {
			if directory == job.request.root {
				job.send(searchEvent{token: job.request.token, scanned: scanned, done: true, err: err})
				return
			}
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})
		for _, entry := range entries {
			if job.cancel.Load() {
				return
			}
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			scanned++
			path := filepath.Join(directory, entry.Name())
			relative, err := filepath.Rel(job.request.root, path)
			if err != nil {
				continue
			}
			candidate := searchCandidate{
				path: path, name: entry.Name(), nameKey: strings.ToLower(entry.Name()),
				relative: relative, relativeKey: strings.ToLower(relative), directory: entry.IsDir(),
			}
			batch = append(batch, candidate)
			if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
				queue = append(queue, path)
			}
			if len(batch) == searchBatchSize {
				if !job.send(searchEvent{token: job.request.token, candidates: batch, scanned: scanned}) {
					return
				}
				batch = make([]searchCandidate, 0, searchBatchSize)
			}
		}
	}
	job.send(searchEvent{token: job.request.token, candidates: batch, scanned: scanned, done: true})
}

func (j *searchJob) send(event searchEvent) bool {
	if j.cancel.Load() {
		return false
	}
	select {
	case j.results <- event:
		return !j.cancel.Load()
	case <-j.stopped:
		return false
	}
}

func (j *searchJob) cancelJob() {
	j.cancel.Store(true)
	j.stop.Do(func() { close(j.stopped) })
}
