package ipc

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
)

// FileEvent represents a file system event detected by the watcher.
type FileEvent struct {
	Path string
	Op   string // "create", "write", "remove"
}

// Watcher watches directories for file changes using inotify/fsnotify.
// This replaces NanoClaw's polling-based IPC with push-based notification.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	log       logr.Logger
	channels  map[string]chan FileEvent
	mu        sync.RWMutex
}

// NewWatcher creates a new file system watcher.
func NewWatcher(basePath string, log logr.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher: fsw,
		log:       log,
		channels:  make(map[string]chan FileEvent),
	}

	// Start the event dispatch loop
	go w.dispatch()

	return w, nil
}

// Watch registers a directory for monitoring and returns a channel of events.
func (w *Watcher) Watch(ctx context.Context, dirPath string) (<-chan FileEvent, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}

	if err := w.fsWatcher.Add(absPath); err != nil {
		return nil, err
	}

	ch := make(chan FileEvent, 64)
	w.mu.Lock()
	w.channels[absPath] = ch
	w.mu.Unlock()

	// Clean up when context is done
	go func() {
		<-ctx.Done()
		w.mu.Lock()
		delete(w.channels, absPath)
		w.mu.Unlock()
		close(ch)
		_ = w.fsWatcher.Remove(absPath)
	}()

	return ch, nil
}

// dispatch runs the fsnotify event loop and dispatches events to registered channels.
func (w *Watcher) dispatch() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Only process create/write events
			if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
				continue
			}

			dirPath := filepath.Dir(event.Name)
			op := "write"
			if event.Has(fsnotify.Create) {
				op = "create"
			}

			fe := FileEvent{
				Path: event.Name,
				Op:   op,
			}

			w.mu.RLock()
			if ch, ok := w.channels[dirPath]; ok {
				select {
				case ch <- fe:
				default:
					w.log.Info("File event channel full, dropping event", "path", event.Name)
				}
			}
			w.mu.RUnlock()

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.log.Error(err, "file watcher error")
		}
	}
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	return w.fsWatcher.Close()
}
