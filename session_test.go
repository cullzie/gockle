package gockle

import (
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/facebookgo/flagenv"
	"github.com/gocql/gocql"
	"github.com/maraino/go-mock"
)

const timeout = 5 * time.Second

const (
	ksCreate  = "create keyspace gockle_test with replication = {'class': 'SimpleStrategy', 'replication_factor': 1};"
	ksDrop    = "drop keyspace gockle_test"
	ksDropIf  = "drop keyspace if exists gockle_test"
	rowInsert = "insert into gockle_test.test (id, n) values (1, 2)"
	tabCreate = "create table gockle_test.test(id int primary key, n int)"
	tabDrop   = "drop table gockle_test.test"
)

var protoVersion = flag.Int("proto-version", 4, "CQL protocol version")

func TestNewSession(t *testing.T) {
	if a, e := NewSession(nil), (session{}); a != e {
		t.Errorf("Actual session %v, expected %v", a, e)
	}

	var c = gocql.NewCluster("localhost")

	c.ProtoVersion = *protoVersion
	c.Timeout = timeout

	var s, err = c.CreateSession()

	if err != nil {
		t.Skip(err)
	}

	if a, e := NewSession(s), (session{s: s}); a != e {
		t.Errorf("Actual session %v, expected %v", a, e)
	}
}

