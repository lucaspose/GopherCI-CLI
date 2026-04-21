package main

import (
	"strings"
	"testing"

	"github.com/lucaspose/goci-cli/internal/api"
)

// ── paginateJobs ──────────────────────────────────────────────────────────────

func TestPaginateJobs_FullPage(t *testing.T) {
	jobs := make([]api.Job, 30)
	for i := range jobs {
		jobs[i] = api.Job{ID: strings.Repeat("a", 8+i%2)}
	}

	page, total := paginateJobs(jobs, 15, 0)
	if len(page) != 15 {
		t.Errorf("expected 15 items, got %d", len(page))
	}
	if total != 2 {
		t.Errorf("expected 2 pages, got %d", total)
	}
}

func TestPaginateJobs_LastPage(t *testing.T) {
	jobs := make([]api.Job, 17)
	page, total := paginateJobs(jobs, 15, 15)
	if len(page) != 2 {
		t.Errorf("expected 2 items on last page, got %d", len(page))
	}
	if total != 2 {
		t.Errorf("expected 2 pages, got %d", total)
	}
}

func TestPaginateJobs_Empty(t *testing.T) {
	page, total := paginateJobs(nil, 15, 0)
	if len(page) != 0 {
		t.Errorf("expected 0 items, got %d", len(page))
	}
	if total != 1 {
		t.Errorf("expected 1 page for empty, got %d", total)
	}
}

func TestPaginateJobs_OffsetBeyondEnd(t *testing.T) {
	jobs := make([]api.Job, 5)
	page, _ := paginateJobs(jobs, 15, 100)
	if len(page) != 0 {
		t.Errorf("expected 0 items beyond end, got %d", len(page))
	}
}

func TestPaginateJobs_ZeroPageSize(t *testing.T) {
	jobs := make([]api.Job, 5)
	page, total := paginateJobs(jobs, 0, 0)
	if len(page) != 5 {
		t.Errorf("expected all items for zero page size, got %d", len(page))
	}
	if total != 1 {
		t.Errorf("expected 1 page, got %d", total)
	}
}

func TestPaginateRange_NormalizesOffset(t *testing.T) {
	start, end, total, offset := paginateRange(17, 5, 99)
	if start != 15 || end != 17 {
		t.Fatalf("expected range [15:17], got [%d:%d]", start, end)
	}
	if total != 4 {
		t.Fatalf("expected 4 pages, got %d", total)
	}
	if offset != 15 {
		t.Fatalf("expected normalized offset 15, got %d", offset)
	}
}

func TestPaginateRange_Empty(t *testing.T) {
	start, end, total, offset := paginateRange(0, 5, 0)
	if start != 0 || end != 0 || total != 1 || offset != 0 {
		t.Fatalf("unexpected empty paginateRange output: %d %d %d %d", start, end, total, offset)
	}
}

// ── filterJobs ────────────────────────────────────────────────────────────────

func TestFilterJobs_EmptyFilter(t *testing.T) {
	jobs := []api.Job{{Status: "success"}, {Status: "failed"}, {Status: "running"}}
	result := filterJobs(jobs, "")
	if len(result) != 3 {
		t.Errorf("expected all 3 jobs, got %d", len(result))
	}
}

func TestFilterJobs_StatusFilter(t *testing.T) {
	jobs := []api.Job{
		{ID: "a", Status: "success"},
		{ID: "b", Status: "failed"},
		{ID: "c", Status: "success"},
	}
	result := filterJobs(jobs, "success")
	if len(result) != 2 {
		t.Errorf("expected 2 success jobs, got %d", len(result))
	}
	for _, j := range result {
		if j.Status != "success" {
			t.Errorf("unexpected status in result: %s", j.Status)
		}
	}
}

func TestFilterJobs_NoMatch(t *testing.T) {
	jobs := []api.Job{{Status: "success"}, {Status: "failed"}}
	result := filterJobs(jobs, "running")
	if result != nil {
		t.Errorf("expected nil for no match, got %v", result)
	}
}

// ── formatDuration ────────────────────────────────────────────────────────────

