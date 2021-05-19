package store

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/webx-top/echo"
	"github.com/webx-top/echo/defaults"
	"github.com/webx-top/echo/engine/mock"

	"github.com/admpub/boltstore/shared"
	"github.com/admpub/securecookie"
	"github.com/admpub/sessions"
	"github.com/boltdb/bolt"
)

var benchmarkDB = fmt.Sprintf("benchmark_store_%d.db", time.Now().Unix())

func init() {
	if os.Getenv("CREATE_BENCHDATA") != "true" {
		return
	}

	// Put data to the database for the benchmark.
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(shared.DefaultBucketName))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	data, err := proto.Marshal(shared.NewSession([]byte{}, shared.DefaultMaxAge))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Start putting data for the benchmark %+v\n", time.Now())

	for i := 0; i < 100; i++ {
		err = db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(shared.DefaultBucketName))
			for j := 0; j < 100000; j++ {
				if err := bucket.Put([]byte(strconv.Itoa(100000*i+j)), data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
		if (i+1)%10 == 0 {
			fmt.Printf("%d key-values were put.\n", (i+1)*100000)
		}
	}

	fmt.Printf("End putting data for the benchmark %+v\n", time.Now())
	if err != nil {
		panic(err)
	}
}

func TestStore_Get(t *testing.T) {
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		t.Error(err)
	}

	session, err := str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	if session.IsNew != true {
		t.Errorf("session.IsNew should be true (actual: %+v)", session.IsNew)
	}
}

func TestStore_New(t *testing.T) {
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)
	ctx.SessionOptions().MaxAge = 1024

	encoded, err := securecookie.EncodeMulti("test", "1", str.codecs...)

	req.AddCookie(sessions.NewCookie(ctx, "test", encoded))

	session, err := str.New(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	if session.IsNew != true {
		t.Errorf("session.IsNew should be true (actual: %+v)", session.IsNew)
	}
}

func TestStore_Save(t *testing.T) {
	// When ctx.SessionOptions().MaxAge < 0
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		t.Error(err)
	}
	w := httptest.NewRecorder()

	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	session, err := str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	ctx.SessionOptions().MaxAge = -1

	if err := str.Save(ctx, session); err != nil {
		t.Error(err)
	}

	// When ctx.SessionOptions().MaxAge >= 0
	session, err = str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	ctx.SessionOptions().MaxAge = 1

	w = httptest.NewRecorder()

	if err := str.Save(ctx, session); err != nil {
		t.Error(err)
	}

	// When ctx.SessionOptions().MaxAge >= 0 and
	// s.save returns an error
	session.Values = make(map[interface{}]interface{})
	session.Values[make(chan int)] = make(chan int)
	if err := str.Save(ctx, session); err == nil || err.Error() != "gob: type not registered for interface: chan int" {
		t.Errorf(`str.Save should return an error "%s" (actual: %+v)`, "gob: type not registered for interface: chan int", err)
	}

	// When ctx.SessionOptions().MaxAge >= 0 and
	// securecookie.EncodeMulti  returns an error
	session.Values = make(map[interface{}]interface{})
	str.codecs = nil
	if err := str.Save(ctx, session); err == nil || err.Error() != "securecookie: no codecs provided" {
		t.Errorf(`str.Save should return an error "%s" (actual: %+v)`, "securecookie: no codecs provided", err)
	}
}

func TestStore_load(t *testing.T) {
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	session, err := str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	if err := str.Save(ctx, session); err != nil {
		t.Error(err)
	}

	exists, err := str.load(session)
	if err != nil {
		t.Error(err)
	}

	if exists != true {
		t.Error("Store.load should return true (actual: false)")
	}

	// When the target session data is nil
	session.ID = "x"
	exists, err = str.load(session)
	if err != nil {
		t.Error(err)
	}

	if exists != false {
		t.Error("Store.load should return false (actual: true)")
	}

	// When shared.Session returns an error
	err = db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(str.config.DBOptions.BucketName).Put([]byte("x"), []byte("test"))
	})
	if err != nil {
		t.Error(err)
	}
	_, err = str.load(session)
	if err == nil || err.Error() != "proto: protobuf.Session: wiretype end group for non-group" {
		t.Errorf(`str.load should return an error "%s" (actual: %s)`, "proto: protobuf.Session: wiretype end group for non-group", err)
	}

	// When the target session data is expired
	session, err = str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}
	ctx.SessionOptions().MaxAge = 0
	if err := str.Save(ctx, session); err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second)
	_, err = str.load(session)
	if err != nil {
		t.Error(err)
	}
}

func TestSession_delete(t *testing.T) {
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		t.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		t.Error(err)
	}

	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	session, err := str.Get(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	db.Close()

	err = str.delete(session)

	if err.Error() != "database not open" {
		t.Errorf(`str.delete should return an error "%s" (actual: %s)`, "database not open", err)
	}
}

