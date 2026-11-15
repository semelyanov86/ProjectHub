package project

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MaxBatchDirectories caps how many directories one AddDirectories call accepts.
const MaxBatchDirectories = 1000

// Storage persists the project catalog. UpdateProjects must apply mutate to a
// copy of the current catalog and persist the result atomically, writing
// nothing when mutate returns an error.
type Storage interface {
	LoadProjects() ([]Project, error)
	UpdateProjects(mutate func([]Project) ([]Project, error)) ([]Project, error)
}

// DirValidator validates a project directory and returns its cleaned absolute
// path. Production code uses launcher.ValidateProjectDir.
type DirValidator func(path string) (string, error)

// errNoChanges aborts a transaction that would not modify the catalog.
var errNoChanges = errors.New("no catalog changes")

// Catalog implements project catalog operations on top of Storage. All
// mutations run inside a single Storage transaction, so concurrent operations
// never lose each other's updates.
type Catalog struct {
	store       Storage
	validateDir DirValidator
	now         func() time.Time
	newID       func() (string, error)
}

// NewCatalog creates a Catalog persisting through store and validating
// directories with validateDir.
func NewCatalog(store Storage, validateDir DirValidator) *Catalog {
	return &Catalog{
		store:       store,
		validateDir: validateDir,
		now:         func() time.Time { return time.Now().UTC() },
		newID:       randomID,
	}
}

func randomID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate project id: %w", err)
	}
	return "p" + hex.EncodeToString(b[:]), nil
}

// List returns all registered projects.
func (c *Catalog) List() ([]Project, error) {
	projects, err := c.store.LoadProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	if projects == nil {
		return []Project{}, nil
	}
	return projects, nil
}

// Get returns the project with the given id.
func (c *Catalog) Get(id string) (Project, error) {
	projects, err := c.List()
	if err != nil {
		return Project{}, err
	}
	if i := indexByID(projects, id); i >= 0 {
		return projects[i], nil
	}
	return Project{}, fmt.Errorf("%w: %q", ErrProjectNotFound, id)
}

// FindByPath returns the project registered for path, if any.
func (c *Catalog) FindByPath(path string) (Project, bool, error) {
	projects, err := c.List()
	if err != nil {
		return Project{}, false, err
	}
	if i := indexByPath(projects, path, ""); i >= 0 {
		return projects[i], true, nil
	}
	return Project{}, false, nil
}

