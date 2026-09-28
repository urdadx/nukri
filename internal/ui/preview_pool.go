package ui

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
	"github.com/urdadx/nukri/internal/preview"
	"github.com/urdadx/nukri/internal/ui/browser"
)

// previewRequest describes the work needed to build a single preview pane.
type previewRequest struct {
	entry      browser.Entry
	width      int
	syntax     string
	codeWindow int
}

/*
previewJob is a unit of preview work handed to a pool worker. Each submitter
attaches its own result channel; the worker fans the single result out to
every subscriber, so concurrent identical requests all observe a value.
*/
type previewJob struct {
	key     string
	req     previewRequest
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	subs    []chan tea.Msg
	result  tea.Msg
	closed  bool
	stale   atomic.Bool
	started atomic.Bool
	high    atomic.Bool
}

/*
PreviewPool runs preview rendering on a fixed set of worker goroutines so the
UI event loop is never blocked by file decoding, PDF rendering or syntax
highlighting. It mirrors the way file managers offload preview work to a
background thread pool:

  - a shared set of workers consumes both a high-priority queue (the currently
    selected entry) and a low-priority queue (prefetched neighbors);
  - requests are de-duplicated by key and stale in-flight renders are cancelled
    as soon as a newer selection takes precedence;
  - completed renders are cached by key so revisiting a file is instant.
*/
type PreviewPool struct {
	svc  *preview.Service
	high chan *previewJob
	low  chan *previewJob
	/*
		mu guards the active map, which tracks in-flight jobs by key. It is not held
		while a worker is executing a job, so the map is only used to de-duplicate
		and cancel work, not to store results.
	*/
	mu     sync.Mutex
	active map[string]*previewJob

	cacheMu    sync.Mutex
	cache      map[string]tea.Msg
	cacheOrder []string

	done chan struct{}
	wg   sync.WaitGroup
}

const (
	previewCacheLimit = 64
	previewQueueDepth = 128
)

// NewPreviewPool starts `workers` renderer goroutines around the given service.
func NewPreviewPool(svc *preview.Service, workers int) *PreviewPool {
	if workers < 1 {
		workers = 1
	}
	p := &PreviewPool{
		svc:    svc,
		high:   make(chan *previewJob, previewQueueDepth),
		low:    make(chan *previewJob, previewQueueDepth),
		active: make(map[string]*previewJob),
		cache:  make(map[string]tea.Msg),
		done:   make(chan struct{}),
	}
	if workers > 1 {
		p.wg.Add(1)
		go p.foregroundWorker()
		workers--
	}
	for range workers {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

// Close stops all workers. It blocks until every in-flight job has finished.
func (p *PreviewPool) Close() {
	p.mu.Lock()
	for _, job := range p.active {
		job.stop()
	}
	p.mu.Unlock()
	close(p.done)
	p.wg.Wait()
}

func (p *PreviewPool) foregroundWorker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.done:
			return
		case job := <-p.high:
			p.execute(job)
		}
	}
}

// worker is the main loop of a single preview renderer goroutine. It consumes
// jobs from the high-priority queue first, then the low-priority queue. It
// exits when the pool is closed.
func (p *PreviewPool) worker() {
	defer p.wg.Done()
	for {
		var job *previewJob
		select {
		case <-p.done:
			return
		case job = <-p.high:
		default:
			select {
			case <-p.done:
				return
			case job = <-p.high:
			case job = <-p.low:
			}
		}
		p.execute(job)
	}
}

// execute performs the actual preview work on a worker goroutine. It checks the
// cache first, then calls the service to render the preview. It stores the result
// in the cache and fans it out to all subscribers.
func (p *PreviewPool) execute(job *previewJob) {
	if !job.started.CompareAndSwap(false, true) {
		return
	}
	defer p.forget(job.key)

	if cached, ok := p.lookup(job.key); ok {
		job.complete(cached)
		return
	}
	if job.stale.Load() || job.ctx.Err() != nil {
		job.complete(cancelledPreview(job.req.entry.Entry.Path))
		return
	}

	msg := renderPreview(job.ctx, p.svc, job.req)
	if job.stale.Load() || job.ctx.Err() != nil {
		job.complete(cancelledPreview(job.req.entry.Entry.Path))
		return
	}
	p.store(job.key, msg)
	job.complete(msg)
}

/*
complete delivers the final message to every subscriber of the job and closes
each result channel. It is idempotent: a later caller that subscribed after
completion receives the stored result.
*/
func (j *previewJob) complete(msg tea.Msg) {
	j.mu.Lock()
	if j.closed {
		j.mu.Unlock()
		return
	}
	j.result = msg
	j.closed = true
	subs := j.subs
	j.subs = nil
	j.mu.Unlock()
	for _, ch := range subs {
		ch <- msg
		close(ch)
	}
}

// subscribe attaches a fresh result channel and returns it. If the job has
// already completed, the stored result is delivered immediately.
func (j *previewJob) subscribe() <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	j.mu.Lock()
	if j.closed {
		result := j.result
		j.mu.Unlock()
		ch <- result
		close(ch)
		return ch
	}
	j.subs = append(j.subs, ch)
	j.mu.Unlock()
	return ch
}

