package desktop

import (
	"context"
	"sync"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

// EventProjectsDropped is emitted to the frontend after directories were
// dropped onto the window from the file manager.
const EventProjectsDropped = "projects:dropped"

// DropResult is the payload of EventProjectsDropped. Exactly one field is set:
// Draft for a single valid directory (the frontend opens the Add dialog),
// Report for several directories (already registered), or Error.
type DropResult struct {
	Draft  *DirectoryDraft      `json:"draft,omitempty"`
	Report *project.BatchReport `json:"report,omitempty"`
	Error  string               `json:"error,omitempty"`
}

// DropHandler processes native file drops. It is deliberately not a bound
// service: paths are accepted only from the native drop event, never from
// JavaScript.
type DropHandler struct {
	svc  *Service
	emit func(DropResult)
	busy sync.Mutex
}

// NewDropHandler creates a DropHandler that reports results through emit.
func NewDropHandler(svc *Service, emit func(DropResult)) *DropHandler {
	return &DropHandler{svc: svc, emit: emit}
}

// Handle validates dropped paths. One directory produces a draft for the Add
// dialog; several are registered immediately with their directory names.
func (h *DropHandler) Handle(ctx context.Context, paths []string) {
	if len(paths) == 0 {
		return
	}
	if !h.busy.TryLock() {
		h.emit(DropResult{Error: "Another drop is still being processed."})
		return
	}
	defer h.busy.Unlock()

	if len(paths) == 1 {
		draft, err := h.svc.describeDirectory(ctx, paths[0])
		if err != nil {
			h.emit(DropResult{Error: err.Error()})
			return
		}
		h.emit(DropResult{Draft: &draft})
		return
	}

	report, err := h.svc.deps.Catalog.AddDirectories(paths)
	if err != nil {
		h.emit(DropResult{Error: err.Error()})
		return
	}
	h.emit(DropResult{Report: &report})
}
