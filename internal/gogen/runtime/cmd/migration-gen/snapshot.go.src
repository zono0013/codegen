package main

import (
	"encoding/json"
	"errors"
	"os"
)

// snapshotFile は migrations/.snapshot.json の論理構造。
// マップではなく struct で wrap するのは将来 schema バージョン番号などの
// メタデータを足す余地を残すため。
type snapshotFile struct {
	Tables map[string]*tableSchema `json:"tables"`
}

// loadSnapshot はファイルが存在しない場合は空の状態として nil-empty マップを返す。
func loadSnapshot(path string) (map[string]*tableSchema, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]*tableSchema{}, nil
	}
	if err != nil {
		return nil, err
	}
	var sf snapshotFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return nil, err
	}
	if sf.Tables == nil {
		sf.Tables = map[string]*tableSchema{}
	}
	return sf.Tables, nil
}

func saveSnapshot(path string, tables map[string]*tableSchema) error {
	sf := snapshotFile{Tables: tables}
	b, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