// Add registers an existing directory as a new project.
func (c *Catalog) Add(in Input) (Project, error) {
	fields, err := normalizeInput(in, nil)
	if err != nil {
		return Project{}, err
	}
	path, err := c.validateDir(in.Path)
	if err != nil {
		return Project{}, err
	}
	id, err := c.newID()
	if err != nil {
		return Project{}, err
	}
	now := c.now()
	created := Project{
		ID:            id,
		Name:          fields.Name,
		Description:   fields.Description,
		Path:          path,
		Favorite:      fields.Favorite,
		DefaultEditor: fields.DefaultEditor,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := created.Validate(); err != nil {
		return Project{}, err
	}

	_, err = c.store.UpdateProjects(func(projects []Project) ([]Project, error) {
		if i := indexByPath(projects, path, ""); i >= 0 {
			return nil, duplicateError(path, projects[i])
		}
		return append(projects, created), nil
	})
	if err != nil {
		return Project{}, err
	}
	return created, nil
}

// Update replaces the user-editable fields of project id. The directory is
// validated only when it changes, so metadata of a project whose directory
// went missing can still be edited.
func (c *Catalog) Update(id string, in Input) (Project, error) {
	current, err := c.Get(id)
	if err != nil {
		return Project{}, err
	}
	fields, err := normalizeInput(in, &current)
	if err != nil {
		return Project{}, err
	}
	path := current.Path
	pathChanged := PathKey(in.Path) != PathKey(current.Path)
	if pathChanged {
		if path, err = c.validateDir(in.Path); err != nil {
			return Project{}, err
		}
	}

	var updated Project
	_, err = c.store.UpdateProjects(func(projects []Project) ([]Project, error) {
		i := indexByID(projects, id)
		if i < 0 {
			return nil, fmt.Errorf("%w: %q", ErrProjectNotFound, id)
		}
		if PathKey(projects[i].Path) != PathKey(current.Path) {
			return nil, fmt.Errorf("%w: %q was moved", ErrProjectChanged, current.Name)
		}
		if dup := indexByPath(projects, path, id); pathChanged && dup >= 0 {
			return nil, duplicateError(path, projects[dup])
		}
		p := projects[i]
		p.Name = fields.Name
		p.Description = fields.Description
		p.DefaultEditor = fields.DefaultEditor
		p.Favorite = fields.Favorite
		p.Path = path
		p.UpdatedAt = c.now()
		if err := p.Validate(); err != nil {
			return nil, err
		}
		projects[i] = p
		updated = p
		return projects, nil
	})
	if err != nil {
		return Project{}, err
	}
	return updated, nil
}

// Remove unregisters project id. The directory on disk is never touched.
func (c *Catalog) Remove(id string) error {
	_, err := c.store.UpdateProjects(func(projects []Project) ([]Project, error) {
		i := indexByID(projects, id)
		if i < 0 {
			return nil, fmt.Errorf("%w: %q", ErrProjectNotFound, id)
		}
		return append(projects[:i], projects[i+1:]...), nil
	})
	return err
}

// SetFavorite marks or unmarks project id as a favorite.
func (c *Catalog) SetFavorite(id string, favorite bool) (Project, error) {
	return c.modify(id, func(p *Project) error {
		p.Favorite = favorite
		p.UpdatedAt = c.now()
		return nil
	})
}

// RecordOpened stores that project id was opened from expectedPath. When
// editorID is non-empty it is remembered as the project's last used editor.
// It fails with ErrProjectChanged when the project was removed or moved while
// the application was being launched, instead of overwriting newer state.
func (c *Catalog) RecordOpened(id, expectedPath, editorID string) (Project, error) {
	if editorID != "" {
		if err := validateEditorRef("lastUsedEditor", editorID); err != nil {
			return Project{}, err
		}
	}
	return c.modify(id, func(p *Project) error {
		if PathKey(p.Path) != PathKey(expectedPath) {
			return fmt.Errorf("%w: %q was moved", ErrProjectChanged, p.Name)
		}
		opened := c.now()
		p.LastOpened = &opened
		if editorID != "" {
			p.LastUsedEditor = editorID
		}
		return nil
	})
}

func (c *Catalog) modify(id string, apply func(*Project) error) (Project, error) {
	var updated Project
	_, err := c.store.UpdateProjects(func(projects []Project) ([]Project, error) {
		i := indexByID(projects, id)
		if i < 0 {
			return nil, fmt.Errorf("%w: %q", ErrProjectNotFound, id)
		}
		p := projects[i]
		if err := apply(&p); err != nil {
			return nil, err
		}
		projects[i] = p
		updated = p
		return projects, nil
	})
	if err != nil {
		return Project{}, err
	}
	return updated, nil
}

// AddDirectories registers several directories at once using their base
// names. Invalid, missing, non-directory, and already registered paths are
// reported as skipped; valid ones are committed in one transaction.
func (c *Catalog) AddDirectories(paths []string) (BatchReport, error) {
	report := BatchReport{Added: []Project{}, Skipped: []Skip{}}
	candidates := make([]Project, 0, min(len(paths), MaxBatchDirectories))
	for i, raw := range paths {
		if i >= MaxBatchDirectories {
			report.Skipped = append(report.Skipped, Skip{Path: raw, Reason: "too many directories in one drop"})
			continue
		}
		path, err := c.validateDir(raw)
		if err != nil {
			report.Skipped = append(report.Skipped, Skip{Path: raw, Reason: err.Error()})
			continue
		}
		id, err := c.newID()
		if err != nil {
			return BatchReport{}, err
		}
		now := c.now()
		candidate := Project{
			ID:        id,
			Name:      SuggestName(path),
			Path:      path,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// Validate each directory separately so one unusual entry cannot
		// abort the whole batch transaction.
		if err := candidate.Validate(); err != nil {
			report.Skipped = append(report.Skipped, Skip{Path: raw, Reason: err.Error()})
			continue
		}
		candidates = append(candidates, candidate)
	}
	return c.commitBatch(candidates, report)
}

// MergeImported adds imported projects that are not registered yet. Projects
// whose path is already registered (or repeated in the batch) are skipped;
// IDs colliding with existing projects are regenerated.
func (c *Catalog) MergeImported(projects []Project) (BatchReport, error) {
	report := BatchReport{Added: []Project{}, Skipped: []Skip{}}
	candidates := make([]Project, 0, len(projects))
	now := c.now()
	for _, p := range projects {
		p.Path = PathKey(p.Path)
		p.UpdatedAt = now
		if err := p.Validate(); err != nil {
			report.Skipped = append(report.Skipped, Skip{Path: p.Path, Name: p.Name, Reason: err.Error()})
			continue
		}
		candidates = append(candidates, p)
	}
	return c.commitBatch(candidates, report)
}

// commitBatch appends candidates that do not duplicate an existing or earlier
// path, regenerating colliding IDs, in a single transaction.
func (c *Catalog) commitBatch(candidates []Project, report BatchReport) (BatchReport, error) {
	if len(candidates) == 0 {
		return report, nil
	}
	var added []Project
	var skipped []Skip
	_, err := c.store.UpdateProjects(func(current []Project) ([]Project, error) {
		added, skipped = nil, nil
		ids := make(map[string]struct{}, len(current)+len(candidates))
		paths := make(map[string]string, len(current)+len(candidates))
		for _, p := range current {
			ids[p.ID] = struct{}{}
			paths[PathKey(p.Path)] = p.Name
		}
		for _, p := range candidates {
			if name, exists := paths[PathKey(p.Path)]; exists {
				skipped = append(skipped, Skip{Path: p.Path, Name: p.Name, Reason: fmt.Sprintf("already registered as %q", name)})
				continue
			}
			if _, taken := ids[p.ID]; taken {
				id, err := c.newID()
				if err != nil {
					return nil, err
				}
				p.ID = id
			}
			ids[p.ID] = struct{}{}
			paths[PathKey(p.Path)] = p.Name
			added = append(added, p)
		}
		if len(added) == 0 {
			return nil, errNoChanges
		}
		return append(current, added...), nil
	})
	if err != nil && !errors.Is(err, errNoChanges) {
		return BatchReport{}, err
	}
	report.Added = append(report.Added, added...)
	report.Skipped = append(report.Skipped, skipped...)
	return report, nil
}

// normalizeInput trims and validates the user-editable fields except the path.
// When current is non-nil (an edit), a name or description that is unchanged
// from the stored record is accepted as is, so records created before the input
// limits existed stay editable.
func normalizeInput(in Input, current *Project) (Input, error) {
	out := Input{
		Name:          strings.TrimSpace(in.Name),
		Description:   strings.TrimSpace(in.Description),
		Path:          in.Path,
		DefaultEditor: strings.TrimSpace(in.DefaultEditor),
		Favorite:      in.Favorite,
	}
	if current == nil || out.Name != strings.TrimSpace(current.Name) {
		if err := validateName(out.Name); err != nil {
			return Input{}, err
		}
	}
	if current == nil || out.Description != strings.TrimSpace(current.Description) {
		if err := validateDescription(out.Description); err != nil {
			return Input{}, err
		}
	}
	if err := validateEditorRef("defaultEditor", out.DefaultEditor); err != nil {
		return Input{}, err
	}
	if strings.TrimSpace(out.Path) == "" {
		return Input{}, fmt.Errorf("%w: directory is required", ErrInvalidProject)
	}
	return out, nil
}

func duplicateError(path string, existing Project) error {
	return fmt.Errorf("%w: %s (as %q)", ErrDuplicateProject, path, existing.Name)
}

func indexByID(projects []Project, id string) int {
	for i, p := range projects {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// indexByPath finds a project registered for path, ignoring exceptID.
func indexByPath(projects []Project, path, exceptID string) int {
	key := PathKey(path)
	for i, p := range projects {
		if p.ID != exceptID && PathKey(p.Path) == key {
			return i
		}
	}
	return -1
}
