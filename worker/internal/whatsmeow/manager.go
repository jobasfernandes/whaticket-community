package whatsmeow

import (
	"context"
	"log/slog"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

type AdvancedSettings struct {
	AlwaysOnline  bool
	RejectCall    bool
	MsgRejectCall string
	ReadMessages  bool
	IgnoreGroups  bool
	IgnoreStatus  bool
}

type Session struct {
	ConnectionID int
	Client       *whatsmeow.Client
	HandlerID    uint32
	Settings     AdvancedSettings
	MediaMode    string
	Cancel       context.CancelFunc
}

type Manager struct {
	mu           sync.RWMutex
	sessions     map[int]*Session
	kills        map[int]chan struct{}
	connecting   map[int]bool
	handlerWG    sync.WaitGroup
	container    *sqlstore.Container
	shuttingDown bool
	log          *slog.Logger
}

func NewManager(container *sqlstore.Container, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		sessions:   make(map[int]*Session),
		kills:      make(map[int]chan struct{}),
		connecting: make(map[int]bool),
		container:  container,
		log:        log,
	}
}

func (m *Manager) Container() *sqlstore.Container {
	return m.container
}

func (m *Manager) Set(id int, sess *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.sessions[id]; ok && old.Client != nil && old.Client != sess.Client {
		old.Client.RemoveEventHandler(old.HandlerID)
		old.Client.Disconnect()
	} else {
		m.handlerWG.Add(1)
	}
	m.sessions[id] = sess
	delete(m.connecting, id)
}

func (m *Manager) Get(id int) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[id]
	return sess, ok
}

func (m *Manager) Delete(id int) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	if sess.Cancel != nil {
		sess.Cancel()
	}
	if sess.Client != nil {
		sess.Client.RemoveEventHandler(sess.HandlerID)
		sess.Client.Disconnect()
	}
	m.handlerWG.Done()
}

func (m *Manager) IsConnected(id int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[id]
	return ok && sess.Client != nil && sess.Client.IsConnected()
}

func (m *Manager) TryStartConnect(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown {
		return false
	}
	if m.connecting[id] {
		return false
	}
	if sess, ok := m.sessions[id]; ok && sess.Client != nil && sess.Client.IsConnected() {
		return false
	}
	m.connecting[id] = true
	return true
}

func (m *Manager) ClearConnecting(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connecting, id)
}

func (m *Manager) RegisterKill(id int) <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ch, ok := m.kills[id]; ok {
		return ch
	}
	ch := make(chan struct{})
	m.kills[id] = ch
	return ch
}

func (m *Manager) SendKill(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch, ok := m.kills[id]
	if !ok {
		return
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func (m *Manager) CleanupKill(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.kills, id)
}

func (m *Manager) IsShuttingDown() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shuttingDown
}

func (m *Manager) ForEach(fn func(int, *Session)) {
	m.mu.RLock()
	snapshot := make(map[int]*Session, len(m.sessions))
	for id, sess := range m.sessions {
		snapshot[id] = sess
	}
	m.mu.RUnlock()
	for id, sess := range snapshot {
		fn(id, sess)
	}
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *Manager) HandlerDone() {
	m.handlerWG.Done()
}

func (m *Manager) UpdateSettings(id int, settings AdvancedSettings, mediaMode string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[id]
	if !ok {
		return false
	}
	sess.Settings = settings
	sess.MediaMode = mediaMode
	return true
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.shuttingDown {
		m.mu.Unlock()
		return nil
	}
	m.shuttingDown = true
	for id, ch := range m.kills {
		select {
		case <-ch:
		default:
			close(ch)
		}
		delete(m.kills, id)
	}
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.Unlock()

	for _, sess := range sessions {
		if sess.Cancel != nil {
			sess.Cancel()
		}
		if sess.Client != nil {
			sess.Client.RemoveEventHandler(sess.HandlerID)
			sess.Client.Disconnect()
		}
	}

	done := make(chan struct{})
	go func() {
		m.handlerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
