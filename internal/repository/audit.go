package repository

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func auditHash(payload auditPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func scanAudit(path string) (uint64, string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var sequence uint64
	previous := ""
	line := 0
	for scanner.Scan() {
		line++
		var record auditRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return 0, "", fmt.Errorf("审计日志第 %d 行损坏：%w", line, err)
		}
		if record.SchemaVersion != schemaVersion || record.Sequence != sequence+1 || record.PreviousHash != previous {
			return 0, "", fmt.Errorf("审计日志第 %d 行序号或前序摘要不连续", line)
		}
		payload := auditPayload{SchemaVersion: record.SchemaVersion, Sequence: record.Sequence, PreviousHash: record.PreviousHash, BatchID: record.BatchID, BatchVersion: record.BatchVersion, Operation: record.Operation, OccurredAt: record.OccurredAt, EventType: record.EventType, EventDetails: record.EventDetails}
		hash, err := auditHash(payload)
		if err != nil || hash != record.Hash {
			return 0, "", fmt.Errorf("审计日志第 %d 行摘要不匹配", line)
		}
		sequence, previous = record.Sequence, record.Hash
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return 0, "", err
	}
	return sequence, previous, nil
}

func appendAudit(path string, records []auditRecord) error {
	if len(records) == 0 {
		return nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
