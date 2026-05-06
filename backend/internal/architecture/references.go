package architecture

import (
	"strings"
	"time"
)

const maxRecentReferenceEvents = 25

func CollectReferenceEvent(input ReferenceInput) ReferenceEvent {
	joined := strings.ToLower(strings.Join(input.Texts, "\n"))
	if strings.TrimSpace(joined) == "" {
		return ReferenceEvent{}
	}
	timestamp := input.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	hits := make([]ReferenceHit, 0)
	for _, rule := range defaultPocketRules() {
		if count := countKeywordMentions(joined, rule.Directory, rule.Keywords); count > 0 {
			hits = append(hits, ReferenceHit{
				NodeID:    rule.ID,
				Directory: rule.Directory,
				Count:     count,
			})
		}
	}
	for _, rule := range defaultContractRules() {
		if count := countKeywordMentions(joined, rule.ID, rule.Keywords); count > 0 {
			hits = append(hits, ReferenceHit{
				Contract: rule.ID,
				Count:    count,
			})
		}
	}
	for _, rule := range defaultDatabaseRules() {
		if count := countKeywordMentions(joined, rule.ID, rule.Keywords); count > 0 {
			hits = append(hits, ReferenceHit{
				Database: rule.ID,
				Count:    count,
			})
		}
	}
	for _, rule := range defaultStructureRules() {
		if count := countKeywordMentions(joined, rule.ID, rule.Keywords); count > 0 {
			hits = append(hits, ReferenceHit{
				Structure: rule.ID,
				Count:     count,
			})
		}
	}
	if len(hits) == 0 {
		return ReferenceEvent{}
	}

	return ReferenceEvent{
		BuildID:   strings.TrimSpace(input.BuildID),
		TaskID:    strings.TrimSpace(input.TaskID),
		TaskType:  strings.TrimSpace(input.TaskType),
		AgentRole: strings.TrimSpace(input.AgentRole),
		Provider:  strings.TrimSpace(input.Provider),
		Model:     strings.TrimSpace(input.Model),
		Timestamp: timestamp.UTC(),
		Hits:      hits,
	}
}

func countKeywordMentions(text, canonical string, keywords []string) int {
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	add(canonical)
	for _, keyword := range keywords {
		add(keyword)
	}
	count := 0
	for keyword := range seen {
		count += strings.Count(text, keyword)
	}
	return count
}

func MergeReferenceTelemetry(base *ReferenceTelemetry, event ReferenceEvent) *ReferenceTelemetry {
	if len(event.Hits) == 0 {
		return CloneReferenceTelemetry(base)
	}
	out := CloneReferenceTelemetry(base)
	if out == nil {
		out = &ReferenceTelemetry{}
	}
	ensureReferenceMaps(out)
	for _, hit := range event.Hits {
		count := hit.Count
		if count <= 0 {
			count = 1
		}
		out.TotalReferences += count
		if hit.NodeID != "" {
			out.ByNode[hit.NodeID] += count
		}
		if hit.Directory != "" {
			out.ByDirectory[hit.Directory] += count
		}
		if hit.Contract != "" {
			out.ByContract[hit.Contract] += count
		}
		if hit.Database != "" {
			out.ByDatabase[hit.Database] += count
		}
		if hit.Structure != "" {
			out.ByStructure[hit.Structure] += count
		}
	}
	if event.AgentRole != "" {
		out.ByAgentRole[event.AgentRole]++
	}
	if event.TaskType != "" {
		out.ByTaskType[event.TaskType]++
	}
	timestamp := event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	out.LastUpdatedAt = &timestamp
	out.RecentEvents = append([]ReferenceEvent{event}, out.RecentEvents...)
	if len(out.RecentEvents) > maxRecentReferenceEvents {
		out.RecentEvents = out.RecentEvents[:maxRecentReferenceEvents]
	}
	return out
}

func MergeTelemetry(base, extra *ReferenceTelemetry) *ReferenceTelemetry {
	out := CloneReferenceTelemetry(base)
	if extra == nil {
		return out
	}
	if out == nil {
		out = &ReferenceTelemetry{}
	}
	ensureReferenceMaps(out)
	out.TotalReferences += extra.TotalReferences
	addCounts(out.ByNode, extra.ByNode)
	addCounts(out.ByDirectory, extra.ByDirectory)
	addCounts(out.ByContract, extra.ByContract)
	addCounts(out.ByDatabase, extra.ByDatabase)
	addCounts(out.ByStructure, extra.ByStructure)
	addCounts(out.ByAgentRole, extra.ByAgentRole)
	addCounts(out.ByTaskType, extra.ByTaskType)
	if extra.LastUpdatedAt != nil && (out.LastUpdatedAt == nil || extra.LastUpdatedAt.After(*out.LastUpdatedAt)) {
		t := *extra.LastUpdatedAt
		out.LastUpdatedAt = &t
	}
	out.RecentEvents = append(out.RecentEvents, extra.RecentEvents...)
	if len(out.RecentEvents) > maxRecentReferenceEvents {
		out.RecentEvents = out.RecentEvents[:maxRecentReferenceEvents]
	}
	return out
}

func CloneReferenceTelemetry(in *ReferenceTelemetry) *ReferenceTelemetry {
	if in == nil {
		return nil
	}
	out := &ReferenceTelemetry{
		TotalReferences: in.TotalReferences,
		ByNode:          cloneCounts(in.ByNode),
		ByDirectory:     cloneCounts(in.ByDirectory),
		ByContract:      cloneCounts(in.ByContract),
		ByDatabase:      cloneCounts(in.ByDatabase),
		ByStructure:     cloneCounts(in.ByStructure),
		ByAgentRole:     cloneCounts(in.ByAgentRole),
		ByTaskType:      cloneCounts(in.ByTaskType),
	}
	if in.LastUpdatedAt != nil {
		t := *in.LastUpdatedAt
		out.LastUpdatedAt = &t
	}
	if len(in.RecentEvents) > 0 {
		out.RecentEvents = make([]ReferenceEvent, len(in.RecentEvents))
		copy(out.RecentEvents, in.RecentEvents)
		for i := range out.RecentEvents {
			if len(out.RecentEvents[i].Hits) > 0 {
				out.RecentEvents[i].Hits = append([]ReferenceHit(nil), out.RecentEvents[i].Hits...)
			}
		}
	}
	return out
}

func ensureReferenceMaps(t *ReferenceTelemetry) {
	if t.ByNode == nil {
		t.ByNode = map[string]int{}
	}
	if t.ByDirectory == nil {
		t.ByDirectory = map[string]int{}
	}
	if t.ByContract == nil {
		t.ByContract = map[string]int{}
	}
	if t.ByDatabase == nil {
		t.ByDatabase = map[string]int{}
	}
	if t.ByStructure == nil {
		t.ByStructure = map[string]int{}
	}
	if t.ByAgentRole == nil {
		t.ByAgentRole = map[string]int{}
	}
	if t.ByTaskType == nil {
		t.ByTaskType = map[string]int{}
	}
}

func cloneCounts(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func addCounts(dst map[string]int, src map[string]int) {
	for key, value := range src {
		dst[key] += value
	}
}
