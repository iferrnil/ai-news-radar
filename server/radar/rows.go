package radar

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"strconv"
)

// Row is one result row with column order preserved (JSON output matches better-sqlite3 `.all()` objects).
type Row struct {
	Keys []string
	Vals []any
}

func (r *Row) Get(k string) any {
	for i, x := range r.Keys {
		if x == k {
			return r.Vals[i]
		}
	}
	return nil
}
func (r *Row) Set(k string, v any) {
	for i, x := range r.Keys {
		if x == k {
			r.Vals[i] = v
			return
		}
	}
	r.Keys = append(r.Keys, k)
	r.Vals = append(r.Vals, v)
}
func (r *Row) Del(k string) {
	for i, x := range r.Keys {
		if x == k {
			r.Keys = append(r.Keys[:i:i], r.Keys[i+1:]...)
			r.Vals = append(r.Vals[:i:i], r.Vals[i+1:]...)
			return
		}
	}
}
func (r *Row) Str(k string) string {
	switch v := r.Get(k).(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}
func (r *Row) Num(k string) float64 {
	switch v := r.Get(k).(type) {
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case float64:
		return v
	}
	return 0
}
func (r *Row) Has(k string) bool { return r.Get(k) != nil }

// DropNulls controls whether nil values are omitted (the API's JSON.stringify replacer `v === null ? undefined : v`).
type Rows struct {
	List      []*Row
	DropNulls bool
}

func (r *Row) encode(b *bytes.Buffer, dropNulls bool) {
	b.WriteByte('{')
	first := true
	for i, k := range r.Keys {
		v := r.Vals[i]
		if v == nil && dropNulls {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		writeVal(b, v, dropNulls)
	}
	b.WriteByte('}')
}

func writeVal(b *bytes.Buffer, v any, dropNulls bool) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case []byte:
		vb, _ := json.Marshal(string(x))
		b.Write(vb)
	case float64:
		b.WriteString(jsNum(x))
	case *Row:
		x.encode(b, dropNulls)
	case []*Row:
		b.WriteByte('[')
		for i, r := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			r.encode(b, dropNulls)
		}
		b.WriteByte(']')
	case json.RawMessage:
		b.Write(x)
	default:
		vb, _ := marshalNoEscape(x)
		b.Write(vb)
	}
}

// jsNum formats like JavaScript's Number#toString for the ranges we use.
func jsNum(f float64) string {
	if f == float64(int64(f)) && f < 1e21 && f > -1e21 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func (r *Row) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	r.encode(&b, false)
	return b.Bytes(), nil
}

func (rs Rows) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	writeVal(&b, rs.List, rs.DropNulls)
	if rs.List == nil {
		return []byte("[]"), nil
	}
	return b.Bytes(), nil
}

// marshalNoEscape is json.Marshal without HTML escaping (JSON.stringify does not escape <>&).
func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

type querier interface {
	Query(string, ...any) (*sql.Rows, error)
}

// all runs a query and returns ordered rows (int64 / float64 / string / nil values).
func all(db querier, q string, args ...any) ([]*Row, error) {
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	out := []*Row{}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out = append(out, &Row{Keys: append([]string(nil), cols...), Vals: vals})
	}
	return out, rs.Err()
}

func one(db querier, q string, args ...any) (*Row, error) {
	rs, err := all(db, q, args...)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	return rs[0], nil
}

// obj builds an ordered JSON object: obj("a", 1, "b", 2).
func obj(kv ...any) *Row {
	r := &Row{}
	for i := 0; i+1 < len(kv); i += 2 {
		r.Set(kv[i].(string), kv[i+1])
	}
	return r
}

type alsoX struct {
	URL    any `json:"url"`
	Author any `json:"author"`
}
