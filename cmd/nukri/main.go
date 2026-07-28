package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/urdadx/nukri/internal/core"
	fileinfo "github.com/urdadx/nukri/internal/file_info"
)

func main() {
	const directory = "."
	entries, err := os.ReadDir(directory)
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	table := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "CLASS\tNAME\tSIZE\tMODIFIED")
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Printf("%s: %v", entry.Name(), err)
			continue
		}

		kind := core.File
		if entry.IsDir() {
			kind = core.Directory
		}
		model := core.Entry{
			Name:     entry.Name(),
			Path:     filepath.Join(directory, entry.Name()),
			Kind:     kind,
			Size:     info.Size(),
			Modified: info.ModTime(),
		}
		facts := fileinfo.InspectEntryFast(&model)
		size := formatSize(model.Size)
		if model.IsDirectory() {
			size = "-"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
			facts.BuiltinClass,
			model.Name,
			size,
			formatModified(model.Modified),
		)
	}
	if err := table.Flush(); err != nil {
		log.Fatal(err)
	}
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func formatModified(modified time.Time) string {
	if modified.IsZero() {
		return "-"
	}
	return modified.Format("2006-01-02 15:04")
}
