package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type clientPolicySnapshot struct {
	UpdatedAt    time.Time `json:"updated_at"`
	BlockedSites []string  `json:"blocked_sites"`
	BlockedApps  []string  `json:"blocked_apps"`
}

type mandatoryNoticeRecord struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Active    bool       `json:"active"`
}

type mandatoryNoticeList struct {
	UpdatedAt time.Time               `json:"updated_at"`
	Notices   []mandatoryNoticeRecord `json:"notices"`
}

type clientPolicyStore struct {
	rootPath   string
	blockPath  string
	noticePath string
	mu         sync.Mutex
}

func newClientPolicyStore(storageRoot string) *clientPolicyStore {
	root := filepath.Join(storageRoot, "client-policy")
	return &clientPolicyStore{
		rootPath:   root,
		blockPath:  filepath.Join(root, "blocklist.json"),
		noticePath: filepath.Join(root, "mandatory_notices.json"),
	}
}

func (s *clientPolicyStore) LoadBlocklist() (clientPolicySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return clientPolicySnapshot{}, err
	}
	snapshot, err := s.readBlocklistLocked()
	if err != nil {
		return clientPolicySnapshot{}, err
	}
	return snapshot, nil
}

func (s *clientPolicyStore) SaveBlocklist(blockedSites []string, blockedApps []string) (clientPolicySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return clientPolicySnapshot{}, err
	}
	snapshot := clientPolicySnapshot{
		UpdatedAt:    time.Now().UTC(),
		BlockedSites: normalizePolicyEntries(blockedSites),
		BlockedApps:  normalizePolicyEntries(blockedApps),
	}
	if err := writeJSONFileAtomic(s.blockPath, snapshot); err != nil {
		return clientPolicySnapshot{}, err
	}
	return snapshot, nil
}

func (s *clientPolicyStore) ListNotices(includeInactive bool) ([]mandatoryNoticeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return nil, err
	}
	list, err := s.readNoticesLocked()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := make([]mandatoryNoticeRecord, 0, len(list.Notices))
	for _, notice := range list.Notices {
		active := notice.Active
		if notice.ExpiresAt != nil && !notice.ExpiresAt.After(now) {
			active = false
		}
		notice.Active = active
		if includeInactive || active {
			result = append(result, notice)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (s *clientPolicyStore) CreateNotice(title string, message string, expiresAfter time.Duration) (mandatoryNoticeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return mandatoryNoticeRecord{}, err
	}
	list, err := s.readNoticesLocked()
	if err != nil {
		return mandatoryNoticeRecord{}, err
	}

	now := time.Now().UTC()
	notice := mandatoryNoticeRecord{
		ID:        newNoticeID(),
		Title:     strings.TrimSpace(title),
		Message:   strings.TrimSpace(message),
		CreatedAt: now,
		Active:    true,
	}
	if expiresAfter > 0 {
		expiresAt := now.Add(expiresAfter)
		notice.ExpiresAt = &expiresAt
	}
	list.UpdatedAt = now
	list.Notices = append(list.Notices, notice)

	if err := writeJSONFileAtomic(s.noticePath, list); err != nil {
		return mandatoryNoticeRecord{}, err
	}
	return notice, nil
}

func (s *clientPolicyStore) DeactivateNotice(id string) (mandatoryNoticeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureRoot(); err != nil {
		return mandatoryNoticeRecord{}, err
	}
	list, err := s.readNoticesLocked()
	if err != nil {
		return mandatoryNoticeRecord{}, err
	}

	needle := strings.TrimSpace(id)
	if needle == "" {
		return mandatoryNoticeRecord{}, fmt.Errorf("notice id is required")
	}

	for index, notice := range list.Notices {
		if notice.ID != needle {
			continue
		}
		list.Notices[index].Active = false
		list.UpdatedAt = time.Now().UTC()
		if err := writeJSONFileAtomic(s.noticePath, list); err != nil {
			return mandatoryNoticeRecord{}, err
		}
		return list.Notices[index], nil
	}
	return mandatoryNoticeRecord{}, fmt.Errorf("notice %q not found", needle)
}

func (s *clientPolicyStore) ensureRoot() error {
	return os.MkdirAll(s.rootPath, 0o700)
}

func (s *clientPolicyStore) readBlocklistLocked() (clientPolicySnapshot, error) {
	payload, err := os.ReadFile(s.blockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return clientPolicySnapshot{
				UpdatedAt:    time.Time{},
				BlockedSites: []string{},
				BlockedApps:  []string{},
			}, nil
		}
		return clientPolicySnapshot{}, err
	}

	var snapshot clientPolicySnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return clientPolicySnapshot{}, err
	}
	snapshot.BlockedSites = normalizePolicyEntries(snapshot.BlockedSites)
	snapshot.BlockedApps = normalizePolicyEntries(snapshot.BlockedApps)
	return snapshot, nil
}

func (s *clientPolicyStore) readNoticesLocked() (mandatoryNoticeList, error) {
	payload, err := os.ReadFile(s.noticePath)
	if err != nil {
		if os.IsNotExist(err) {
			return mandatoryNoticeList{
				UpdatedAt: time.Time{},
				Notices:   []mandatoryNoticeRecord{},
			}, nil
		}
		return mandatoryNoticeList{}, err
	}

	var list mandatoryNoticeList
	if err := json.Unmarshal(payload, &list); err != nil {
		return mandatoryNoticeList{}, err
	}
	if list.Notices == nil {
		list.Notices = []mandatoryNoticeRecord{}
	}
	return list, nil
}

func writeJSONFileAtomic(path string, payload any) error {
	tempPath := path + ".tmp"
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func normalizePolicyEntries(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		normalized := strings.TrimSpace(raw)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}

func newNoticeID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("notice-%d", time.Now().UnixNano())
	}
	return "notice-" + hex.EncodeToString(buffer)
}
