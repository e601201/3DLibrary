package generate

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func waitIdle(t *testing.T, q *Queue) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := q.Status()
		if s.Running == nil && s.PendingCount == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("queue did not become idle")
}

func TestQueueRunsJobsSerially(t *testing.T) {
	var mu sync.Mutex
	concurrent, maxConcurrent, ran := 0, 0, []string{}

	q := NewQueue(func(j Job) error {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		concurrent--
		ran = append(ran, j.Title)
		mu.Unlock()
		return nil
	}, nil)
	defer q.Stop()

	for _, title := range []string{"A", "B", "C"} {
		q.Enqueue(Job{Category: "Props", Title: title})
	}
	waitIdle(t, q)

	mu.Lock()
	defer mu.Unlock()
	if maxConcurrent != 1 {
		t.Errorf("maxConcurrent = %d, want 1(直列キュー)", maxConcurrent)
	}
	if len(ran) != 3 || ran[0] != "A" || ran[1] != "B" || ran[2] != "C" {
		t.Errorf("ran = %v, want [A B C](投入順)", ran)
	}
}

func TestQueueDedupesSameAsset(t *testing.T) {
	block := make(chan struct{})
	var mu sync.Mutex
	count := 0
	q := NewQueue(func(j Job) error {
		<-block
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}, nil)
	defer q.Stop()

	if !q.Enqueue(Job{Category: "Props", Title: "A"}) {
		t.Fatal("first enqueue should be accepted")
	}
	// 実行中の A と待機中の B はそれぞれ再投入を拒否する
	q.Enqueue(Job{Category: "Props", Title: "B"})
	if q.Enqueue(Job{Category: "Props", Title: "A"}) {
		t.Error("running job must not be enqueued twice")
	}
	if q.Enqueue(Job{Category: "Props", Title: "B"}) {
		t.Error("pending job must not be enqueued twice")
	}
	close(block)
	waitIdle(t, q)
	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestQueueStatusAndBatchProgress(t *testing.T) {
	step := make(chan struct{})
	q := NewQueue(func(j Job) error {
		<-step
		return nil
	}, nil)
	defer q.Stop()

	q.Enqueue(Job{Category: "Props", Title: "A"})
	q.Enqueue(Job{Category: "Props", Title: "B"})
	q.Enqueue(Job{Category: "Props", Title: "C"})

	// A 実行中、B・C 待機
	deadline := time.Now().Add(2 * time.Second)
	for {
		s := q.Status()
		if s.Running != nil && s.Running.Title == "A" && s.PendingCount == 2 {
			if s.BatchTotal != 3 || s.BatchDone != 0 {
				t.Fatalf("batch = %d/%d, want 0/3", s.BatchDone, s.BatchTotal)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status = %+v", s)
		}
		time.Sleep(5 * time.Millisecond)
	}

	step <- struct{}{} // A 完了
	for {
		s := q.Status()
		if s.Running != nil && s.Running.Title == "B" {
			if s.BatchDone != 1 || s.BatchTotal != 3 {
				t.Fatalf("batch = %d/%d, want 1/3", s.BatchDone, s.BatchTotal)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status = %+v", s)
		}
		time.Sleep(5 * time.Millisecond)
	}
	step <- struct{}{}
	step <- struct{}{}
	waitIdle(t, q)

	// 空になったらバッチはリセット。次の投入で新しいバッチが始まる
	q.Enqueue(Job{Category: "Props", Title: "D"})
	s := q.Status()
	if s.BatchTotal != 1 {
		t.Errorf("new batch total = %d, want 1", s.BatchTotal)
	}
	step <- struct{}{}
	waitIdle(t, q)
}

func TestQueueRecordsLastErrorAndContinues(t *testing.T) {
	var done []string
	var mu sync.Mutex
	q := NewQueue(func(j Job) error {
		mu.Lock()
		done = append(done, j.Title)
		mu.Unlock()
		if j.Title == "Bad" {
			return errors.New("blender exploded")
		}
		return nil
	}, nil)
	defer q.Stop()

	q.Enqueue(Job{Category: "Props", Title: "Bad"})
	q.Enqueue(Job{Category: "Props", Title: "Good"})
	waitIdle(t, q)

	mu.Lock()
	if len(done) != 2 {
		t.Fatalf("done = %v(失敗しても次のジョブへ進む)", done)
	}
	mu.Unlock()
	s := q.Status()
	if s.LastError == nil || s.LastError.Title != "Bad" {
		t.Fatalf("lastError = %+v", s.LastError)
	}
}

func TestQueueOnDoneCallback(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	q := NewQueue(
		func(j Job) error { return nil },
		func(j Job, err error) {
			mu.Lock()
			calls++
			mu.Unlock()
		},
	)
	defer q.Stop()
	q.Enqueue(Job{Category: "C", Title: "A"})
	q.Enqueue(Job{Category: "C", Title: "B"})
	waitIdle(t, q)
	// onDone は完了ごとに呼ばれる(サーバーはここで再スキャンする)
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		c := calls
		mu.Unlock()
		if c == 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("onDone calls = %d, want 2", c)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
