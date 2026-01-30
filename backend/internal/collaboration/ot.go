// APEX.BUILD Operational Transformation Engine
// Real-time collaborative editing with conflict resolution

package collaboration

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Operation types for OT
type OpType string

const (
	OpInsert OpType = "insert"
	OpDelete OpType = "delete"
	OpRetain OpType = "retain"
)

// Operation represents a single OT operation
type Operation struct {
	Type     OpType `json:"type"`
	Position int    `json:"position"`
	Text     string `json:"text,omitempty"`     // For insert
	Count    int    `json:"count,omitempty"`    // For delete/retain
}

// Document represents the current state of a collaborative document
type Document struct {
	ID        uint       `json:"id"`
	Content   string     `json:"content"`
	Version   int        `json:"version"`
	History   []Revision `json:"history"`
	mu        sync.RWMutex
}

// Revision represents a single revision in document history
type Revision struct {
	Version    int         `json:"version"`
	UserID     uint        `json:"user_id"`
	Operations []Operation `json:"operations"`
	Timestamp  time.Time   `json:"timestamp"`
}

// TextOperation represents a complete text operation with metadata
type TextOperation struct {
	Operations   []Operation `json:"operations"`
	BaseVersion  int         `json:"base_version"`
	UserID       uint        `json:"user_id"`
	FileID       uint        `json:"file_id"`
	Timestamp    time.Time   `json:"timestamp"`
}

// OTEngine handles operational transformation
type OTEngine struct {
	documents map[uint]*Document
	mu        sync.RWMutex
}

// NewOTEngine creates a new OT engine
func NewOTEngine() *OTEngine {
	return &OTEngine{
		documents: make(map[uint]*Document),
	}
}

// GetDocument retrieves or creates a document
func (e *OTEngine) GetDocument(fileID uint, initialContent string) *Document {
	e.mu.Lock()
	defer e.mu.Unlock()

	if doc, exists := e.documents[fileID]; exists {
		return doc
	}

	doc := &Document{
		ID:      fileID,
		Content: initialContent,
		Version: 0,
		History: make([]Revision, 0),
	}
	e.documents[fileID] = doc
	return doc
}

