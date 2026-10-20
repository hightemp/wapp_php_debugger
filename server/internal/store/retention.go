package store

import (
	"context"
	"time"
)

// PruneRequests keeps the viewer's database within a project's retention: requests started more than
// days ago go, then the oldest ones while the messages take more than maxBytes. nil means no limit.
// Pinned requests always stay and do not count against the size.
// Log file states stay, so pruned requests are not imported again. It returns the number removed.
func (s *Store) PruneRequests(ctx context.Context, projectID int64, now time.Time, days *int, maxBytes *int64) (int64, error) {
	var removed int64
	if days != nil {
		cutoff := now.Add(-time.Duration(*days) * 24 * time.Hour).UnixMilli()
		result, err := s.db.ExecContext(ctx, `DELETE FROM requests WHERE project_id = ? AND started_at < ? AND pinned = 0`, projectID, cutoff)
		if err != nil {
			return removed, err
		}
		n, _ := result.RowsAffected()
		removed += n
	}
	if maxBytes != nil {
		// Running total of message sizes from the newest request back; past the limit everything goes
		result, err := s.db.ExecContext(ctx, `
			DELETE FROM requests WHERE id IN (
				SELECT id FROM (
					SELECT r.id, SUM(COALESCE(m.bytes, 0)) OVER (ORDER BY r.started_at DESC, r.id DESC) AS total
					FROM requests r
					LEFT JOIN (
						SELECT m.request_id, SUM(m.size_bytes) AS bytes
						FROM messages m JOIN requests own ON own.id = m.request_id
						WHERE own.project_id = ? GROUP BY m.request_id
					) m ON m.request_id = r.id
					WHERE r.project_id = ? AND r.pinned = 0
				) WHERE total > ?
			)`, projectID, projectID, *maxBytes)
		if err != nil {
			return removed, err
		}
		n, _ := result.RowsAffected()
		removed += n
	}
	return removed, nil
}
