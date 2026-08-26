package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

const pendingFileName = "pending-commit.json"

type pendingCommit struct {
	SchemaVersion int              `json:"schemaVersion"`
	BatchID       string           `json:"batchId"`
	Snapshot      snapshotEnvelope `json:"snapshot"`
	AuditRecords  []auditRecord    `json:"auditRecords"`
	Idempotency   idempotencyFile  `json:"idempotency"`
}

func stageCommit(dir string, pending pendingCommit) error {
	if err := atomicJSON(filepath.Join(dir, pendingFileName), pending); err != nil {
		return fmt.Errorf("保存提交意图：%w", err)
	}
	return nil
}

func applyPending(dir string, pending pendingCommit) error {
	if pending.SchemaVersion != schemaVersion || pending.BatchID == "" || pending.Snapshot.SchemaVersion != schemaVersion {
		return fmt.Errorf("未完成提交的 schemaVersion 或批次 ID 无效")
	}
	sequence, hash, err := scanAudit(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		return err
	}
	start := 0
	for start < len(pending.AuditRecords) && pending.AuditRecords[start].Sequence <= sequence {
		start++
	}
	if start < len(pending.AuditRecords) {
		next := pending.AuditRecords[start]
		if next.Sequence != sequence+1 || next.PreviousHash != hash {
			return fmt.Errorf("未完成提交无法接续当前审计链")
		}
		if err := appendAudit(filepath.Join(dir, "audit.jsonl"), pending.AuditRecords[start:]); err != nil {
			return err
		}
	} else if len(pending.AuditRecords) > 0 && sequence > pending.AuditRecords[len(pending.AuditRecords)-1].Sequence {
		return fmt.Errorf("未完成提交落后于当前审计链")
	}
	if err := atomicJSON(filepath.Join(dir, "snapshots", pending.BatchID+".json"), pending.Snapshot); err != nil {
		return err
	}
	if err := atomicJSON(filepath.Join(dir, "idempotency.json"), pending.Idempotency); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, pendingFileName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func recoverPending(dir string) error {
	var pending pendingCommit
	err := readJSON(filepath.Join(dir, pendingFileName), &pending)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取未完成提交：%w", err)
	}
	if err := applyPending(dir, pending); err != nil {
		return fmt.Errorf("恢复未完成提交：%w", err)
	}
	return nil
}