// Apply applies an operation to a document with OT
func (e *OTEngine) Apply(op TextOperation) (*Document, []Operation, error) {
	e.mu.Lock()
	doc, exists := e.documents[op.FileID]
	e.mu.Unlock()

	if !exists {
		return nil, nil, errors.New("document not found")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	// Transform operation against concurrent operations
	transformedOps := op.Operations
	if op.BaseVersion < doc.Version {
		// Need to transform against intervening operations
		for i := op.BaseVersion; i < doc.Version; i++ {
			if i < len(doc.History) {
				concurrentOps := doc.History[i].Operations
				transformedOps = TransformOperations(transformedOps, concurrentOps)
			}
		}
	}

	// Apply transformed operations
	newContent, err := ApplyOperations(doc.Content, transformedOps)
	if err != nil {
		return nil, nil, err
	}

	// Update document
	doc.Content = newContent
	doc.Version++
	doc.History = append(doc.History, Revision{
		Version:    doc.Version,
		UserID:     op.UserID,
		Operations: transformedOps,
		Timestamp:  time.Now(),
	})

	// Trim history to prevent unbounded growth (keep last 1000 revisions)
	if len(doc.History) > 1000 {
		doc.History = doc.History[len(doc.History)-1000:]
	}

	return doc, transformedOps, nil
}

// TransformOperations transforms op1 against op2
// Returns transformed version of op1 that can be applied after op2
func TransformOperations(op1, op2 []Operation) []Operation {
	result := make([]Operation, 0, len(op1))

	for _, o1 := range op1 {
		transformed := transformSingle(o1, op2)
		result = append(result, transformed...)
	}

	return result
}

// transformSingle transforms a single operation against a list of operations
func transformSingle(op Operation, against []Operation) []Operation {
	result := []Operation{op}

	for _, other := range against {
		newResult := make([]Operation, 0)
		for _, o := range result {
			transformed := transformPair(o, other)
			newResult = append(newResult, transformed...)
		}
		result = newResult
	}

	return result
}

// transformPair transforms operation a against operation b
func transformPair(a, b Operation) []Operation {
	switch a.Type {
	case OpInsert:
		return transformInsert(a, b)
	case OpDelete:
		return transformDelete(a, b)
	case OpRetain:
		return transformRetain(a, b)
	}
	return []Operation{a}
}

// transformInsert transforms an insert operation
func transformInsert(ins Operation, other Operation) []Operation {
	switch other.Type {
	case OpInsert:
		// If other insert is before or at same position, shift our insert
		if other.Position <= ins.Position {
			return []Operation{{
				Type:     OpInsert,
				Position: ins.Position + len(other.Text),
				Text:     ins.Text,
			}}
		}
		return []Operation{ins}

	case OpDelete:
		if other.Position < ins.Position {
			// Delete before insert - shift position
			newPos := ins.Position - other.Count
			if newPos < other.Position {
				newPos = other.Position
			}
			return []Operation{{
				Type:     OpInsert,
				Position: newPos,
				Text:     ins.Text,
			}}
		}
		return []Operation{ins}

	case OpRetain:
		return []Operation{ins}
	}

	return []Operation{ins}
}

// transformDelete transforms a delete operation
func transformDelete(del Operation, other Operation) []Operation {
	switch other.Type {
	case OpInsert:
		if other.Position <= del.Position {
			// Insert before delete - shift delete position
			return []Operation{{
				Type:     OpDelete,
				Position: del.Position + len(other.Text),
				Count:    del.Count,
			}}
		} else if other.Position < del.Position + del.Count {
			// Insert within delete region - split delete
			firstPart := other.Position - del.Position
			secondPart := del.Count - firstPart
			return []Operation{
				{Type: OpDelete, Position: del.Position, Count: firstPart},
				{Type: OpDelete, Position: del.Position + len(other.Text), Count: secondPart},
			}
		}
		return []Operation{del}

	case OpDelete:
		// Handle overlapping deletes
		if other.Position >= del.Position + del.Count {
			// Other delete is after ours - no change
			return []Operation{del}
		} else if del.Position >= other.Position + other.Count {
			// Our delete is after other - shift position
			return []Operation{{
				Type:     OpDelete,
				Position: del.Position - other.Count,
				Count:    del.Count,
			}}
		} else {
			// Overlapping deletes - complex case
			start1, end1 := del.Position, del.Position + del.Count
			start2, end2 := other.Position, other.Position + other.Count

			// Calculate non-overlapping portion
			if start1 < start2 {
				if end1 <= end2 {
					// Our delete starts before, ends within or at other
					return []Operation{{
						Type:     OpDelete,
						Position: start1,
						Count:    start2 - start1,
					}}
				} else {
					// Our delete encompasses other
					return []Operation{{
						Type:     OpDelete,
						Position: start1,
						Count:    del.Count - other.Count,
					}}
				}
			} else {
				if end1 <= end2 {
					// Our delete is within other - nothing to delete
					return []Operation{}
				} else {
					// Our delete starts within, ends after
					return []Operation{{
						Type:     OpDelete,
						Position: start2,
						Count:    end1 - end2,
					}}
				}
			}
		}

	case OpRetain:
		return []Operation{del}
	}

	return []Operation{del}
}

// transformRetain transforms a retain operation
func transformRetain(ret Operation, other Operation) []Operation {
	// Retain operations generally don't need transformation
	// They just move the cursor position
	return []Operation{ret}
}

// ApplyOperations applies a list of operations to a string
func ApplyOperations(content string, ops []Operation) (string, error) {
	runes := []rune(content)

	for _, op := range ops {
		switch op.Type {
		case OpInsert:
			if op.Position < 0 || op.Position > len(runes) {
				// Clamp position to valid range
				if op.Position < 0 {
					op.Position = 0
				} else {
					op.Position = len(runes)
				}
			}
			newRunes := make([]rune, 0, len(runes)+len(op.Text))
			newRunes = append(newRunes, runes[:op.Position]...)
			newRunes = append(newRunes, []rune(op.Text)...)
			newRunes = append(newRunes, runes[op.Position:]...)
			runes = newRunes

		case OpDelete:
			if op.Position < 0 || op.Position >= len(runes) {
				continue // Skip invalid deletes
			}
			end := op.Position + op.Count
			if end > len(runes) {
				end = len(runes)
			}
			newRunes := make([]rune, 0, len(runes)-op.Count)
			newRunes = append(newRunes, runes[:op.Position]...)
			newRunes = append(newRunes, runes[end:]...)
			runes = newRunes

		case OpRetain:
			// Retain doesn't modify content
			continue
		}
	}

	return string(runes), nil
}

// ComposeOperations composes two sequences of operations into one
func ComposeOperations(ops1, ops2 []Operation) []Operation {
	result := make([]Operation, 0, len(ops1)+len(ops2))

	// Simple composition - append and optimize
	result = append(result, ops1...)

	// Adjust ops2 positions based on ops1
	offset := 0
	for _, op := range ops1 {
		switch op.Type {
		case OpInsert:
			offset += len(op.Text)
		case OpDelete:
			offset -= op.Count
		}
	}

	for _, op := range ops2 {
		adjusted := op
		if op.Type != OpRetain {
			adjusted.Position += offset
		}
		result = append(result, adjusted)
	}

	return optimizeOperations(result)
}

// optimizeOperations merges consecutive operations of the same type
func optimizeOperations(ops []Operation) []Operation {
	if len(ops) == 0 {
		return ops
	}

	result := make([]Operation, 0, len(ops))
	current := ops[0]

	for i := 1; i < len(ops); i++ {
		op := ops[i]

		// Try to merge with current
		if current.Type == op.Type {
			switch current.Type {
			case OpInsert:
				if current.Position + len(current.Text) == op.Position {
					current.Text += op.Text
					continue
				}
			case OpDelete:
				if current.Position == op.Position {
					current.Count += op.Count
					continue
				}
			case OpRetain:
				current.Count += op.Count
				continue
			}
		}

		result = append(result, current)
		current = op
	}

	result = append(result, current)
	return result
}

// CreateInsertOp creates an insert operation
func CreateInsertOp(position int, text string) Operation {
	return Operation{
		Type:     OpInsert,
		Position: position,
		Text:     text,
	}
}

// CreateDeleteOp creates a delete operation
func CreateDeleteOp(position, count int) Operation {
	return Operation{
		Type:     OpDelete,
		Position: position,
		Count:    count,
	}
}

// CreateRetainOp creates a retain operation
func CreateRetainOp(count int) Operation {
	return Operation{
		Type:  OpRetain,
		Count: count,
	}
}

// DocumentState returns JSON-serializable document state
func (d *Document) State() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"id":      d.ID,
		"content": d.Content,
		"version": d.Version,
	}
}

// ToJSON serializes a TextOperation to JSON
func (op *TextOperation) ToJSON() ([]byte, error) {
	return json.Marshal(op)
}

// FromJSON deserializes a TextOperation from JSON
func TextOperationFromJSON(data []byte) (*TextOperation, error) {
	var op TextOperation
	err := json.Unmarshal(data, &op)
	return &op, err
}
