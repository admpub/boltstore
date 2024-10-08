package store

import (
	"bytes"
	"encoding/base32"
	"encoding/gob"
	"strings"

	"github.com/gogo/protobuf/proto"

	"github.com/admpub/boltstore/shared"
	"github.com/admpub/securecookie"
	"github.com/admpub/sessions"
	"github.com/webx-top/echo"
	bolt "go.etcd.io/bbolt"
)

// Store represents a session store.
type Store struct {
	codecs []securecookie.Codec
	config Config
	db     *bolt.DB
}

// Get returns a session for the given name after adding it to the registry.
//
// See gorilla/sessions FilesystemStore.Get().
func (s *Store) Get(ctx echo.Context, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(ctx).Get(s, name)
}

// New returns a session for the given name without adding it to the registry.
//
// See gorilla/sessions FilesystemStore.New().
func (s *Store) New(ctx echo.Context, name string) (*sessions.Session, error) {
	var err error
	session := sessions.NewSession(s, name)
	session.IsNew = true
	if v := ctx.GetCookie(name); len(v) > 0 {
		err = securecookie.DecodeMultiWithMaxAge(name, v, &session.ID, ctx.CookieOptions().MaxAge, s.codecs...)
		if err == nil {
			ok, err := s.load(session)
			session.IsNew = !(err == nil && ok) // not new if no error and data available
		}
	}
	return session, err
}

func (s *Store) Reload(ctx echo.Context, session *sessions.Session) error {
	ok, err := s.load(session)
	session.IsNew = !(err == nil && ok) // not new if no error and data available
	return err
}

// Save adds a single session to the response.
func (s *Store) Save(ctx echo.Context, session *sessions.Session) error {
	if ctx.CookieOptions().MaxAge < 0 {
		s.delete(session)
		sessions.SetCookie(ctx, session.Name(), "")
	} else {
		// Build an alphanumeric ID.
		if len(session.ID) == 0 {
			session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
		}
		if err := s.save(ctx, session); err != nil {
			return err
		}
		encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
		if err != nil {
			return err
		}
		sessions.SetCookie(ctx, session.Name(), encoded)
	}
	return nil
}

// load loads a session data from the database.
// True is returned if there is a session data in the database.
func (s *Store) load(session *sessions.Session) (bool, error) {
	// exists represents whether a session data exists or not.
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		id := []byte(session.ID)
		bucket := tx.Bucket(s.config.DBOptions.BucketName)
		// Get the session data.
		data := bucket.Get(id)
		if data == nil {
			return nil
		}
		sessionData, err := shared.Session(data)
		if err != nil {
			return err
		}
		// Check the expiration of the session data.
		if shared.Expired(sessionData) {
			err := s.db.Update(func(txu *bolt.Tx) error {
				return txu.Bucket(s.config.DBOptions.BucketName).Delete(id)
			})
			return err
		}
		exists = true
		dec := gob.NewDecoder(bytes.NewBuffer(sessionData.Values))
		return dec.Decode(&session.Values)
	})
	return exists, err
}

// Remove removes the key-value from the database.
func (s *Store) Remove(sessionID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.config.DBOptions.BucketName).Delete([]byte(sessionID))
	})
}

// delete removes the key-value from the database.
func (s *Store) delete(session *sessions.Session) error {
	return s.Remove(session.ID)
}

// save stores the session data in the database.
func (s *Store) save(ctx echo.Context, session *sessions.Session) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(session.Values)
	if err != nil {
		return err
	}
	maxAge := ctx.CookieOptions().MaxAge
	if maxAge == 0 {
		maxAge = shared.DefaultMaxAge
	}
	data, err := proto.Marshal(shared.NewSession(buf.Bytes(), maxAge))
	if err != nil {
		return err
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.config.DBOptions.BucketName).Put([]byte(session.ID), data)
	})
	return err
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default for a new FilesystemStore is 4096.
func (s *Store) MaxLength(l int) {
	securecookie.SetMaxLength(s.codecs, l)
}

// New creates and returns a session store.
func New(db *bolt.DB, config Config, keyPairs ...[]byte) (*Store, error) {
	config.setDefault()
	store := &Store{
		codecs: securecookie.CodecsFromPairs(keyPairs...),
		config: config,
		db:     db,
	}
	if config.MaxLength > 0 {
		store.MaxLength(config.MaxLength)
	}
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(config.DBOptions.BucketName)
		return err
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}