func (j *previewJob) stop() {
	j.stale.Store(true)
	j.cancel()
}

/*
Submit queues a preview render and returns a channel that yields exactly one
message when the work finishes (or is cancelled). The channel is closed
afterwards. When the result is already cached it is returned immediately.
codeWindow caps the number of leading code lines rendered (0 = whole file).
*/
func (p *PreviewPool) Submit(entry browser.Entry, width int, syntax string, codeWindow int, high bool) <-chan tea.Msg {
	key := previewKey(entry, width, syntax, codeWindow)
	if high {
		p.mu.Lock()
		for activeKey, job := range p.active {
			if activeKey != key {
				job.stop()
			}
		}
		p.mu.Unlock()
	}
	if cached, ok := p.lookup(key); ok {
		ch := make(chan tea.Msg, 1)
		ch <- cached
		close(ch)
		return ch
	}

	var (
		job *previewJob
		ch  <-chan tea.Msg
	)

	p.mu.Lock()
	if existing, ok := p.active[key]; ok {
		promote := high && !existing.high.Swap(true) && !existing.started.Load()
		ch := existing.subscribe()
		p.mu.Unlock()
		if promote {
			p.high <- existing
		}
		return ch
	}
	ctx, cancel := context.WithCancel(context.Background())
	job = &previewJob{
		key: key, ctx: ctx, cancel: cancel,
		req: previewRequest{entry: entry, width: width, syntax: syntax, codeWindow: codeWindow},
	}
	job.high.Store(high)
	p.active[key] = job
	ch = job.subscribe()
	p.mu.Unlock()

	if high {
		p.high <- job
	} else {
		p.low <- job
	}
	return ch
}

func (p *PreviewPool) forget(key string) {
	p.mu.Lock()
	delete(p.active, key)
	p.mu.Unlock()
}

func (p *PreviewPool) lookup(key string) (tea.Msg, bool) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	msg, ok := p.cache[key]
	return msg, ok
}

// store caches a preview message by its key in a simple LRU.
// It is called only by a worker after the job has completed.
func (p *PreviewPool) store(key string, msg tea.Msg) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	if _, ok := p.cache[key]; ok {
		// Refresh LRU position.
		for i, k := range p.cacheOrder {
			if k == key {
				p.cacheOrder = append(p.cacheOrder[:i], p.cacheOrder[i+1:]...)
				break
			}
		}
		p.cacheOrder = append(p.cacheOrder, key)
		return
	}
	p.cache[key] = msg
	p.cacheOrder = append(p.cacheOrder, key)
	for len(p.cacheOrder) > previewCacheLimit {
		old := p.cacheOrder[0]
		p.cacheOrder = p.cacheOrder[1:]
		delete(p.cache, old)
	}
}

/*
PreviewKey returns the cache-key string used to de-duplicate and cache a
preview request. It is exported so external packages (e.g. tests) can reason
about cache identity. Two requests share a cache entry only when the path, on
disk size, modification time, preview width, highlight syntax and code window
all match.
*/
func PreviewKey(entry browser.Entry, width int, syntax string, codeWindow int) string {
	return previewKey(entry, width, syntax, codeWindow)
}

func previewKey(entry browser.Entry, width int, syntax string, codeWindow int) string {
	e := entry.Entry
	modified := int64(0)
	if !e.Modified.IsZero() {
		modified = e.Modified.UnixNano()
	}
	return fmt.Sprintf("%s|%d|%d|%d|%s|%d", e.Path, e.Size, modified, width, syntax, codeWindow)
}

func cancelledPreview(path string) previewMsg {
	return previewMsg{path: path, err: nil}
}

// renderPreview performs the actual service.Render + BuildView work. It runs on
// a worker goroutine, never on the UI event loop.
func renderPreview(ctx context.Context, service *preview.Service, req previewRequest) previewMsg {
	req.entry.Facts = fileinfo.InspectEntry(&req.entry.Entry)
	if err := ctx.Err(); err != nil {
		return cancelledPreview(req.entry.Entry.Path)
	}
	value, err := service.Render(ctx, preview.Request{Path: req.entry.Entry.Path, Facts: req.entry.Facts, Width: max(1, req.width)})
	if err != nil {
		if errors.Is(err, preview.ErrUnsupported) {
			err = fmt.Errorf("preview is not available for this file type")
		} else {
			var toolErr *preview.ToolUnavailableError
			if errors.As(err, &toolErr) {
				err = fmt.Errorf("install %s to preview this file", toolErr.Tool)
			}
		}
		return previewMsg{path: req.entry.Entry.Path, err: err}
	}
	msg := previewMsg{path: req.entry.Entry.Path}
	if directory, ok := value.(*preview.DirectoryPreview); ok {
		msg.directory = true
		msg.entries = directoryPreviewEntries(req.entry.Entry.Path, directory.Entries)
	}
	view, viewErr := preview.BuildViewContext(ctx, value, preview.ViewOptions{
		Width:       max(1, req.width),
		SyntaxStyle: req.syntax,
		CodeWindow:  req.codeWindow,
	})
	msg.view, msg.err = view, viewErr
	return msg
}
