package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// FileDiff represents the lines that were added/modified in a file
type FileDiff struct {
	FileName      string
	AddedLines    map[int]bool // line numbers that were added
	ModifiedLines map[int]bool // line numbers that were modified (for now, treat same as added)
}

// DiffInfo contains diff information for all changed files
type DiffInfo struct {
	Files map[string]*FileDiff // maps file path to its diff
}

// ParseDiffInfo parses a JSON file containing diff information
// Expected format: { "file.go": { "added_lines": [1, 2, 3], "modified_lines": [5, 6] } }
func ParseDiffInfo(filename string) (*DiffInfo, error) {
	if filename == "" {
		return nil, nil // No diff info provided
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var rawDiff map[string]struct {
		AddedLines    []int `json:"added_lines"`
		ModifiedLines []int `json:"modified_lines"`
	}

	err = json.Unmarshal(data, &rawDiff)
	if err != nil {
		return nil, err
	}

	diffInfo := &DiffInfo{
		Files: make(map[string]*FileDiff),
	}

	for fileName, fileDiff := range rawDiff {
		fd := &FileDiff{
			FileName:      fileName,
			AddedLines:    make(map[int]bool),
			ModifiedLines: make(map[int]bool),
		}

		for _, line := range fileDiff.AddedLines {
			fd.AddedLines[line] = true
		}
		for _, line := range fileDiff.ModifiedLines {
			fd.ModifiedLines[line] = true
		}

		diffInfo.Files[fileName] = fd
	}

	return diffInfo, nil
}

// ParseUnifiedDiff parses a unified diff format (git diff output)
// This is an alternative format that's more standard
func ParseUnifiedDiff(filename string) (*DiffInfo, error) {
	if filename == "" {
		return nil, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	diffInfo := &DiffInfo{
		Files: make(map[string]*FileDiff),
	}

	scanner := bufio.NewScanner(file)
	var currentFile *FileDiff
	var currentLine int

	for scanner.Scan() {
		line := scanner.Text()

		// Check for file header: +++ b/path/to/file.go
		if strings.HasPrefix(line, "+++ b/") {
			fileName := strings.TrimPrefix(line, "+++ b/")
			currentFile = &FileDiff{
				FileName:      fileName,
				AddedLines:    make(map[int]bool),
				ModifiedLines: make(map[int]bool),
			}
			diffInfo.Files[fileName] = currentFile
			continue
		}

		// Check for hunk header: @@ -old_start,old_count +new_start,new_count @@
		if strings.HasPrefix(line, "@@") {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				// Parse +new_start,new_count
				newPart := strings.TrimPrefix(parts[2], "+")
				newParts := strings.Split(newPart, ",")
				if len(newParts) > 0 {
					start, err := strconv.Atoi(newParts[0])
					if err == nil {
						currentLine = start
					}
				}
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		// Lines starting with + are added lines
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			currentFile.AddedLines[currentLine] = true
			currentLine++
		} else if strings.HasPrefix(line, " ") {
			// Context line (unchanged)
			currentLine++
		}
		// Lines starting with - are deleted lines (we don't track these)
	}

	return diffInfo, scanner.Err()
}

// findFileDiff tries to find a FileDiff for the given fileName
// It handles the case where fileName might have a package prefix (e.g., "github.com/user/repo/cmd/file.go")
// while the diff has relative paths (e.g., "cmd/file.go")
func (d *DiffInfo) findFileDiff(fileName string) *FileDiff {
	if d == nil {
		return nil
	}

	// Try exact match first
	if fileDiff, ok := d.Files[fileName]; ok {
		return fileDiff
	}

	// Try to match by suffix - the diff path should be a suffix of the coverage path
	// Coverage: "github.com/user/repo/cmd/file.go"
	// Diff:     "cmd/file.go"
	for diffPath, fileDiff := range d.Files {
		if strings.HasSuffix(fileName, diffPath) {
			return fileDiff
		}
	}

	// Try the reverse - maybe the coverage path is shorter
	for diffPath, fileDiff := range d.Files {
		if strings.HasSuffix(diffPath, fileName) {
			return fileDiff
		}
	}

	return nil
}

// IsLineAdded checks if a specific line was added in the diff
func (d *DiffInfo) IsLineAdded(fileName string, lineNum int) bool {
	fileDiff := d.findFileDiff(fileName)
	if fileDiff == nil {
		return false
	}

	return fileDiff.AddedLines[lineNum] || fileDiff.ModifiedLines[lineNum]
}

// IsLineInRange checks if any line in the range [startLine, endLine] was added
func (d *DiffInfo) IsLineInRange(fileName string, startLine, endLine int) bool {
	fileDiff := d.findFileDiff(fileName)
	if fileDiff == nil {
		return false
	}

	for line := startLine; line <= endLine; line++ {
		if fileDiff.AddedLines[line] || fileDiff.ModifiedLines[line] {
			return true
		}
	}

	return false
}
