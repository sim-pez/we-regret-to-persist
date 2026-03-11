package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sim-pez/we-regret-to-persist/internal/core/entity"
)

// mockExtractor implements CompanyAndStatusExtractor
type mockExtractor struct {
	company string
	status  entity.ApplicationStatus
	proceed bool
}

func (m *mockExtractor) Execute(_ context.Context, email *entity.Email) (string, entity.ApplicationStatus, bool, error) {
	return m.company, m.status, m.proceed, nil
}

// mockRepository implements Repository
type mockRepository struct {
	existingApp     *entity.Application
	capturedApp     *entity.Application
	capturedProceed bool
	called          bool
	err             error
}

func (m *mockRepository) UpsertApplication(_ context.Context, _ string, updateFn func(*entity.Application) (bool, *entity.Application)) error {
	if m.err != nil {
		return m.err
	}
	m.called = true
	m.capturedProceed, m.capturedApp = updateFn(m.existingApp)
	return nil
}

func newTestUsecase(ext *mockExtractor, repo *mockRepository) *UpdateApplicationStatus {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewUpdateApplicationStatus(logger, repo, ext)
}

func testEmail() *entity.Email {
	t := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	return &entity.Email{
		From:    "hr@company.com",
		Subject: "Your application",
		Date:    t,
	}
}

func TestUpdateApplicationStatus_SkipNonJobEmail(t *testing.T) {
	ext := &mockExtractor{proceed: false}
	repo := &mockRepository{}
	uc := newTestUsecase(ext, repo)

	err := uc.Execute(context.Background(), testEmail())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.called {
		t.Error("repository should not be called when extractor returns proceed=false")
	}
}

func TestUpdateApplicationStatus_NewApplied(t *testing.T) {
	ext := &mockExtractor{company: "acme", status: entity.ApplicationStatusApplied, proceed: true}
	repo := &mockRepository{existingApp: nil}
	uc := newTestUsecase(ext, repo)

	email := testEmail()
	err := uc.Execute(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.capturedProceed {
		t.Error("expected proceed=true")
	}
	if repo.capturedApp == nil {
		t.Fatal("expected non-nil application")
	}
	if repo.capturedApp.Status != entity.ApplicationStatusApplied {
		t.Errorf("expected status=%q, got %q", entity.ApplicationStatusApplied, repo.capturedApp.Status)
	}
	if repo.capturedApp.AppliedAt == nil || !repo.capturedApp.AppliedAt.Equal(email.Date) {
		t.Error("expected AppliedAt to be set to email date")
	}
}

func TestUpdateApplicationStatus_RejectionUnknownCompany(t *testing.T) {
	ext := &mockExtractor{company: "ghost", status: entity.ApplicationStatusRejected, proceed: true}
	repo := &mockRepository{existingApp: nil}
	uc := newTestUsecase(ext, repo)

	email := testEmail()
	err := uc.Execute(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.capturedProceed {
		t.Error("expected proceed=true for rejection of unknown company")
	}
	if repo.capturedApp == nil {
		t.Fatal("expected non-nil application for unknown rejection")
	}
	if repo.capturedApp.Status != entity.ApplicationStatusRejected {
		t.Errorf("expected status=%q, got %q", entity.ApplicationStatusRejected, repo.capturedApp.Status)
	}
	if repo.capturedApp.RejectedAt == nil || !repo.capturedApp.RejectedAt.Equal(email.Date) {
		t.Error("expected RejectedAt to be set to email date")
	}
}

func TestUpdateApplicationStatus_AdvancedUnknownCompany(t *testing.T) {
	ext := &mockExtractor{company: "newco", status: entity.ApplicationStatusAdvanced, proceed: true}
	repo := &mockRepository{existingApp: nil}
	uc := newTestUsecase(ext, repo)

	err := uc.Execute(context.Background(), testEmail())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.capturedProceed {
		t.Error("expected proceed=true for advanced status on new company")
	}
	if repo.capturedApp == nil {
		t.Fatal("expected non-nil application")
	}
	if repo.capturedApp.Status != entity.ApplicationStatusAdvanced {
		t.Errorf("expected status=%q, got %q", entity.ApplicationStatusAdvanced, repo.capturedApp.Status)
	}
	if repo.capturedApp.AppliedAt != nil {
		t.Error("AppliedAt should be nil for previously unseen advanced company")
	}
}

func TestUpdateApplicationStatus_RejectionExistingCompany(t *testing.T) {
	appliedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := &entity.Application{
		Company:   "acme",
		AppliedAt: &appliedAt,
		Status:    entity.ApplicationStatusApplied,
	}
	ext := &mockExtractor{company: "acme", status: entity.ApplicationStatusRejected, proceed: true}
	repo := &mockRepository{existingApp: existing}
	uc := newTestUsecase(ext, repo)

	email := testEmail()
	err := uc.Execute(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.capturedProceed {
		t.Error("expected proceed=true")
	}
	if repo.capturedApp.Status != entity.ApplicationStatusRejected {
		t.Errorf("expected status=%q, got %q", entity.ApplicationStatusRejected, repo.capturedApp.Status)
	}
	if repo.capturedApp.RejectedAt == nil || !repo.capturedApp.RejectedAt.Equal(email.Date) {
		t.Error("expected RejectedAt to be set to email date")
	}
}

func TestUpdateApplicationStatus_AdvancedExistingCompany(t *testing.T) {
	appliedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := &entity.Application{
		Company:   "acme",
		AppliedAt: &appliedAt,
		Status:    entity.ApplicationStatusApplied,
	}
	ext := &mockExtractor{company: "acme", status: entity.ApplicationStatusAdvanced, proceed: true}
	repo := &mockRepository{existingApp: existing}
	uc := newTestUsecase(ext, repo)

	err := uc.Execute(context.Background(), testEmail())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.capturedProceed {
		t.Error("expected proceed=true")
	}
	if repo.capturedApp.Status != entity.ApplicationStatusAdvanced {
		t.Errorf("expected status=%q, got %q", entity.ApplicationStatusAdvanced, repo.capturedApp.Status)
	}
}

func TestUpdateApplicationStatus_RepositoryError(t *testing.T) {
	ext := &mockExtractor{company: "acme", status: entity.ApplicationStatusApplied, proceed: true}
	repo := &mockRepository{err: errors.New("db error")}
	uc := newTestUsecase(ext, repo)

	err := uc.Execute(context.Background(), testEmail())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