func TestSessionMetadata(t *testing.T) {
	var exec = func(s Session, q string) {
		if err := s.QueryExecute(q); err != nil {
			t.Errorf("Actual error %v, expected no error", err)
		}
	}

	var s = newSession(t)

	exec(s, ksDropIf)
	exec(s, ksCreate)
	exec(s, tabCreate)

	s = newSession(t)

	if a, err := s.Tables("gockle_test"); err == nil {
		if e := ([]string{"test"}); !reflect.DeepEqual(a, e) {
			t.Errorf("Actual tables %v, expected %v", a, e)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	if a, err := s.Columns("gockle_test", "test"); err == nil {
		var ts = map[string]gocql.Type{"id": gocql.TypeInt, "n": gocql.TypeInt}

		if la, le := len(a), len(ts); la == le {
			for n, at := range a {
				if et, ok := ts[n]; ok {
					if at.Type() != et {
						t.Errorf("Actual type %v, expected %v", at, et)
					}
				} else {
					t.Errorf("Actual name %v invalid, expected valid", n)
				}
			}
		} else {
			t.Errorf("Actual count %v, expected %v", la, le)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	exec(s, tabDrop)
	exec(s, ksDrop)

	s.Close()
}

func TestSessionMock(t *testing.T) {
	var m, e = &SessionMock{}, fmt.Errorf("e")

	testMock(t, m, &m.Mock, []struct {
		method    string
		arguments []interface{}
		results   []interface{}
	}{
		{"Close", nil, nil},
		{"Columns", []interface{}{"", ""}, []interface{}{map[string]gocql.TypeInfo(nil), nil}},
		{"Columns", []interface{}{"a", "b"}, []interface{}{map[string]gocql.TypeInfo{"c": gocql.NativeType{}}, e}},
		{"QueryBatch", []interface{}{BatchKind(0)}, []interface{}{(*batch)(nil)}},
		{"QueryBatch", []interface{}{BatchKind(1)}, []interface{}{&batch{}}},
		{"QueryExecute", []interface{}{"", []interface{}(nil)}, []interface{}{nil}},
		{"QueryExecute", []interface{}{"a", []interface{}{1}}, []interface{}{e}},
		{"QueryIterate", []interface{}{"", []interface{}(nil)}, []interface{}{(*iterator)(nil)}},
		{"QueryIterate", []interface{}{"a", []interface{}{1}}, []interface{}{iterator{}}},
		{"QueryScan", []interface{}{"", []interface{}(nil), []interface{}(nil)}, []interface{}{nil}},
		{"QueryScan", []interface{}{"a", []interface{}{1}, []interface{}{1}}, []interface{}{e}},
		{"QueryScanMap", []interface{}{"", []interface{}(nil), map[string]interface{}(nil)}, []interface{}{nil}},
		{"QueryScanMap", []interface{}{"a", []interface{}{1}, map[string]interface{}{"b": 2}}, []interface{}{e}},
		{"QueryScanMapTransaction", []interface{}{"", []interface{}(nil), map[string]interface{}(nil)}, []interface{}{false, nil}},
		{"QueryScanMapTransaction", []interface{}{"a", []interface{}{1}, map[string]interface{}{"b": 2}}, []interface{}{true, e}},
		{"QuerySliceMap", []interface{}{"", []interface{}(nil)}, []interface{}{[]map[string]interface{}(nil), nil}},
		{"QuerySliceMap", []interface{}{"a", []interface{}{1}}, []interface{}{[]map[string]interface{}{{"b": 2}}, e}},
		{"Tables", []interface{}{""}, []interface{}{[]string(nil), nil}},
		{"Tables", []interface{}{"a"}, []interface{}{[]string{"b"}, e}},
	})
}

func TestSessionQuery(t *testing.T) {
	var s = newSession(t)

	var exec = func(q string) {
		if err := s.QueryExecute(q); err != nil {
			t.Fatalf("Actual error %v, expected no error", err)
		}
	}

	exec(ksDropIf)
	exec(ksCreate)
	exec(tabCreate)
	exec(rowInsert)

	// QueryBatch
	if s.QueryBatch(BatchKind(0)) == nil {
		t.Error("Actual batch nil, expected not nil")
	}

	// QueryIterate
	if s.QueryIterate("select * from gockle_test.test") == nil {
		t.Error("Actual iterator nil, expected not nil")
	}

	// QueryScan
	var id, n int

	if err := s.QueryScan("select id, n from gockle_test.test", nil, []interface{}{&id, &n}); err == nil {
		if id != 1 {
			t.Errorf("Actual id %v, expected 1", id)
		}

		if n != 2 {
			t.Errorf("Actual n %v, expected 2", n)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	// QueryScanMap
	var am, em = map[string]interface{}{}, map[string]interface{}{"id": 1, "n": 2}

	if err := s.QueryScanMap("select id, n from gockle_test.test", nil, am); err == nil {
		if !reflect.DeepEqual(am, em) {
			t.Errorf("Actual map %v, expected %v", am, em)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	// QueryScanMapTransaction
	am = map[string]interface{}{}

	if b, err := s.QueryScanMapTransaction("update gockle_test.test set n = 3 where id = 1 if n = 2", nil, am); err == nil {
		if !b {
			t.Error("Actual applied false, expected true")
		}

		if l := len(am); l != 0 {
			t.Errorf("Actual length %v, expected 0", l)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	// QuerySliceMap
	var es = []map[string]interface{}{{"id": 1, "n": 3}}

	if as, err := s.QuerySliceMap("select * from gockle_test.test"); err == nil {
		if !reflect.DeepEqual(as, es) {
			t.Errorf("Actual rows %v, expected %v", as, es)
		}
	} else {
		t.Errorf("Actual error %v, expected no error", err)
	}

	exec(tabDrop)
	exec(ksDrop)

	s.Close()
}

func init() {
	flag.Parse()

	if err := flagenv.ParseSet("gockle_", flag.CommandLine); err != nil {
		panic(err)
	}
}

func newSession(t *testing.T) Session {
	var c = gocql.NewCluster("localhost")

	c.ProtoVersion = *protoVersion
	c.Timeout = timeout

	var s, err = c.CreateSession()

	if err != nil {
		t.Skip(err)
	}

	return NewSession(s)
}

func testMock(t *testing.T, i interface{}, m *mock.Mock, tests []struct {
	method    string
	arguments []interface{}
	results   []interface{}
}) {
	var v = reflect.ValueOf(i)

	for _, test := range tests {
		t.Log("Test:", test)
		m.Reset()
		m.When(test.method, test.arguments...).Return(test.results...)

		var vs []reflect.Value

		for _, a := range test.arguments {
			vs = append(vs, reflect.ValueOf(a))
		}

		var method = v.MethodByName(test.method)

		if method.Type().IsVariadic() {
			vs = method.CallSlice(vs)
		} else {
			vs = method.Call(vs)
		}

		var is []interface{}

		for _, v := range vs {
			is = append(is, v.Interface())
		}

		if !reflect.DeepEqual(is, test.results) {
			t.Errorf("Actual %v, expected %v", is, test.results)
		}
	}
}
