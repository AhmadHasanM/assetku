package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
)

const (
	// MaxFileBytes is the upload size limit.
	MaxFileBytes = 10 << 20
	// MaxDataRows is the limit on non-blank data rows below the header.
	MaxDataRows = 10000
	// batchSize rows are applied per transaction; processed_rows is updated
	// after each commit so polling clients see progress.
	batchSize = 25
)

var (
	errHeaderNotFound = errors.New("import: header row not found")
	errTooManyRows    = errors.New("import: too many rows")
)

// dataRow is a non-blank data row and its 1-based spreadsheet row number.
type dataRow struct {
	num   int
	cells []string
}

// parsedFile is an uploaded file reduced to what a job needs.
type parsedFile struct {
	cols    map[string]int
	unknown []string
	rows    []dataRow
}

// parseFile reads the file, finds the header row and keeps the non-blank data
// rows below it. It returns ErrUnsupportedFormat, errHeaderNotFound,
// errTooManyRows, or a read error.
func parseFile(name string, r io.Reader) (*parsedFile, error) {
	rows, err := ReadFile(name, r)
	if err != nil {
		return nil, err
	}
	idx, ok := FindHeaderRow(rows)
	if !ok {
		return nil, errHeaderNotFound
	}
	f := &parsedFile{}
	f.cols, f.unknown = MapHeaders(rows[idx])
	for i := idx + 1; i < len(rows); i++ {
		if blankRow(rows[i]) {
			continue
		}
		if len(f.rows) == MaxDataRows {
			return nil, errTooManyRows
		}
		f.rows = append(f.rows, dataRow{num: i + 1, cells: rows[i]})
	}
	return f, nil
}

// process applies every row of f in batches of batchSize, each batch in its
// own transaction (one savepoint per row inside it). progress, if set, is
// called with the number of rows processed after each committed batch.
func process(ctx context.Context, pool *pgxpool.Pool, p auth.Principal, f *parsedFile, progress func(n int) error) (*Result, error) {
	res := NewResult()
	if f.unknown != nil {
		res.UnknownColumns = f.unknown
	}
	seen := map[string]int{}
	for start := 0; start < len(f.rows); start += batchSize {
		end := min(start+batchSize, len(f.rows))
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			for _, r := range f.rows[start:end] {
				if err := ApplyRow(ctx, tx, p, f.cols, r.cells, r.num, res, seen); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if progress != nil {
			if err := progress(end); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

// job is a queued import held in memory between upload and Run.
type job struct {
	principal auth.Principal
	file      *parsedFile
}

// pending holds parsed uploads keyed by import id until Run takes them.
var pending = struct {
	sync.Mutex
	jobs map[uuid.UUID]*job
}{jobs: map[uuid.UUID]*job{}}

func putJob(id uuid.UUID, j *job) {
	pending.Lock()
	defer pending.Unlock()
	pending.jobs[id] = j
}

func takeJob(id uuid.UUID) *job {
	pending.Lock()
	defer pending.Unlock()
	j := pending.jobs[id]
	delete(pending.jobs, id)
	return j
}

// Run executes the queued import jobID: it applies the rows held in memory
// for it, updating processed_rows every batchSize rows, and finishes with
// status done and the result, or failed and the error text. Rows committed
// before a failure stay imported.
func Run(ctx context.Context, pool *pgxpool.Pool, jobID uuid.UUID) (err error) {
	j := takeJob(jobID)
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("import: panic: %v", rec)
		}
		if err != nil {
			if _, uerr := pool.Exec(context.WithoutCancel(ctx),
				`UPDATE imports SET status = 'failed', error = $2, finished_at = now() WHERE id = $1`,
				jobID, err.Error()); uerr != nil {
				log.Printf("import %s: mark failed: %v", jobID, uerr)
			}
		}
	}()
	if j == nil {
		return errors.New("job_data_missing")
	}
	if _, err := pool.Exec(ctx, `UPDATE imports SET status = 'running' WHERE id = $1`, jobID); err != nil {
		return err
	}
	res, err := process(ctx, pool, j.principal, j.file, func(n int) error {
		_, err := pool.Exec(ctx, `UPDATE imports SET processed_rows = $2 WHERE id = $1`, jobID, n)
		return err
	})
	if err != nil {
		return err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE imports SET status = 'done', processed_rows = total_rows, result = $2,
		finished_at = now() WHERE id = $1`, jobID, b)
	return err
}

// FailInterrupted marks imports left queued or running by a previous server
// process as failed with error 'server_restarted'. Call once at startup.
func FailInterrupted(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `UPDATE imports SET status = 'failed', error = 'server_restarted', finished_at = now()
		WHERE status IN ('queued', 'running')`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
