package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

type manifest struct {
	Version int         `json:"version"`
	Files   []fileEntry `json:"files"`
}

func writeManifest(path string, files []fileEntry) error {
	data := manifest{Version: 1, Files: files}
	if err := data.validate(); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("создать манифест: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("не удалось полностью записать манифест %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("не удалось завершить запись манифеста %q: %w", path, closeErr)
	}
	return nil
}

func readManifest(path string) ([]fileEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("открыть манифест: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var data manifest
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("прочитать манифест: %w", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, fmt.Errorf("ожидался ровно один JSON-объект манифеста")
	}
	if err := data.validate(); err != nil {
		return nil, err
	}
	for i := range data.Files {
		data.Files[i].SHA256 = strings.ToLower(data.Files[i].SHA256)
	}
	sortFiles(data.Files)
	return data.Files, nil
}

func (m manifest) validate() error {
	if m.Version != 1 {
		return fmt.Errorf("версия манифеста %d не поддерживается", m.Version)
	}
	if m.Files == nil {
		return fmt.Errorf("в манифесте отсутствует массив files")
	}
	paths := make(map[string]bool, len(m.Files))
	for _, file := range m.Files {
		if file.Path == "." || !fs.ValidPath(file.Path) {
			return fmt.Errorf("неверный относительный путь %q в манифесте", file.Path)
		}
		if paths[file.Path] {
			return fmt.Errorf("путь %q дважды указан в манифесте", file.Path)
		}
		paths[file.Path] = true
		if sum, err := hex.DecodeString(file.SHA256); err != nil || len(sum) != 32 || file.Size < 0 {
			return fmt.Errorf("неверный размер или SHA-256 у %q в манифесте", file.Path)
		}
	}
	return nil
}
