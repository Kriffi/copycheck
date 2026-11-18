package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotAndVerify(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	manifest := filepath.Join(t.TempDir(), "photos.json")

	writeFile(t, source, "DCIM/IMG_0142.jpg", "holiday photo")
	writeFile(t, source, "DCIM/IMG_0190.jpg", "original")
	writeFile(t, source, "DCIM/IMG_0211.jpg", "missing photo")
	writeFile(t, source, "Фото/совпало.jpg", "unchanged")
	writeFile(t, destination, "Отпуск/Море.jpg", "holiday photo")
	writeFile(t, destination, "DCIM/IMG_0190.jpg", "corrupt!")
	writeFile(t, destination, "Фото/совпало.jpg", "unchanged")
	writeFile(t, destination, "Отпуск/заметки.txt", "new file")

	invoke(t, 0, "snapshot", source, "--out", manifest)
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}

	output := invoke(t, 1, "verify", destination, "--manifest", manifest)
	for _, want := range []string{
		"Совпало: 1",
		"Перемещён: DCIM/IMG_0142.jpg → Отпуск/Море.jpg",
		"Изменён: DCIM/IMG_0190.jpg",
		"Отсутствует: DCIM/IMG_0211.jpg",
		"Новый: Отпуск/заметки.txt",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("report lacks %q:\n%s", want, output)
		}
	}
}

func TestDuplicateFilesUseEachDestinationOnce(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	manifest := filepath.Join(t.TempDir(), "files.json")
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, source, name, "same contents")
	}
	writeFile(t, destination, "c.txt", "same contents")
	writeFile(t, destination, "x.txt", "same contents")
	invoke(t, 0, "snapshot", source, "--out", manifest)
	output := invoke(t, 1, "verify", destination, "--manifest", manifest)
	for _, want := range []string{"Совпало: 1", "Перемещён: a.txt → x.txt", "Отсутствует: b.txt"} {
		if !strings.Contains(output, want) {
			t.Errorf("report lacks %q:\n%s", want, output)
		}
	}
}

func TestManifestInsideFolderIsExcluded(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "snapshot.json")
	writeFile(t, dir, "empty.txt", "")
	invoke(t, 0, "snapshot", dir, "--out", manifest)
	invoke(t, 2, "snapshot", dir, "--out", manifest)
	output := invoke(t, 0, "verify", dir, "--manifest", manifest)
	if !strings.Contains(output, "Совпало: 1") {
		t.Fatalf("manifest should not count as a file:\n%s", output)
	}
}

func TestSnapshotDoesNotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "original.txt", "original contents")
	original := filepath.Join(dir, "original.txt")
	invoke(t, 2, "snapshot", dir, "--out", original)
	content, err := os.ReadFile(original)
	if err != nil || string(content) != "original contents" {
		t.Fatalf("snapshot overwrote original file: %q, %v", content, err)
	}
}

func TestInvalidManifestAndScanErrors(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "invalid.json")
	for _, content := range []string{
		"invalid JSON",
		"{\"version\":2,\"files\":[]}",
		"{\"version\":1,\"files\":[]} {}",
		fmt.Sprintf("{\"version\":1,\"files\":[{\"path\":\"../escape\",\"size\":0,\"sha256\":\"%s\"}]}", strings.Repeat("0", 64)),
		fmt.Sprintf("{\"version\":1,\"files\":[{\"path\":\"same\",\"size\":0,\"sha256\":\"%[1]s\"},{\"path\":\"same\",\"size\":0,\"sha256\":\"%[1]s\"}]}", strings.Repeat("0", 64)),
	} {
		writeFile(t, dir, "invalid.json", content)
		invoke(t, 2, "verify", dir, "--manifest", manifest)
	}
	writeFile(t, dir, "original.json", "existing manifest")
	old := filepath.Join(dir, "original.json")
	invoke(t, 2, "snapshot", filepath.Join(dir, "missing"), "--out", old)
	content, err := os.ReadFile(old)
	if err != nil || string(content) != "existing manifest" {
		t.Fatalf("failed snapshot changed existing file: %q, %v", content, err)
	}
}

func TestNoUnexpectedFilesOrLinksInEmptySnapshot(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(t.TempDir(), "empty.json")
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "linked-directory")); err != nil {
		t.Logf("creating symlinks is unavailable: %v", err)
	}
	invoke(t, 0, "snapshot", dir, "--out", manifest)
	if output := invoke(t, 0, "verify", dir, "--manifest", manifest); !strings.Contains(output, "Совпало: 0") {
		t.Fatal(output)
	}
}

func writeFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func invoke(t *testing.T, code int, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if got := run(args, &stdout, &stderr); got != code {
		t.Fatalf("%v: exit %d, want %d\nstdout: %s\nstderr: %s", args, got, code, &stdout, &stderr)
	}
	if code == 2 && !strings.Contains(stderr.String(), "Ошибка") {
		t.Fatalf("expected an error message, got %q", stderr.String())
	}
	return stdout.String()
}
