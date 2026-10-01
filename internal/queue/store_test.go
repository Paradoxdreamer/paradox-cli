package queue

import (
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestEnqueueClaimComplete(t *testing.T) {
	st := testStore(t)
	j := &Job{ID: "job_1", Type: "echo", Payload: map[string]any{"x": 1}, MaxAttempts: 3}
	if err := st.Enqueue(j); err != nil {
		t.Fatal(err)
	}
	got, err := st.ClaimNext()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "job_1" {
		t.Fatalf("claim: %+v", got)
	}
	if got.Status != StatusRunning || got.Attempts != 1 {
		t.Fatalf("status=%s attempts=%d", got.Status, got.Attempts)
	}
	if err := st.Complete(got); err != nil {
		t.Fatal(err)
	}
	counts, err := st.Counts()
	if err != nil {
		t.Fatal(err)
	}
	if counts[StatusSucceeded] != 1 || counts[StatusPending] != 0 {
		t.Fatalf("counts=%v", counts)
	}
}

func TestFailMovesToDeadAfterMaxAttempts(t *testing.T) {
	st := testStore(t)
	j := &Job{ID: "job_fail", Type: "echo", MaxAttempts: 1}
	if err := st.Enqueue(j); err != nil {
		t.Fatal(err)
	}
	got, err := st.ClaimNext()
	if err != nil || got == nil {
		t.Fatalf("claim: %v %+v", err, got)
	}
	if err := st.Fail(got, "boom"); err != nil {
		t.Fatal(err)
	}
	counts, _ := st.Counts()
	if counts[StatusDead] != 1 {
		t.Fatalf("want dead=1, counts=%v", counts)
	}
	next, err := st.ClaimNext()
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("expected no claimable job, got %+v", next)
	}
}

func TestClaimSkipsFutureRunAt(t *testing.T) {
	st := testStore(t)
	j := &Job{ID: "job_later", Type: "echo", MaxAttempts: 3}
	if err := st.Enqueue(j); err != nil {
		t.Fatal(err)
	}
	j.Status = StatusPending
	j.RunAt = time.Now().UTC().Add(time.Hour)
	if err := st.write(j); err != nil {
		t.Fatal(err)
	}
	next, err := st.ClaimNext()
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("should skip future RunAt, got %+v", next)
	}
}
