// Package generate は生成ジョブ(GLB・サムネイル・抽出メタデータ)の
// 直列キューと Blender CLI 実行を提供する(requirements.md §7 生成)。
// ジョブ状態の通知方式は ADR-0002(ポーリング)を参照。
package generate

import "sync"

// Job は 1 アセット分の生成ジョブ。
type Job struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	// BlendPath は model.blend の絶対パス(投入時に確定)。
	BlendPath string `json:"-"`
	// LibDir は対象ライブラリ(キャッシュの書き出し先)。
	LibDir string `json:"-"`
}

func (j Job) key() string { return j.Category + "/" + j.Title }

// JobError は直近で失敗したジョブとその理由。
type JobError struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	Message  string `json:"message"`
}

// Status はキューの現在状態(GET /api/jobs のレスポンス)。
type Status struct {
	Running      *Job      `json:"running"`
	PendingCount int       `json:"pendingCount"`
	BatchDone    int       `json:"batchDone"`
	BatchTotal   int       `json:"batchTotal"`
	LastError    *JobError `json:"lastError"`
}

// Queue は同時実行 1 の直列ジョブキュー(Blender の多重起動防止)。
type Queue struct {
	run    func(Job) error
	onDone func(Job, error) // 完了ごとに呼ばれる(nil 可)。サーバーは再スキャンに使う

	mu         sync.Mutex
	pending    []Job
	running    *Job
	batchDone  int
	batchTotal int
	lastError  *JobError
	wake       chan struct{}
	stopped    bool
}

// NewQueue はワーカーを起動したキューを返す。
func NewQueue(run func(Job) error, onDone func(Job, error)) *Queue {
	q := &Queue{run: run, onDone: onDone, wake: make(chan struct{}, 1)}
	go q.worker()
	return q
}

// Enqueue はジョブを投入する。同じアセットが実行中・待機中なら false。
// キューが空(アイドル)からの投入で新しいバッチが始まる。
func (q *Queue) Enqueue(job Job) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		return false
	}
	if q.running != nil && q.running.key() == job.key() {
		return false
	}
	for _, p := range q.pending {
		if p.key() == job.key() {
			return false
		}
	}
	if q.running == nil && len(q.pending) == 0 {
		// 新しいバッチの開始
		q.batchDone = 0
		q.batchTotal = 0
		q.lastError = nil
	}
	q.pending = append(q.pending, job)
	q.batchTotal++
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return true
}

// Status は現在のキュー状態を返す。
func (q *Queue) Status() Status {
	q.mu.Lock()
	defer q.mu.Unlock()
	s := Status{
		PendingCount: len(q.pending),
		BatchDone:    q.batchDone,
		BatchTotal:   q.batchTotal,
		LastError:    q.lastError,
	}
	if q.running != nil {
		r := *q.running
		s.Running = &r
	}
	return s
}

// Stop はワーカーを止める(テスト・終了時用)。実行中ジョブは完了を待たない。
func (q *Queue) Stop() {
	q.mu.Lock()
	q.stopped = true
	q.pending = nil
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue) worker() {
	for {
		q.mu.Lock()
		if q.stopped {
			q.mu.Unlock()
			return
		}
		if len(q.pending) == 0 {
			q.mu.Unlock()
			<-q.wake
			continue
		}
		job := q.pending[0]
		q.pending = q.pending[1:]
		q.running = &job
		q.mu.Unlock()

		err := q.run(job)
		// onDone(再スキャン)が終わるまでは Running のままにする。
		// ポーリングが「空」を見た時点で、完了分は必ずインデックスに
		// 反映済みという保証になる(ADR-0002)
		if q.onDone != nil {
			q.onDone(job, err)
		}

		q.mu.Lock()
		q.running = nil
		q.batchDone++
		if err != nil {
			q.lastError = &JobError{Category: job.Category, Title: job.Title, Message: err.Error()}
		}
		q.mu.Unlock()
	}
}
