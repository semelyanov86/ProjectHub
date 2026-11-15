package desktop

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestPrepareImportPreview(t *testing.T) {
	f := newImportFixture(t)
	before := readTestFile(t, f.store.ProjectsPath())
	preview := prepareTestImport(t, f.Service)
	if preview.Total != 3 || preview.Eligible != 2 || preview.New != 1 || preview.Duplicates != 1 || preview.Current != 2 {
		t.Fatalf("preview counts = %+v, want total/eligible/new/duplicates/current = 3/2/1/1/2", preview)
	}
	if preview.FileName != filepath.Base(f.dialogs.importFile) || len(preview.Skipped) != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	assertSkippedPath(t, preview.Skipped, f.imported[2].Path, "does not exist")
	if !bytes.Equal(before, readTestFile(t, f.store.ProjectsPath())) {
		t.Fatal("preview modified the catalog")
	}
}

func TestApplyImportMerge(t *testing.T) {
	f := newImportFixture(t)
	preview := prepareTestImport(t, f.Service)
	report, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: ImportMerge})
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != ImportMerge || report.Added != 1 || len(report.Skipped) != 2 || report.BackupPath != "" {
		t.Fatalf("merge report = %+v", report)
	}
	assertSkippedPath(t, report.Skipped, f.imported[0].Path, "already registered")
	assertSkippedPath(t, report.Skipped, f.imported[2].Path, "does not exist")
	projects := listTestProjects(t, f.Service)
	if len(projects) != preview.Current+1 || projects[len(projects)-1].Path != f.imported[1].Path {
		t.Fatalf("merged projects = %+v", projects)
	}
	if _, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: ImportMerge}); !errors.Is(err, ErrImportExpired) {
		t.Fatalf("reused token error = %v, want ErrImportExpired", err)
	}
}

func TestApplyImportReplaceRequiresConfirmation(t *testing.T) {
	f := newImportFixture(t)
	before := readTestFile(t, f.store.ProjectsPath())
	preview := prepareTestImport(t, f.Service)
	request := ImportRequest{Token: preview.Token, Mode: ImportReplace}
	if _, err := f.ApplyImport(request); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("unconfirmed replace error = %v, want ErrConfirmationRequired", err)
	}
	if !bytes.Equal(before, readTestFile(t, f.store.ProjectsPath())) {
		t.Fatal("unconfirmed replace changed the catalog")
	}
	request.ReplaceConfirmed = true
	report, err := f.ApplyImport(request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != ImportReplace || report.Added != 2 || len(report.Skipped) != 1 || report.BackupPath == "" {
		t.Fatalf("replace report = %+v", report)
	}
	if !bytes.Equal(before, readTestFile(t, report.BackupPath)) {
		t.Fatal("backup does not equal the previous projects.json byte for byte")
	}
	assertSkippedPath(t, report.Skipped, f.imported[2].Path, "does not exist")
	if got := listTestProjects(t, f.Service); !reflect.DeepEqual(got, f.imported[:2]) {
		t.Fatalf("replaced projects = %+v, want %+v", got, f.imported[:2])
	}
}

func TestApplyImportExpiredOrDiscarded(t *testing.T) {
	for _, name := range []string{"expired", "discarded"} {
		t.Run(name, func(t *testing.T) {
			f := newImportFixture(t)
			before := readTestFile(t, f.store.ProjectsPath())
			preview := prepareTestImport(t, f.Service)
			if name == "expired" {
				future := f.now().Add(10*time.Minute + time.Second)
				f.now = func() time.Time { return future }
			} else {
				f.DiscardImport(preview.Token)
			}
			if _, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: ImportMerge}); !errors.Is(err, ErrImportExpired) {
				t.Fatalf("ApplyImport() error = %v, want ErrImportExpired", err)
			}
			if !bytes.Equal(before, readTestFile(t, f.store.ProjectsPath())) {
				t.Fatal("expired or discarded import changed the catalog")
			}
		})
	}
}

func TestApplyImportUnknownMode(t *testing.T) {
	f := newImportFixture(t)
	preview := prepareTestImport(t, f.Service)
	if _, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: "unknown"}); err == nil {
		t.Fatal("unknown import mode succeeded")
	}
	if _, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: ImportMerge}); err != nil {
		t.Fatalf("invalid mode consumed token: %v", err)
	}
}