func TestFormatDuration_Zero(t *testing.T) {
	if s := formatDuration(0); s != "—" {
		t.Errorf("expected —, got %s", s)
	}
}

func TestFormatDuration_Negative(t *testing.T) {
	if s := formatDuration(-5); s != "—" {
		t.Errorf("expected —, got %s", s)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	if s := formatDuration(45); s != "45s" {
		t.Errorf("expected 45s, got %s", s)
	}
}

func TestFormatDuration_Minutes(t *testing.T) {
	if s := formatDuration(90); s != "1m 30s" {
		t.Errorf("expected 1m 30s, got %s", s)
	}
}

func TestFormatDuration_ExactMinute(t *testing.T) {
	if s := formatDuration(120); s != "2m 0s" {
		t.Errorf("expected 2m 0s, got %s", s)
	}
}

// ── renderBreadcrumb ──────────────────────────────────────────────────────────

func TestRenderBreadcrumb_Single(t *testing.T) {
	out := renderBreadcrumb("Menu")
	if !strings.Contains(out, "Menu") {
		t.Errorf("expected Menu in breadcrumb, got: %s", out)
	}
}

func TestRenderBreadcrumb_Multiple(t *testing.T) {
	out := renderBreadcrumb("Menu", "SSH Keys", "Ajouter")
	for _, seg := range []string{"Menu", "SSH Keys", "Ajouter"} {
		if !strings.Contains(out, seg) {
			t.Errorf("expected %q in breadcrumb output", seg)
		}
	}
}

func TestRenderBreadcrumb_Empty(t *testing.T) {
	out := renderBreadcrumb()
	if out != "" {
		t.Errorf("expected empty string for no segments, got %q", out)
	}
}

// ── renderPagination ──────────────────────────────────────────────────────────

func TestRenderPagination_SinglePage(t *testing.T) {
	if out := renderPagination(1, 1); out != "" {
		t.Errorf("expected empty for single page, got %q", out)
	}
}

func TestRenderPagination_MultiPage(t *testing.T) {
	out := renderPagination(2, 5)
	if !strings.Contains(out, "2") || !strings.Contains(out, "5") {
		t.Errorf("expected page numbers in output, got %q", out)
	}
}

func TestRenderPaginationWithHint(t *testing.T) {
	out := renderPaginationWithHint(2, 5, "left/right")
	if !strings.Contains(out, "left/right") {
		t.Fatalf("expected custom hint in output, got %q", out)
	}
}

// ── clamp ─────────────────────────────────────────────────────────────────────

func TestClamp_BelowMin(t *testing.T) {
	if v := clamp(-5, 0, 10); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
}

func TestClamp_AboveMax(t *testing.T) {
	if v := clamp(15, 0, 10); v != 10 {
		t.Errorf("expected 10, got %d", v)
	}
}

func TestClamp_InRange(t *testing.T) {
	if v := clamp(5, 0, 10); v != 5 {
		t.Errorf("expected 5, got %d", v)
	}
}

func TestClamp_InvertedBounds(t *testing.T) {
	if v := clamp(5, 10, 0); v != 10 {
		t.Errorf("expected lo when hi<lo, got %d", v)
	}
}

// ── shortID ───────────────────────────────────────────────────────────────────

func TestShortID_Long(t *testing.T) {
	if s := shortID("abcdef1234567890"); s != "abcdef12" {
		t.Errorf("expected abcdef12, got %s", s)
	}
}

func TestShortID_Short(t *testing.T) {
	if s := shortID("abc"); s != "abc" {
		t.Errorf("expected abc, got %s", s)
	}
}

// ── truncate ──────────────────────────────────────────────────────────────────

func TestTruncate_WithinLimit(t *testing.T) {
	if s := truncate("hello", 10); s != "hello" {
		t.Errorf("expected hello, got %s", s)
	}
}

func TestTruncate_AtLimit(t *testing.T) {
	if s := truncate("hello", 5); s != "hello" {
		t.Errorf("expected hello, got %s", s)
	}
}

func TestTruncate_Exceeds(t *testing.T) {
	if s := truncate("hello world", 5); s != "hello…" {
		t.Errorf("expected hello…, got %s", s)
	}
}
