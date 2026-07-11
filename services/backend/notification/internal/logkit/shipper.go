package logkit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ingestBody is the request payload for POST /api/ingest (shared contract).
type ingestBody struct {
	Records []Record `json:"records"`
}

// shipper owns asynchronous, resilient delivery of Records to the log-server.
//
// A single event-loop goroutine owns the batch buffer and ALL spool-file I/O,
// so no locks are needed for those; the only cross-goroutine surface is the
// bounded queue (enqueue) and the done/flushReq channels. Callers of enqueue
// never block: a full queue drops (and counts) the record.
type shipper struct {
	opts      Options
	endpoint  string
	spoolPath string

	queue    chan Record
	flushReq chan chan struct{}
	done     chan struct{}
	stopped  chan struct{}

	closeOnce sync.Once
	wg        sync.WaitGroup
	dropped   atomic.Int64
}

func newShipper(serviceName string, opts Options) *shipper {
	// Best-effort: if the spool dir can't be created, spooling simply fails and
	// records are dropped — logging must never crash the app.
	_ = os.MkdirAll(opts.SpoolDir, 0o755)

	s := &shipper{
		opts:      opts,
		endpoint:  strings.TrimRight(opts.ServerURL, "/") + "/api/ingest",
		spoolPath: filepath.Join(opts.SpoolDir, serviceName+".jsonl"),
		queue:     make(chan Record, opts.QueueSize),
		flushReq:  make(chan chan struct{}),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
	s.wg.Add(1)
	go s.loop()
	return s
}

// enqueue offers rec to the queue without ever blocking. On a full queue the
// record is dropped and counted.
func (s *shipper) enqueue(rec Record) {
	select {
	case s.queue <- rec:
	default:
		s.dropped.Add(1)
	}
}

// dropped reports how many records were dropped due to a full queue.
func (s *shipper) droppedCount() int64 { return s.dropped.Load() }

func (s *shipper) loop() {
	defer s.wg.Done()

	flushTicker := time.NewTicker(s.opts.FlushInterval)
	retryTicker := time.NewTicker(s.opts.RetryInterval)
	defer flushTicker.Stop()
	defer retryTicker.Stop()

	var batch []Record

	for {
		select {
		case rec := <-s.queue:
			batch = append(batch, rec)
			if len(batch) >= s.opts.BatchSize {
				s.send(batch)
				batch = batch[:0]
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				s.send(batch)
				batch = batch[:0]
			}

		case <-retryTicker.C:
			s.drainSpool()

		case reply := <-s.flushReq:
			batch = s.drainQueue(batch)
			if len(batch) > 0 {
				s.send(batch)
				batch = batch[:0]
			}
			s.drainSpool()
			close(reply)

		case <-s.done:
			batch = s.drainQueue(batch)
			if len(batch) > 0 {
				s.send(batch)
			}
			s.drainSpool()
			close(s.stopped)
			return
		}
	}
}

// drainQueue pulls every currently-queued record into batch without blocking.
func (s *shipper) drainQueue(batch []Record) []Record {
	for {
		select {
		case rec := <-s.queue:
			batch = append(batch, rec)
		default:
			return batch
		}
	}
}

// flush synchronously drains the queue, sends any pending batch, and retries
// the spool, then returns. Bounded by ctx.
func (s *shipper) flush(ctx context.Context) {
	reply := make(chan struct{})
	select {
	case s.flushReq <- reply:
	case <-s.done:
		return
	case <-ctx.Done():
		return
	}
	select {
	case <-reply:
	case <-ctx.Done():
	}
}

// close drains and stops the shipper, bounded by ctx.
func (s *shipper) close(ctx context.Context) {
	s.closeOnce.Do(func() { close(s.done) })
	select {
	case <-s.stopped:
	case <-ctx.Done():
	}
}

// send delivers records in BatchSize chunks. On the first failing chunk it
// spools that chunk and all remaining records (preserving order) and stops.
func (s *shipper) send(records []Record) {
	for start := 0; start < len(records); start += s.opts.BatchSize {
		end := min(start+s.opts.BatchSize, len(records))
		if err := s.post(records[start:end]); err != nil {
			s.spoolAppend(records[start:])
			return
		}
	}
}

// drainSpool attempts to backfill spooled records to the server. It sends in
// chunks and rewrites the spool with only the records it could not deliver, so
// a partial success still makes progress and nothing is lost.
func (s *shipper) drainSpool() {
	recs := s.spoolRead()
	if len(recs) == 0 {
		return
	}
	sent := 0
	for start := 0; start < len(recs); start += s.opts.BatchSize {
		end := min(start+s.opts.BatchSize, len(recs))
		if err := s.post(recs[start:end]); err != nil {
			break // server still unhealthy; keep the rest spooled
		}
		sent = end
	}
	if sent == 0 {
		return
	}
	s.spoolRewrite(recs[sent:])
}

// post sends one batch to POST /api/ingest. A non-2xx status or transport error
// is returned as "not delivered".
func (s *shipper) post(records []Record) error {
	body, err := json.Marshal(ingestBody{Records: records})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.opts.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("logkit: ingest returned status %d", resp.StatusCode)
	}
	return nil
}

// --- spool file (JSON-lines); all calls happen on the loop goroutine ---

func (s *shipper) spoolAppend(records []Record) {
	if len(records) == 0 {
		return
	}
	f, err := os.OpenFile(s.spoolPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.dropped.Add(int64(len(records)))
		return
	}
	w := bufio.NewWriter(f)
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			continue
		}
		_, _ = w.Write(line)
		_ = w.WriteByte('\n')
	}
	_ = w.Flush()
	_ = f.Close()
	s.enforceCap()
}

func (s *shipper) spoolRead() []Record {
	f, err := os.Open(s.spoolPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var recs []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue // skip corrupt line
		}
		recs = append(recs, r)
	}
	return recs
}

// spoolRewrite replaces the spool with exactly records (removing the file when
// empty), via a temp file + atomic rename.
func (s *shipper) spoolRewrite(records []Record) {
	if len(records) == 0 {
		_ = os.Remove(s.spoolPath)
		return
	}
	tmp := s.spoolPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			continue
		}
		_, _ = w.Write(line)
		_ = w.WriteByte('\n')
	}
	_ = w.Flush()
	_ = f.Close()
	_ = os.Rename(tmp, s.spoolPath)
}

// enforceCap drops the oldest spooled records when the file exceeds
// SpoolMaxBytes, so the spool can never fill the disk.
func (s *shipper) enforceCap() {
	fi, err := os.Stat(s.spoolPath)
	if err != nil || fi.Size() <= s.opts.SpoolMaxBytes {
		return
	}
	recs := s.spoolRead()
	// Drop oldest records until the estimated on-disk size is under the cap.
	var size int64
	for _, r := range recs {
		if line, err := json.Marshal(r); err == nil {
			size += int64(len(line)) + 1
		}
	}
	drop := 0
	for drop < len(recs) && size > s.opts.SpoolMaxBytes {
		if line, err := json.Marshal(recs[drop]); err == nil {
			size -= int64(len(line)) + 1
		}
		drop++
	}
	s.spoolRewrite(recs[drop:])
}
