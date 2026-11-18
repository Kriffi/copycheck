package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

const (
	moved   = "Перемещён"
	changed = "Изменён"
	missing = "Отсутствует"
	added   = "Новый"
)

type difference struct {
	status string
	path   string
	to     string
}

type report struct {
	matched     int
	differences []difference
}

type fingerprint struct {
	size   int64
	sha256 string
}

func identity(file fileEntry) fingerprint {
	return fingerprint{size: file.Size, sha256: file.SHA256}
}

func compare(expected, actual []fileEntry) report {
	var result report
	byPath := make(map[string]fileEntry, len(actual))
	for _, file := range actual {
		byPath[file.Path] = file
	}
	var unpaired []fileEntry
	for _, file := range expected {
		current, exists := byPath[file.Path]
		if !exists {
			unpaired = append(unpaired, file)
			continue
		}
		delete(byPath, file.Path)
		if identity(file) == identity(current) {
			result.matched++
		} else {
			result.differences = append(result.differences, difference{status: changed, path: file.Path})
		}
	}

	byContent := make(map[fingerprint][]string, len(byPath))
	for _, file := range actual {
		if _, unpaired := byPath[file.Path]; unpaired {
			key := identity(file)
			byContent[key] = append(byContent[key], file.Path)
		}
	}
	for _, file := range unpaired {
		key := identity(file)
		candidates := byContent[key]
		if len(candidates) == 0 {
			result.differences = append(result.differences, difference{status: missing, path: file.Path})
			continue
		}
		to := candidates[0]
		byContent[key] = candidates[1:]
		delete(byPath, to)
		result.differences = append(result.differences, difference{status: moved, path: file.Path, to: to})
	}
	for _, file := range actual {
		if _, extra := byPath[file.Path]; extra {
			result.differences = append(result.differences, difference{status: added, path: file.Path})
		}
	}
	return result
}

func (r report) print(out io.Writer) {
	fmt.Fprintln(out, "Проверка копии")
	fmt.Fprintf(out, "Совпало: %d\n", r.matched)
	counts := make(map[string]int)
	for _, diff := range r.differences {
		counts[diff.status]++
	}
	fmt.Fprintf(out, "Перемещено: %d | Изменено: %d | Отсутствует: %d | Новых: %d\n",
		counts[moved], counts[changed], counts[missing], counts[added])
	if len(r.differences) > 0 {
		fmt.Fprintln(out)
	}
	for _, diff := range r.differences {
		name := displayPath(diff.path)
		if diff.to != "" {
			name += " → " + displayPath(diff.to)
		}
		fmt.Fprintf(out, "%s: %s\n", diff.status, name)
	}
}

func displayPath(path string) string {
	if strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return strconv.Quote(path)
	}
	return path
}