func TestNew(t *testing.T) {
	// When db.Update returns an error
	db, err := bolt.Open("./sessions.db", 0666, nil)
	if err != nil {
		t.Error(err)
	}
	db.Close()

	_, err = New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err.Error() != "database not open" {
		t.Errorf(`str.delete  should return an error "%s" (actual: %s)`, "database not open", err)
	}
}

func ExampleStore_Get() {
	// db(*bolt.DB) should be opened beforehand and passed by the other function.
	var db *bolt.DB

	// r(*http.Request) should be passed by the other function.
	var r *http.Request

	// Create a store.
	str, err := New(db, Config{}, []byte("secret-key"))
	if err != nil {
		panic(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(r), mock.NewResponse(w), defaults.Default)

	// Get a session.
	session, err := str.Get(ctx, "session-key")
	if err != nil {
		panic(err)
	}

	// Add a value on the session.
	session.Values["foo"] = "bar"
}

func ExampleStore_New() {
	// db(*bolt.DB) should be opened beforehand and passed by the other function.
	var db *bolt.DB

	// r(*http.Request) should be passed by the other function.
	var r *http.Request

	// Create a store.
	str, err := New(db, Config{}, []byte("secret-key"))
	if err != nil {
		panic(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(r), mock.NewResponse(w), defaults.Default)

	// Create a session.
	session, err := str.New(ctx, "session-key")
	if err != nil {
		panic(err)
	}

	// Add a value on the session.
	session.Values["foo"] = "bar"
}

func ExampleStore_Save() {
	// db(*bolt.DB) should be opened beforehand and passed by the other function.
	var db *bolt.DB

	// w(http.ResponseWriter) should be passed by the other function.
	var w http.ResponseWriter

	// r(*http.Request) should be passed by the other function.
	var r *http.Request

	// Create a store.
	str, err := New(db, Config{}, []byte("secret-key"))
	if err != nil {
		panic(err)
	}

	ctx := echo.NewContext(mock.NewRequest(r), mock.NewResponse(w), defaults.Default)

	// Create a session.
	session, err := str.New(ctx, "session-key")
	if err != nil {
		panic(err)
	}

	// Add a value on the session.
	session.Values["foo"] = "bar"

	// Save the session.
	if err := sessions.Save(ctx); err != nil {
		panic(err)
	}

	// You can delete the session by setting the session options's MaxAge
	// to a minus value.
	ctx.SessionOptions().MaxAge = -1
	if err := sessions.Save(ctx); err != nil {
		panic(err)
	}
}

func ExampleNew() {
	// db should be opened beforehand and passed by the other function.
	var db *bolt.DB

	// r(*http.Request) should be passed by the other function.
	var r *http.Request

	// Create a store.
	str, err := New(db, Config{}, []byte("secret-key"))
	if err != nil {
		panic(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(r), mock.NewResponse(w), defaults.Default)

	// Get a session.
	session, err := str.Get(ctx, "session-key")
	if err != nil {
		panic(err)
	}

	// Add a value on the session.
	session.Values["foo"] = "bar"
}

func BenchmarkNew(b *testing.B) {
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		b.Error(err)
	}

	defer db.Close()

	for i := 0; i < b.N; i++ {
		_, err = New(
			db,
			Config{},
			[]byte("secret-key"),
		)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkStore_Get(b *testing.B) {
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		b.Error(err)
	}

	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		b.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		b.Error(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	for i := 0; i < b.N; i++ {
		_, err := str.Get(ctx, "test")
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkStore_New(b *testing.B) {
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		b.Error(err)
	}

	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		b.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		b.Error(err)
	}
	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	for i := 0; i < b.N; i++ {
		_, err := str.New(ctx, "test")
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkStore_Save(b *testing.B) {
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		b.Error(err)
	}

	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		b.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		b.Error(err)
	}

	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	session, err := str.Get(ctx, "test")
	if err != nil {
		b.Error(err)
	}

	session.Values["foo"] = "bar"

	for i := 0; i < b.N; i++ {
		if err := str.Save(ctx, session); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkStore_Save_delete(b *testing.B) {
	db, err := bolt.Open(benchmarkDB, 0666, nil)
	if err != nil {
		b.Error(err)
	}

	defer db.Close()

	str, err := New(
		db,
		Config{},
		[]byte("secret-key"),
	)
	if err != nil {
		b.Error(err)
	}

	req, err := http.NewRequest("GET", "http://localhost:3000/", nil)
	if err != nil {
		b.Error(err)
	}

	w := httptest.NewRecorder()
	ctx := echo.NewContext(mock.NewRequest(req), mock.NewResponse(w), defaults.Default)

	session, err := str.Get(ctx, "test")
	if err != nil {
		b.Error(err)
	}

	session.Values["foo"] = "bar"

	ctx.SessionOptions().MaxAge = -1

	for i := 0; i < b.N; i++ {
		if err := str.Save(ctx, session); err != nil {
			b.Error(err)
		}
	}
}
