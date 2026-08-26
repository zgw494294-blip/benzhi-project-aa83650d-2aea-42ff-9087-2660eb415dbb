package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"acousticverdictworkbench/internal/domain"
)

type Store struct {
	mu          sync.RWMutex
	dir         string
	batches     map[string]*domain.ReviewBatch
	idempotency map[string]idempotencyRecord
	sequence    uint64
	lastHash    string
	closed      bool
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o750); err != nil {
		return nil, err
	}
	if err := recoverPending(dir); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, batches: map[string]*domain.ReviewBatch{}, idempotency: map[string]idempotencyRecord{}}
	sequence, hash, err := scanAudit(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		return nil, err
	}
	s.sequence, s.lastHash = sequence, hash
	if err := s.loadSnapshots(); err != nil {
		return nil, err
	}
	if err := s.loadIdempotency(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadSnapshots() error {
	entries, err := os.ReadDir(filepath.Join(s.dir, "snapshots"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var envelope snapshotEnvelope
		if err := readJSON(filepath.Join(s.dir, "snapshots", entry.Name()), &envelope); err != nil {
			return fmt.Errorf("读取快照 %s：%w", entry.Name(), err)
		}
		if envelope.SchemaVersion != schemaVersion {
			return fmt.Errorf("快照 %s 的 schemaVersion 不受支持", entry.Name())
		}
		var batch domain.ReviewBatch
		if err := json.Unmarshal(envelope.Batch, &batch); err != nil {
			return fmt.Errorf("解析快照 %s：%w", entry.Name(), err)
		}
		if batch.ID == "" {
			return fmt.Errorf("快照 %s 缺少批次 ID", entry.Name())
		}
		s.batches[batch.ID] = &batch
	}
	return nil
}

func (s *Store) loadIdempotency() error {
	var file idempotencyFile
	err := readJSON(filepath.Join(s.dir, "idempotency.json"), &file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if file.SchemaVersion != schemaVersion {
		return fmt.Errorf("幂等记录 schemaVersion 不受支持")
	}
	for key, result := range file.Results {
		batch := s.batches[result.BatchID]
		var recorded domain.ReviewBatch
		if batch == nil || len(result.Batch) == 0 || json.Unmarshal(result.Batch, &recorded) != nil || recorded.ID != result.BatchID || recorded.Version != result.Version {
			return fmt.Errorf("幂等记录 %s 引用了无效版本", key)
		}
	}
	s.idempotency = file.Results
	return nil
}

func (s *Store) Get(_ context.Context, id string) (*domain.ReviewBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	batch := s.batches[id]
	if batch == nil {
		return nil, domain.ErrNotFound
	}
	return cloneBatch(batch)
}

func (s *Store) List(_ context.Context) ([]*domain.ReviewBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.batches))
	for id := range s.batches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*domain.ReviewBatch, 0, len(ids))
	for _, id := range ids {
		copy, err := cloneBatch(s.batches[id])
		if err != nil {
			return nil, err
		}
		result = append(result, copy)
	}
	return result, nil
}

func (s *Store) Idempotent(_ context.Context, key string) (*CommitResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.idempotency[key]
	if !ok {
		return nil, false
	}
	if s.batches[record.BatchID] == nil {
		return nil, false
	}
	var recorded domain.ReviewBatch
	if err := json.Unmarshal(record.Batch, &recorded); err != nil {
		return nil, false
	}
	return &CommitResult{Batch: &recorded}, true
}

func (s *Store) Commit(_ context.Context, request CommitRequest) (*CommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("仓储已关闭")
	}
	if request.Batch == nil || request.Operation == "" {
		return nil, domain.Invalid("commit", "提交缺少批次或操作名")
	}
	if request.IdempotencyKey != "" {
		if record, ok := s.idempotency[request.IdempotencyKey]; ok {
			var recorded domain.ReviewBatch
			if err := json.Unmarshal(record.Batch, &recorded); err != nil {
				return nil, err
			}
			return &CommitResult{Batch: &recorded}, nil
		}
	}
	current := s.batches[request.Batch.ID]
	if current == nil {
		if request.ExpectedVersion != 0 {
			return nil, domain.ErrVersionConflict
		}
	} else if current.Version != request.ExpectedVersion {
		return nil, domain.ErrVersionConflict
	}
	copy, err := cloneBatch(request.Batch)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(copy)
	if err != nil {
		return nil, err
	}
	envelope := snapshotEnvelope{SchemaVersion: schemaVersion, SavedAt: request.CommittedAt.UTC(), Batch: payload}
	records := make([]auditRecord, 0, len(request.Events))
	seq, previous := s.sequence, s.lastHash
	for _, event := range request.Events {
		seq++
		payload := auditPayload{SchemaVersion: schemaVersion, Sequence: seq, PreviousHash: previous, BatchID: copy.ID, BatchVersion: event.Version, Operation: request.Operation, OccurredAt: event.OccurredAt, EventType: event.Type, EventDetails: event.Details}
		hash, err := auditHash(payload)
		if err != nil {
			return nil, err
		}
		records = append(records, auditRecord{SchemaVersion: schemaVersion, Sequence: seq, PreviousHash: previous, Hash: hash, BatchID: copy.ID, BatchVersion: event.Version, Operation: request.Operation, OccurredAt: event.OccurredAt, EventType: event.Type, EventDetails: event.Details})
		previous = hash
	}
	newIdempotency := make(map[string]idempotencyRecord, len(s.idempotency)+1)
	for k, v := range s.idempotency {
		newIdempotency[k] = v
	}
	if request.IdempotencyKey != "" {
		stored, marshalErr := json.Marshal(copy)
		if marshalErr != nil {
			return nil, marshalErr
		}
		newIdempotency[request.IdempotencyKey] = idempotencyRecord{BatchID: copy.ID, Version: copy.Version, Batch: stored}
	}
	pending := pendingCommit{SchemaVersion: schemaVersion, BatchID: copy.ID, Snapshot: envelope, AuditRecords: records, Idempotency: idempotencyFile{SchemaVersion: schemaVersion, Results: newIdempotency}}
	if err := stageCommit(s.dir, pending); err != nil {
		return nil, err
	}
	if err := applyPending(s.dir, pending); err != nil {
		return nil, fmt.Errorf("应用持久化提交：%w", err)
	}
	s.batches[copy.ID], s.idempotency, s.sequence, s.lastHash = copy, newIdempotency, seq, previous
	result, _ := cloneBatch(copy)
	return &CommitResult{Batch: result}, nil
}

func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closed = true; return nil }