func TestPrepareImportCancelled(t *testing.T) {
	h := newTestService(t)
	preview, err := h.PrepareImport(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Cancelled || preview.Token != "" {
		t.Fatalf("PrepareImport() = %+v, want cancellation", preview)
	}
}

func TestPrepareImportMalformedJSON(t *testing.T) {
	h := newTestService(t)
	h.dialogs.importFile = filepath.Join(t.TempDir(), "broken.json")
	writeTestFile(t, h.dialogs.importFile, []byte("{"))
	if _, err := h.PrepareImport(t.Context()); !errors.Is(err, config.ErrCorruptedConfig) {
		t.Fatalf("PrepareImport() error = %v, want ErrCorruptedConfig", err)
	}
}

func TestApplyImportDirectoryDeletedAfterPreview(t *testing.T) {
	f := newImportFixture(t)
	preview := prepareTestImport(t, f.Service)
	if err := os.Remove(f.imported[1].Path); err != nil {
		t.Fatal(err)
	}
	report, err := f.ApplyImport(ImportRequest{Token: preview.Token, Mode: ImportMerge})
	if err != nil {
		t.Fatal(err)
	}
	if report.Added != 0 || len(report.Skipped) != 3 {
		t.Fatalf("report after directory disappeared = %+v", report)
	}
	assertSkippedPath(t, report.Skipped, f.imported[1].Path, "does not exist")
	if got := listTestProjects(t, f.Service); len(got) != preview.Current {
		t.Fatalf("catalog changed after skipped import: %+v", got)
	}
}

func TestExportProjects(t *testing.T) {
	h := newTestService(t)
	p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	result, err := h.ExportProjects(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cancelled {
		t.Fatalf("ExportProjects() = %+v, want cancellation", result)
	}
	h.dialogs.exportFile = filepath.Join(t.TempDir(), "my export")
	result, err = h.ExportProjects(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if result.Cancelled || result.Path != h.dialogs.exportFile || result.Count != 1 {
		t.Fatalf("export result = %+v", result)
	}
	exported, err := h.store.ReadProjectsDocument(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exported, []project.Project{p}) {
		t.Fatalf("exported projects = %+v, want %+v", exported, p)
	}
	if h.dialogs.exportDefaultName != "projecthub-projects-2026-11-15.json" {
		t.Errorf("default export name = %q", h.dialogs.exportDefaultName)
	}
	before := readTestFile(t, h.store.ProjectsPath())
	h.dialogs.exportFile = h.store.ProjectsPath()
	if _, err := h.ExportProjects(t.Context()); err == nil {
		t.Fatal("export to projects.json succeeded")
	}
	if !bytes.Equal(before, readTestFile(t, h.store.ProjectsPath())) {
		t.Fatal("export overwrote projects.json")
	}
}

func TestApplyImportKeepsTokenAfterStorageFailure(t *testing.T) {
	f := newImportFixture(t)
	preview := prepareTestImport(t, f.Service)
	original := readTestFile(t, f.store.ProjectsPath())
	writeTestFile(t, f.store.ProjectsPath(), []byte("{broken"))

	request := ImportRequest{Token: preview.Token, Mode: ImportReplace, ReplaceConfirmed: true}
	if _, err := f.ApplyImport(request); err == nil {
		t.Fatal("ApplyImport() over a corrupted catalog succeeded")
	}
	if got := readTestFile(t, f.store.ProjectsPath()); string(got) != "{broken" {
		t.Fatalf("corrupted catalog was modified: %q", got)
	}

	writeTestFile(t, f.store.ProjectsPath(), original)
	report, err := f.ApplyImport(request)
	if err != nil {
		t.Fatalf("retry with the same token failed: %v", err)
	}
	if report.Mode != ImportReplace || report.BackupPath == "" {
		t.Fatalf("retry report = %+v, want replace with backup", report)
	}
}

func TestExportKeepsDialogFilename(t *testing.T) {
	h := newTestService(t)
	addTestProject(t, h.Service, projectDirectory(t, "alpha"))
	dir := t.TempDir()
	existing := filepath.Join(dir, "catalog.json")
	writeTestFile(t, existing, []byte("keep me"))
	h.dialogs.exportFile = filepath.Join(dir, "catalog")

	result, err := h.ExportProjects(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != h.dialogs.exportFile {
		t.Fatalf("export path = %q, want the dialog filename %q", result.Path, h.dialogs.exportFile)
	}
	if got := readTestFile(t, existing); string(got) != "keep me" {
		t.Fatalf("export overwrote an unconfirmed file: %q", got)
	}
}
