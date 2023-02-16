package sessions

import (
	"log"
	"net/http"

	ginsessions "github.com/gin-contrib/sessions"
	gsessions "github.com/gorilla/sessions"
)

type Store interface {
	gsessions.Store
	Options(ginsessions.Options)
}

type Session struct {
	name    string
	request *http.Request
	store   Store
	session *gsessions.Session
	written bool
	writer  http.ResponseWriter
}

func NewSession(name string, req *http.Request, store Store, writer http.ResponseWriter) *Session {
	return &Session{name, req, store, nil, false, writer}
}

func (s *Session) ID() string {
	return s.GetSession().ID
}

func (s *Session) Get(key interface{}) interface{} {
	return s.GetSession().Values[key]
}

func (s *Session) Set(key interface{}, val interface{}) {
	s.GetSession().Values[key] = val
	s.written = true
}

func (s *Session) Delete(key interface{}) {
	delete(s.GetSession().Values, key)
	s.written = true
}

func (s *Session) Clear() {
	for key := range s.GetSession().Values {
		s.Delete(key)
	}
}

func (s *Session) AddFlash(value interface{}, vars ...string) {
	s.GetSession().AddFlash(value, vars...)
	s.written = true
}

func (s *Session) Flashes(vars ...string) []interface{} {
	s.written = true
	return s.GetSession().Flashes(vars...)
}

func (s *Session) Options(options ginsessions.Options) {
	s.written = true
	s.GetSession().Options = options.ToGorillaOptions()
}

func (s *Session) Save() error {
	if s.Written() {
		e := s.GetSession().Save(s.request, s.writer)
		if e == nil {
			s.written = false
		}
		return e
	}
	return nil
}

func (s *Session) GetSession() *gsessions.Session {
	if s.session == nil {
		var err error
		s.session, err = s.store.Get(s.request, s.name)
		if err != nil {
			log.Printf("[sessions] ERROR! %s\n", err)
		}
	}
	return s.session
}

func (s *Session) Written() bool {
	return s.written
}
