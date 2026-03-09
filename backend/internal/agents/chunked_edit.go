// Package agents — chunked file editing protocol for large files.
//
// When AI agents need to edit files larger than ChunkThreshold lines, sending
// the entire file in a single LLM call wastes tokens and risks truncation.
// ChunkedEditor splits large files into overlapping windows, applies targeted
// edits per chunk, and reassembles the result — keeping each LLM call within
// a manageable context budget while maintaining line-accurate output.
//
// Design:
//   - Files ≤ ChunkThreshold lines are passed through unchanged (zero overhead).
//   - Chunks overlap by OverlapLines so the model sees enough surrounding
//     context to produce coherent edits without re-reading the whole file.
//   - Reassembly uses the overlap to detect and deduplicate boundary regions,
//     giving a clean final file.
//   - All operations are pure functions on strings — no I/O, easy to test.

package agents

import (
	"fmt"
	"strings"
)

const (
	// ChunkThreshold is the line count above which a file is split into chunks.
	ChunkThreshold = 400

	// DefaultChunkSize is the number of lines per chunk (excluding overlap).
	DefaultChunkSize = 300

	// OverlapLines is how many lines each chunk shares with the next/previous
	// chunk so the model sees enough context at the boundaries.
	OverlapLines = 25
)

// FileChunk represents one window of a large file sent to the LLM.
type FileChunk struct {
	// Index is the 0-based position of this chunk in the sequence.
	Index int `json:"index"`

	// StartLine and EndLine are 1-based line numbers (inclusive) of the
	// content window within the original file.
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`

	// Content is the raw text of this chunk (may include overlap lines).
	Content string `json:"content"`

	// IsFirst / IsLast flags help the caller construct accurate prompts.
	IsFirst bool `json:"is_first"`
	IsLast  bool `json:"is_last"`

	// TotalChunks is the total number of chunks this file was split into.
	TotalChunks int `json:"total_chunks"`
}

// ChunkEditResult holds the LLM's replacement text for a single chunk.
type ChunkEditResult struct {
	ChunkIndex  int    `json:"chunk_index"`
	EditedText  string `json:"edited_text"`
	ChangesMade bool   `json:"changes_made"` // false = LLM returned the chunk unchanged
}

// ChunkedEditor splits, tracks, and reassembles large-file edits.
// It is stateless — the same instance can be used concurrently.
type ChunkedEditor struct{}

// NewChunkedEditor creates a ChunkedEditor.
func NewChunkedEditor() *ChunkedEditor { return &ChunkedEditor{} }

// NeedsChunking reports whether content exceeds the chunk threshold.
func (c *ChunkedEditor) NeedsChunking(content string) bool {
	return countLines(content) > ChunkThreshold
}

// SplitIntoChunks divides content into overlapping chunks.
//
// chunkSize is the number of primary (non-overlap) lines per chunk.
// overlapLines is the number of shared lines at each boundary.
// If chunkSize ≤ 0 DefaultChunkSize is used; if overlapLines < 0 OverlapLines is used.
func (c *ChunkedEditor) SplitIntoChunks(content string, chunkSize, overlapLines int) []FileChunk {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlapLines < 0 {
		overlapLines = OverlapLines
	}

	lines := strings.Split(content, "\n")
	total := len(lines)

	if total == 0 {
		return nil
	}

	var chunks []FileChunk
	idx := 0
	chunkIdx := 0

	for idx < total {
		// Core window: [idx, end)
		end := idx + chunkSize
		if end > total {
			end = total
		}

		// Expand downward by overlap (but don't go before file start)
		startWithOverlap := idx - overlapLines
		if startWithOverlap < 0 {
			startWithOverlap = 0
		}

		// Expand upward by overlap (but don't go past file end)
		endWithOverlap := end + overlapLines
		if endWithOverlap > total {
			endWithOverlap = total
		}

		chunkLines := lines[startWithOverlap:endWithOverlap]
		chunks = append(chunks, FileChunk{
			Index:     chunkIdx,
			StartLine: startWithOverlap + 1, // 1-based
			EndLine:   endWithOverlap,        // 1-based inclusive
			Content:   strings.Join(chunkLines, "\n"),
			IsFirst:   chunkIdx == 0,
			IsLast:    false, // fixed up below
		})

		idx = end
		chunkIdx++
	}

	if len(chunks) > 0 {
		chunks[len(chunks)-1].IsLast = true
		for i := range chunks {
			chunks[i].TotalChunks = len(chunks)
		}
	}

	return chunks
}

// ApplyChunkEdit merges an LLM-edited chunk back into the original file content.
//
// original is the full file text before any edits.
// editedChunk is the replacement text the LLM produced for chunk.
// chunk describes which lines of original this covers (1-based, inclusive).
//
// The returned string is the full file with those lines replaced by editedChunk.
func (c *ChunkedEditor) ApplyChunkEdit(original, editedChunk string, chunk FileChunk) string {
	lines := strings.Split(original, "\n")
	total := len(lines)

	// Convert 1-based inclusive to 0-based slice indices.
	start := chunk.StartLine - 1
	end := chunk.EndLine // exclusive for slicing

	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}

	editedLines := strings.Split(editedChunk, "\n")

	result := make([]string, 0, len(lines))
	result = append(result, lines[:start]...)
	result = append(result, editedLines...)
	if end < total {
		result = append(result, lines[end:]...)
	}

	return strings.Join(result, "\n")
}

// ReassembleFromResults takes the original file and an ordered list of chunk
// edit results, applies each edit in order (earliest chunk first), and returns
// the final assembled file.
//
// Results that report ChangesMade=false are skipped to avoid redundant work.
// Results must be sorted by ChunkIndex ascending — call SortChunkResults first
// if ordering is not guaranteed.
//
// Because each ApplyChunkEdit call recomputes line offsets from the running
// result, this handles edits that insert or delete lines correctly.
func (c *ChunkedEditor) ReassembleFromResults(original string, chunks []FileChunk, results []ChunkEditResult) string {
	// Index results by chunk index for O(1) lookup.
	byIdx := make(map[int]ChunkEditResult, len(results))
	for _, r := range results {
		byIdx[r.ChunkIndex] = r
	}

	// Apply edits from the last chunk to the first so that earlier line numbers
	// remain valid after each substitution.
	current := original
	for i := len(chunks) - 1; i >= 0; i-- {
		r, ok := byIdx[chunks[i].Index]
		if !ok || !r.ChangesMade {
			continue
		}
		current = c.ApplyChunkEdit(current, r.EditedText, chunks[i])
	}
	return current
}

// BuildChunkPrompt constructs the LLM prompt for editing a single chunk.
// instruction is the high-level edit directive (e.g. "add error handling to all async functions").
func (c *ChunkedEditor) BuildChunkPrompt(chunk FileChunk, instruction string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"You are editing part %d of %d of a source file.\n",
		chunk.Index+1, chunk.TotalChunks,
	))
	sb.WriteString(fmt.Sprintf(
		"This chunk covers lines %d–%d of the original file.\n",
		chunk.StartLine, chunk.EndLine,
	))
	if !chunk.IsFirst {
		sb.WriteString(fmt.Sprintf(
			"The first %d lines are overlap context from the previous chunk — preserve them exactly.\n",
			OverlapLines,
		))
	}
	if !chunk.IsLast {
		sb.WriteString(fmt.Sprintf(
			"The last %d lines are overlap context for the next chunk — preserve them exactly.\n",
			OverlapLines,
		))
	}
	sb.WriteString("\n## Edit Instruction\n\n")
	sb.WriteString(instruction)
	sb.WriteString("\n\n## Chunk Content\n\n```\n")
	sb.WriteString(chunk.Content)
	sb.WriteString("\n```\n\n")
	sb.WriteString("Return ONLY the modified chunk content. No explanation, no code fences, no extra text.\n")
	sb.WriteString("If no changes are needed for this chunk, return the chunk content unchanged.\n")
	return sb.String()
}

// SortChunkResults sorts results in ascending ChunkIndex order (insertion sort
// for small slices — chunk counts are typically < 20).
func SortChunkResults(results []ChunkEditResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].ChunkIndex < results[j-1].ChunkIndex; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// countLines returns the number of lines in s (number of \n + 1).
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
