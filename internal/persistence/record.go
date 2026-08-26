package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"phonemereleasedesk/internal/domain"
)

const schemaVersion = 1

type ledgerRecord struct {
	SchemaVersion int          `json:"schemaVersion"`
	Sequence      uint64       `json:"sequence"`
	PreviousHash  string       `json:"previousHash"`
	Hash          string       `json:"hash"`
	Event         domain.Event `json:"event"`
}

type hashMaterial struct {
	SchemaVersion int          `json:"schemaVersion"`
	Sequence      uint64       `json:"sequence"`
	PreviousHash  string       `json:"previousHash"`
	Event         domain.Event `json:"event"`
}

func makeRecord(sequence uint64, previous string, event domain.Event) (ledgerRecord, error) {
	material := hashMaterial{SchemaVersion: schemaVersion, Sequence: sequence, PreviousHash: previous, Event: event}
	encoded, err := json.Marshal(material)
	if err != nil {
		return ledgerRecord{}, err
	}
	sum := sha256.Sum256(encoded)
	return ledgerRecord{SchemaVersion: schemaVersion, Sequence: sequence, PreviousHash: previous, Hash: hex.EncodeToString(sum[:]), Event: event}, nil
}

func verifyRecord(record ledgerRecord, expectedSequence uint64, previous string) error {
	if record.SchemaVersion != schemaVersion {
		return fmt.Errorf("不支持的账本 schemaVersion: %d", record.SchemaVersion)
	}
	if record.Sequence != expectedSequence {
		return fmt.Errorf("账本序号不连续: 期望 %d，实际 %d", expectedSequence, record.Sequence)
	}
	if record.PreviousHash != previous {
		return fmt.Errorf("账本前序哈希不匹配")
	}
	rebuilt, err := makeRecord(record.Sequence, record.PreviousHash, record.Event)
	if err != nil {
		return err
	}
	if rebuilt.Hash != record.Hash {
		return fmt.Errorf("账本记录哈希校验失败")
	}
	return nil
}
