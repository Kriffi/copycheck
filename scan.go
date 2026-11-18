package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type fileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type scanResult struct {
	entry fileEntry
	err   error
}

func scanFolder(root, manifestPath string) ([]fileEntry, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("открыть папку: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("открыть папку: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q не является папкой", root)
	}
	manifestPath, err = absoluteManifestPath(manifestPath)
	if err != nil {
		return nil, err
	}

	workers := min(runtime.GOMAXPROCS(0), 4)
	paths := make(chan string, workers)
	results := make(chan scanResult, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for path := range paths {
				entry, err := hashFile(root, path)
				results <- scanResult{entry: entry, err: err}
			}
		}()
	}

	go func() {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				results <- scanResult{err: walkErr}
				return nil
			}
			if entry.Type().IsRegular() && !samePath(path, manifestPath) {
				paths <- path
			}
			return nil
		})
		if err != nil {
			results <- scanResult{err: err}
		}
		close(paths)
		group.Wait()
		close(results)
	}()

	entries := make([]fileEntry, 0)
	var failures []string
	for result := range results {
		if result.err != nil {
			failures = append(failures, result.err.Error())
			continue
		}
		entries = append(entries, result.entry)
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return nil, fmt.Errorf("не удалось прочитать папку целиком:\n  %s", strings.Join(failures, "\n  "))
	}
	sortFiles(entries)
	return entries, nil
}

func hashFile(root, path string) (fileEntry, error) {
	name, err := filepath.Rel(root, path)
	if err != nil {
		return fileEntry{}, err
	}
	entry, err := readFileHash(path)
	if err != nil {
		return fileEntry{}, fmt.Errorf("%q: %w", name, err)
	}
	entry.Path = filepath.ToSlash(name)
	return entry, nil
}

func readFileHash(path string) (fileEntry, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return fileEntry{}, err
	}
	if !before.Mode().IsRegular() {
		return fileEntry{}, errors.New("файл перестал быть обычным")
	}
	file, err := os.Open(path)
	if err != nil {
		return fileEntry{}, err
	}
	defer file.Close()

	opened, err := file.Stat()
	if err != nil {
		return fileEntry{}, err
	}
	if !os.SameFile(before, opened) || before.Size() != opened.Size() || !before.ModTime().Equal(opened.ModTime()) {
		return fileEntry{}, errors.New("файл изменился перед чтением")
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return fileEntry{}, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return fileEntry{}, err
	}
	if !os.SameFile(before, after) || size != before.Size() || size != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fileEntry{}, errors.New("файл изменился во время чтения")
	}
	return fileEntry{Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func absoluteManifestPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err == nil {
		return filepath.Join(parent, filepath.Base(absolute)), nil
	}
	return absolute, nil
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	if runtime.GOOS != "windows" || !strings.EqualFold(a, b) {
		return false
	}
	first, firstErr := os.Lstat(a)
	second, secondErr := os.Lstat(b)
	return firstErr == nil && secondErr == nil && os.SameFile(first, second)
}

func sortFiles(files []fileEntry) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
}
