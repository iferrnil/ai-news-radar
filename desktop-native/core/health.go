package core

import "strings"

// Health is one row of the 运行状态 page (sources + source_health, like the web /health page).
type Health struct {
	Source
	Runs, LastErr, LastKind string
	LastOK, LastErrAt       float64
	Consec                  int
	LatMs, LastMs           int64
	OpenUntil               float64
	Total                   int // items stored incl. hidden/merged
	Hidden                  int
}

// SuccessRate over the recorded runs (0..1), -1 when none.
func (h Health) SuccessRate() float64 {
	if h.Runs == "" {
		return -1
	}
	return float64(strings.Count(h.Runs, "1")) / float64(len(h.Runs))
}

func (s *Store) Health() ([]Health, error) {
	rows, err := s.DB.Query(`SELECT ` + sourceCols + `,
		(SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=0 AND i.merged_into IS NULL),
		(SELECT COUNT(*) FROM items i WHERE i.source_id=s.id),
		(SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=1),
		COALESCE(h.runs,''),COALESCE(h.last_err,''),COALESCE(h.last_kind,''),COALESCE(h.last_ok,0),COALESCE(h.last_err_at,0),
		COALESCE(h.consec,0),COALESCE(h.lat_ms,0),COALESCE(h.last_ms,0),COALESCE(h.open_until,0)
		FROM sources s LEFT JOIN source_health h ON h.source_id=s.id ORDER BY s.enabled DESC, COALESCE(h.consec,0) DESC, s.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Health
	for rows.Next() {
		var h Health
		src, err := scanSource(rows, &h.Count, &h.Total, &h.Hidden, &h.Runs, &h.LastErr, &h.LastKind, &h.LastOK, &h.LastErrAt, &h.Consec, &h.LatMs, &h.LastMs, &h.OpenUntil)
		if err != nil {
			return nil, err
		}
		cnt := h.Count
		h.Source = src
		h.Count = cnt
		out = append(out, h)
	}
	return out, rows.Err()
}
