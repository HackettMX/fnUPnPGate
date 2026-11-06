package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Mapping 是本应用管理的端口映射（持久化的部分）。
type Mapping struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	Protocol       string `json:"protocol"`
	ExternalPort   int    `json:"external_port"`
	InternalPort   int    `json:"internal_port"`
	InternalClient string `json:"internal_client"`
	LeaseTime      int    `json:"lease_time"`
	CreatedAt      string `json:"created_at"`
}

type storeData struct {
	Mappings []Mapping `json:"mappings"`
}

// Store 提供端口映射配置的并发安全 JSON 持久化。
// 写入采用「临时文件 + 重命名」保证原子性，避免断电产生半截文件。
type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

func NewStore(path string) *Store {
	s := &Store{path: path}
	s.load()
	return s
}

func (s *Store) load() {
	s.data = storeData{Mappings: []Mapping{}}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var loaded storeData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// 文件损坏时以空配置启动，不阻断服务。
		return
	}
	if loaded.Mappings == nil {
		loaded.Mappings = []Mapping{}
	}
	s.data = loaded
}

// saveLocked 必须在持有写锁时调用。
func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) All() []Mapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Mapping, len(s.data.Mappings))
	copy(out, s.data.Mappings)
	return out
}

func (s *Store) Get(id string) (Mapping, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.data.Mappings {
		if m.ID == id {
			return m, true
		}
	}
	return Mapping{}, false
}

func (s *Store) Add(m Mapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Mappings = append(s.data.Mappings, m)
	return s.saveLocked()
}

func (s *Store) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Mapping, 0, len(s.data.Mappings))
	removed := false
	for _, m := range s.data.Mappings {
		if m.ID == id {
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		return false
	}
	s.data.Mappings = kept
	_ = s.saveLocked()
	return true
}

// RemoveByPort 按外部端口 + 协议移除本应用记录（用于清理外部条目时同步本地）。
func (s *Store) RemoveByPort(externalPort int, protocol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Mapping, 0, len(s.data.Mappings))
	changed := false
	for _, m := range s.data.Mappings {
		if m.ExternalPort == externalPort && m.Protocol == protocol {
			changed = true
			continue
		}
		kept = append(kept, m)
	}
	if changed {
		s.data.Mappings = kept
		_ = s.saveLocked()
	}
}

// nowISO 返回带本地时区偏移的 ISO8601 时间（秒级），与前端展示一致。
func nowISO() string {
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}
