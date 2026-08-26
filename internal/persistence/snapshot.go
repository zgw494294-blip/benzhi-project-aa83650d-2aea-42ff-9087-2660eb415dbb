package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"phonemereleasedesk/internal/domain"
)

func safeName(id string) string {
	value := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
	if value == "" {
		return "batch"
	}
	return value
}

func (r *FileRepository) writeSnapshot(batch *domain.ReleaseBatch) error {
	directory := filepath.Join(r.dir, "snapshots")
	finalPath := filepath.Join(directory, safeName(batch.ID)+".json")
	temporary, err := os.CreateTemp(directory, ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func() { temporary.Close(); os.Remove(temporaryPath) }
	snapshot := domain.BatchSnapshot{SchemaVersion: schemaVersion, LastSequence: r.sequence, LastHash: r.lastHash, Batch: batch.Clone()}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
