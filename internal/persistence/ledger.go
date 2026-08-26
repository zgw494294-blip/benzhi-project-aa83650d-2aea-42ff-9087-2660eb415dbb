package persistence

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"

	"phonemereleasedesk/internal/domain"
)

func (r *FileRepository) appendRecord(record ledgerRecord) error {
	file, err := os.OpenFile(r.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(record); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (r *FileRepository) recoverLedger() error {
	file, err := os.Open(r.ledgerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	sequence := uint64(1)
	previous := ""
	events := map[string][]domain.Event{}
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			var record ledgerRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return err
			}
			if err := verifyRecord(record, sequence, previous); err != nil {
				return err
			}
			events[record.Event.BatchID] = append(events[record.Event.BatchID], record.Event)
			previous, r.lastHash, r.sequence = record.Hash, record.Hash, record.Sequence
			sequence++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	for id, history := range events {
		batch, err := domain.Replay(history)
		if err != nil {
			return err
		}
		r.batches[id] = batch
	}
	return nil
}
