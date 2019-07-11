package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

var testOutput = `
import (
	"testing"
	"time"
)

func NewKey(i int) {{ .Key}} {
	panic("implement me")
}

func NewValue(i int) {{ .Value}} {
	panic("implement me")
}

func ValuesEqualityCheck(x, y {{ .Value}}) bool {
	panic("implement me")
}

func TestExpireMap_Get(t *testing.T) {
	const N = 1000
	expireMap := New()
	defer expireMap.Close()

	keys := make([]{{ .Key}}, N, N)
	values := make([]{{ .Value}}, N, N)

	for i := 0; i < N; i++ {
		keys[i] = NewKey(i)
		values[i] = NewValue(i)
		expireMap.Set(keys[i], values[i], time.Second)
	}

	for i := 0; i < N; i++ {
		if v, ok := expireMap.Get(keys[i]); !ok || !ValuesEqualityCheck(v, values[i]) {
			t.Errorf("Get() - got %v %v, want %v %v", v, ok, values[i], true)
			t.FailNow()
		}
	}
	time.Sleep(time.Second)

	for i := 0; i < N; i++ {
		if v, ok := expireMap.Get(keys[i]); ok || !ValuesEqualityCheck(v, {{ .ValueZero}}) {
			t.Errorf("Get() - got %v %v, want %v %v", v, ok, {{ .ValueZero}}, false)
			t.FailNow()
		}
	}
}

func TestExpireMap_Set(t *testing.T) {
	const N = 10
	expireMap := New()
	defer expireMap.Close()

	key := NewKey(0)

	for i := 0; i < N; i++ {
		value := NewValue(i)
		expireMap.Set(key, value, time.Second)
		if v, ok := expireMap.Get(key); !ok || !ValuesEqualityCheck(v, value) {
			t.Errorf("Get() - got %v %v, want %v %v", v, ok, value, true)
			t.FailNow()
		}
		if v := expireMap.Size(); v != 1 {
			t.Error("Size() - got wrong size")
			t.FailNow()
		}
	}
}

func TestExpireMap_GetAndSetTTL(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()
	key := NewKey(0)
	value := NewValue(0)

	expireMap.Set(key, value, time.Minute)
	if ttl := time.Duration(expireMap.GetTTL(key)); ttl >= time.Minute + time.Second || ttl <= time.Minute - time.Second  {
		t.Errorf("GetTTL() - something wrong with ttl - got %v", ttl)
		t.FailNow()
	}
	expireMap.SetTTL(key, time.Hour)
	if ttl := time.Duration(expireMap.GetTTL(key)); ttl >= time.Hour + time.Second || ttl <= time.Hour - time.Second  {
		t.Errorf("GetTTL() - something wrong with ttl - got %v", ttl)
		t.FailNow()
	}
	expireMap.SetTTL(key, 0)
	if ttl := expireMap.GetTTL(key); ttl != 0 {
		t.Errorf("GetTTL() - got %v, want 0", ttl)
		t.FailNow()
	}
}

func TestExpireMap_Delete(t *testing.T) {
	const N = 1000
	expireMap := New()
	defer expireMap.Close()

	keys := make([]{{ .Key}}, N, N)

	for i := 0; i < N; i++ {
		keys[i] = NewKey(i)
		expireMap.Set(keys[i], NewValue(i), time.Second)
	}
	if expireMap.Size() != N {
		t.Error("Size() - size mismatch")
	}
	for i := 0; i < N; i++ {
		expireMap.Delete(keys[i])
	}

	if expireMap.Size() != 0 {
		t.Error("Size() - size mismatch")
	}

	for i := 0; i < N; i++ {
		if v, ok := expireMap.Get(keys[i]); ok || !ValuesEqualityCheck(v, {{ .ValueZero}}) {
			t.Errorf("Get() - got %v %v, want %v %v", v, ok, {{ .ValueZero}}, false)
			t.FailNow()
		}
	}
}

func TestExpireMap_GetAll(t *testing.T) {
	const N = 1000
	expireMap := New()
	defer expireMap.Close()

	keys := make([]{{ .Key}}, N, N)
	values := make([]{{ .Value}}, N, N)

	for i := 0; i < N; i++ {
		keys[i] = NewKey(i)
		values[i] = NewValue(i)
		expireMap.Set(keys[i], values[i], time.Second)
	}

	kvs := expireMap.GetAll()

	if len(kvs) != N {
		t.Error("GetAll() - length mismatch")
		t.FailNow()
	}

	for i := 0; i < N; i++ {
		found := false
		for j := 0; j < N; j++ {
			if kvs[j].Key == keys[i] && ValuesEqualityCheck(kvs[j].Value, values[i]) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetAll() - did not found %v %v key-value pair", keys[i], values[i])
			t.FailNow()
		}
	}
	time.Sleep(time.Second)

	kvs = expireMap.GetAll()

	if len(kvs) != 0 {
		t.Error("GetAll() - length mismatch")
		t.FailNow()
	}
}

{{if .Notify}}
func TestExpireMap_Notify(t *testing.T) {
	expireMap := New()
	defer expireMap.Close()

	c := make(chan Event, 10)
	
	expireMap.Notify(c, AllEvents)
	key1 := NewKey(1)
	key2 := NewKey(2)
	value1 := NewValue(1)
	value2 := NewValue(2)
	
	expireMap.Set(key1, value1, time.Second)

	if e := <-c; e.Key != key1 || !ValuesEqualityCheck(e.Value, value1) || e.Type != Set {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Set(key1, value2, time.Second)

	if e := <-c; e.Key != key1 || !ValuesEqualityCheck(e.Value, value2) || e.Type != Update {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Set(key2, value2, time.Second)

	if e := <-c; e.Key != key2 || !ValuesEqualityCheck(e.Value, value2) || e.Type != Set {
		t.Error("Notify() - got wrong value")
	}

	expireMap.Delete(key2)

	if e := <-c; e.Key != key2 ||!ValuesEqualityCheck(e.Value, value2) || e.Type != Delete {
		t.Error("Notify() - got wrong value")
	}

	time.Sleep(time.Second)

	if e := <-c; e.Key != key1 || !ValuesEqualityCheck(e.Value, value2) || e.Type != Expire {
		t.Error("Notify() - got wrong value")
	}
}
{{- end}}

`

func main() {
	r := struct {
		Key       string
		Value     string
		ValueZero string
		Notify    bool
	}{}

	flag.StringVar(&r.Key, "k", "interface{}", "name of key struct")
	flag.StringVar(&r.Value, "v", "interface{}", "name of value struct")
	flag.StringVar(&r.ValueZero, "zv", "", "zero value for values (for pointers - type nil, struct T - type &T{})")
	flag.BoolVar(&r.Notify, "n", true, "presence of Notify method")
	flag.Parse()

	if r.ValueZero == "" {
		if v := r.Value; v == "interface{}" || strings.HasPrefix(v, "*") {
			r.ValueZero = "nil"
		} else {
			r.ValueZero = r.Value + "{}"
		}
	}
	fmt.Fprintf(os.Stderr, "Key - %s, Value - %s, Zero Value - %s, Use notify - %t\nWriting results to stdout\n", r.Key, r.Value, r.ValueZero, r.Notify)

	t, err := template.New("output").Parse(testOutput)

	if err != nil {
		log.Fatal(err)
	}

	err = t.Execute(os.Stdout, r)

	if err != nil {
		log.Fatal(err)
	}
}
