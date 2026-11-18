package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const usage = `CopyCheck · проверка копирования по SHA-256

  copycheck snapshot <исходная папка> --out <манифест.json>
  copycheck verify <папка копии> --manifest <манифест.json>

Коды выхода: 0 — успех, 1 — есть различия, 2 — ошибка.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) < 2 || (args[0] != "snapshot" && args[0] != "verify") {
		fmt.Fprintln(stderr, "Ошибка: укажи команду и папку.")
		fmt.Fprint(stderr, usage)
		return 2
	}

	command, root := args[0], args[1]
	option := "manifest"
	if command == "snapshot" {
		option = "out"
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String(option, "", "")
	if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 || *manifestPath == "" {
		fmt.Fprintf(stderr, "Ошибка: после папки нужно указать --%s <манифест.json>.\n\n", option)
		fmt.Fprint(stderr, usage)
		return 2
	}

	if command == "snapshot" {
		files, err := scanFolder(root, *manifestPath)
		if err != nil {
			return printError(stderr, err)
		}
		if err := writeManifest(*manifestPath, files); err != nil {
			return printError(stderr, err)
		}
		fmt.Fprintf(stdout, "Снимок сохранён: %s\nФайлов: %d\n", *manifestPath, len(files))
		return 0
	}

	expected, err := readManifest(*manifestPath)
	if err != nil {
		return printError(stderr, err)
	}
	actual, err := scanFolder(root, *manifestPath)
	if err != nil {
		return printError(stderr, err)
	}
	result := compare(expected, actual)
	result.print(stdout)
	if len(result.differences) != 0 {
		return 1
	}
	return 0
}

func printError(out io.Writer, err error) int {
	fmt.Fprintf(out, "Ошибка: %v\n", err)
	return 2
}
