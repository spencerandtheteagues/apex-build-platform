package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"apex-build/internal/ai"

	"github.com/google/uuid"
)

const (
	maxBuildConversationMessages = 120
	maxBuildSteeringNotes        = 16
	maxPendingRevisions          = 8
	maxBuildActivityEntries      = 160
	maxBuildApprovalEvents       = 80
)

type leadMessagePermissionRequest struct {
	Scope          string `json:"scope"`
	Target         string `json:"target"`
	Reason         string `json:"reason"`
	CommandPreview string `json:"command_preview"`
	Blocking       bool   `json:"blocking"`
}

type leadMessageAgentDirective struct {
	TargetMode      string `json:"target_mode"`
	TargetAgentID   string `json:"target_agent_id"`
	TargetAgentRole string `json:"target_agent_role"`
	Message         string `json:"message"`
}

type leadMessagePlan struct {
	Reply                string                         `json:"reply"`
	ApplyChanges         bool                           `json:"apply_changes"`
	RequiresUserResponse bool                           `json:"requires_user_response"`
	Question             string                         `json:"question"`
	PauseBuild           bool                           `json:"pause_build"`
	ResumeBuild          bool                           `json:"resume_build"`
	SteeringUpdates      []string                       `json:"steering_updates"`
	AgentDirectives      []leadMessageAgentDirective    `json:"agent_directives"`
	PermissionRequests   []leadMessagePermissionRequest `json:"permission_requests"`
}

type buildMessageTarget struct {
	Mode      BuildMessageTargetMode
	AgentID   string
	AgentRole string
}

func normalizePermissionScope(raw string) BuildPermissionScope {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "filesystem", "file", "path":
		return PermissionScopeFilesystem
	case "network", "port", "host":
		return PermissionScopeNetwork
	case "service", "daemon":
		return PermissionScopeService
	default:
		return PermissionScopeProgram
	}
}

func normalizePermissionMode(raw string) BuildPermissionMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "once":
		return PermissionModeOnce
	default:
		return PermissionModeBuild
	}
}

func normalizePermissionDecision(raw string) BuildPermissionDecision {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "deny", "denied":
		return PermissionDecisionDeny
	case "allow", "allowed", "approve", "approved":
		return PermissionDecisionAllow
	default:
		return PermissionDecisionAsk
	}
}

func normalizeBuildMessageTargetMode(raw string) BuildMessageTargetMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(BuildMessageTargetAgent):
		return BuildMessageTargetAgent
	case string(BuildMessageTargetRole):
		return BuildMessageTargetRole
	case string(BuildMessageTargetAllAgents), "all", "broadcast":
		return BuildMessageTargetAllAgents
	default:
		return BuildMessageTargetLead
	}
}

func normalizeBuildMessageTarget(mode string, agentID string, agentRole string) buildMessageTarget {
	target := buildMessageTarget{
		Mode:      normalizeBuildMessageTargetMode(mode),
		AgentID:   strings.TrimSpace(agentID),
		AgentRole: strings.TrimSpace(strings.ToLower(agentRole)),
	}

	switch target.Mode {
	case BuildMessageTargetAgent:
		if target.AgentID == "" {
			target.Mode = BuildMessageTargetLead
		}
	case BuildMessageTargetRole:
		if target.AgentRole == "" {
			target.Mode = BuildMessageTargetLead
		}
	case BuildMessageTargetAllAgents:
		target.AgentID = ""
		target.AgentRole = ""
	default:
		target.Mode = BuildMessageTargetLead
		target.AgentID = ""
		target.AgentRole = ""
	}

	return target
}

func copyBuildInteractionStateLocked(build *Build) BuildInteractionState {
	if build == nil {
		return BuildInteractionState{}
	}

	copyMessages := make([]BuildConversationMessage, len(build.Interaction.Messages))
	copy(copyMessages, build.Interaction.Messages)

	copyNotes := make([]string, len(build.Interaction.SteeringNotes))
	copy(copyNotes, build.Interaction.SteeringNotes)

	copyPendingRevisions := make([]string, len(build.Interaction.PendingRevisions))
	copy(copyPendingRevisions, build.Interaction.PendingRevisions)

	copyRules := make([]BuildPermissionRule, len(build.Interaction.PermissionRules))
	copy(copyRules, build.Interaction.PermissionRules)

	copyRequests := make([]BuildPermissionRequest, len(build.Interaction.PermissionRequests))
	copy(copyRequests, build.Interaction.PermissionRequests)

	copyApprovalEvents := make([]BuildApprovalEvent, len(build.Interaction.ApprovalEvents))
	copy(copyApprovalEvents, build.Interaction.ApprovalEvents)

	return BuildInteractionState{
		Messages:           copyMessages,
		SteeringNotes:      copyNotes,
		PendingRevisions:   copyPendingRevisions,
		PendingQuestion:    build.Interaction.PendingQuestion,
		WaitingForUser:     build.Interaction.WaitingForUser,
		Paused:             build.Interaction.Paused,
		PauseReason:        build.Interaction.PauseReason,
		PermissionRules:    copyRules,
		PermissionRequests: copyRequests,
		ApprovalEvents:     copyApprovalEvents,
		AttentionRequired:  build.Interaction.AttentionRequired,
	}
}

func copyBuildActivityTimelineLocked(build *Build) []BuildActivityEntry {
	if build == nil || len(build.ActivityTimeline) == 0 {
		return []BuildActivityEntry{}
	}

	timeline := make([]BuildActivityEntry, len(build.ActivityTimeline))
	copy(timeline, build.ActivityTimeline)
	return timeline
}

func appendBuildActivityEntryLocked(build *Build, entry BuildActivityEntry) {
	if build == nil {
		return
	}
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	build.ActivityTimeline = append(build.ActivityTimeline, entry)
	if len(build.ActivityTimeline) > maxBuildActivityEntries {
		build.ActivityTimeline = append([]BuildActivityEntry(nil), build.ActivityTimeline[len(build.ActivityTimeline)-maxBuildActivityEntries:]...)
	}
}

func appendBuildConversationMessageLocked(build *Build, msg BuildConversationMessage) BuildConversationMessage {
	if build == nil {
		return msg
	}
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now().UTC()
	}
	if msg.Status == "" {
		msg.Status = "sent"
	}

	build.Interaction.Messages = append(build.Interaction.Messages, msg)
	if len(build.Interaction.Messages) > maxBuildConversationMessages {
		build.Interaction.Messages = append([]BuildConversationMessage(nil), build.Interaction.Messages[len(build.Interaction.Messages)-maxBuildConversationMessages:]...)
	}
	refreshInteractionAttentionLocked(build)
	return msg
}

func appendSteeringNoteLocked(build *Build, note string) {
	if build == nil {
		return
	}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return
	}
	for _, existing := range build.Interaction.SteeringNotes {
		if strings.EqualFold(strings.TrimSpace(existing), trimmed) {
			return
		}
	}
	build.Interaction.SteeringNotes = append(build.Interaction.SteeringNotes, trimmed)
	if len(build.Interaction.SteeringNotes) > maxBuildSteeringNotes {
		build.Interaction.SteeringNotes = append([]string(nil), build.Interaction.SteeringNotes[len(build.Interaction.SteeringNotes)-maxBuildSteeringNotes:]...)
	}
}

func appendPendingRevisionLocked(build *Build, request string) {
	if build == nil {
		return
	}
	trimmed := strings.TrimSpace(request)
	if trimmed == "" {
		return
	}
	for _, existing := range build.Interaction.PendingRevisions {
		if strings.EqualFold(strings.TrimSpace(existing), trimmed) {
			return
		}
	}
	build.Interaction.PendingRevisions = append(build.Interaction.PendingRevisions, trimmed)
	if len(build.Interaction.PendingRevisions) > maxPendingRevisions {
		build.Interaction.PendingRevisions = append([]string(nil), build.Interaction.PendingRevisions[len(build.Interaction.PendingRevisions)-maxPendingRevisions:]...)
	}
}

func appendBuildApprovalEventLocked(build *Build, event BuildApprovalEvent) BuildApprovalEvent {
	if build == nil {
		return event
	}
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	build.Interaction.ApprovalEvents = append(build.Interaction.ApprovalEvents, event)
	if len(build.Interaction.ApprovalEvents) > maxBuildApprovalEvents {
		build.Interaction.ApprovalEvents = append([]BuildApprovalEvent(nil), build.Interaction.ApprovalEvents[len(build.Interaction.ApprovalEvents)-maxBuildApprovalEvents:]...)
	}
	return event
}

func refreshInteractionAttentionLocked(build *Build) {
	if build == nil {
		return
	}
	build.Interaction.AttentionRequired = build.Interaction.Paused || build.Interaction.WaitingForUser || hasPendingBlockingPermissionRequestLocked(build)
}

func permissionRuleIndexLocked(build *Build, scope BuildPermissionScope, target string) int {
	if build == nil {
		return -1
	}
	normalizedTarget := strings.TrimSpace(strings.ToLower(target))
	for idx, rule := range build.Interaction.PermissionRules {
		if rule.Scope == scope && strings.TrimSpace(strings.ToLower(rule.Target)) == normalizedTarget {
			return idx
		}
	}
	return -1
}

func permissionDecisionForLocked(build *Build, scope BuildPermissionScope, target string) (BuildPermissionDecision, BuildPermissionMode, bool) {
	idx := permissionRuleIndexLocked(build, scope, target)
	if idx < 0 {
		return PermissionDecisionAsk, "", false
	}
	rule := build.Interaction.PermissionRules[idx]
	return rule.Decision, rule.Mode, true
}

func removePermissionRuleIndexLocked(build *Build, idx int) {
	if build == nil || idx < 0 || idx >= len(build.Interaction.PermissionRules) {
		return
	}
	build.Interaction.PermissionRules = append(build.Interaction.PermissionRules[:idx], build.Interaction.PermissionRules[idx+1:]...)
}

func hasPendingBlockingPermissionRequestLocked(build *Build) bool {
	if build == nil {
		return false
	}
	for _, req := range build.Interaction.PermissionRequests {
		if req.Blocking && req.Status == PermissionRequestPending {
			return true
		}
	}
	return false
}

func resolveWaitingStateLocked(build *Build) {
	if build == nil {
		return
	}
	if strings.TrimSpace(build.Interaction.PendingQuestion) != "" {
		build.Interaction.WaitingForUser = true
	} else {
		build.Interaction.WaitingForUser = hasPendingBlockingPermissionRequestLocked(build)
	}
	refreshInteractionAttentionLocked(build)
}

func recordPermissionRequestLocked(build *Build, req leadMessagePermissionRequest, requestedBy *Agent) (BuildPermissionRequest, bool) {
	if build == nil {
		return BuildPermissionRequest{}, false
	}
	target := strings.TrimSpace(req.Target)
	reason := strings.TrimSpace(req.Reason)
	if target == "" || reason == "" {
		return BuildPermissionRequest{}, false
	}

	scope := normalizePermissionScope(req.Scope)
	if idx := permissionRuleIndexLocked(build, scope, target); idx >= 0 {
		rule := build.Interaction.PermissionRules[idx]
		switch rule.Decision {
		case PermissionDecisionAllow:
			if rule.Mode == PermissionModeOnce {
				removePermissionRuleIndexLocked(build, idx)
			}
			appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s is approved (%s).", scope, target, rule.Mode))
			return BuildPermissionRequest{}, false
		case PermissionDecisionDeny:
			if rule.Mode == PermissionModeOnce {
				removePermissionRuleIndexLocked(build, idx)
			}
			appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s is denied. Do not depend on it.", scope, target))
			return BuildPermissionRequest{}, false
		}
	}

	for _, existing := range build.Interaction.PermissionRequests {
		if existing.Scope == scope &&
			strings.EqualFold(strings.TrimSpace(existing.Target), target) &&
			existing.Status == PermissionRequestPending {
			return existing, false
		}
	}

	request := BuildPermissionRequest{
		ID:             uuid.New().String(),
		Scope:          scope,
		Target:         target,
		Reason:         reason,
		CommandPreview: strings.TrimSpace(req.CommandPreview),
		Blocking:       req.Blocking,
		Status:         PermissionRequestPending,
		RequestedAt:    time.Now().UTC(),
	}
	if requestedBy != nil {
		request.RequestedByID = requestedBy.ID
		request.RequestedByRole = string(requestedBy.Role)
	}

	build.Interaction.PermissionRequests = append(build.Interaction.PermissionRequests, request)
	appendBuildApprovalEventLocked(build, BuildApprovalEvent{
		Kind:       fmt.Sprintf("permission_%s", scope),
		Title:      fmt.Sprintf("Permission request for %s", target),
		Status:     ApprovalEventPending,
		Summary:    reason,
		SourceType: "permission_request",
		SourceID:   request.ID,
		Actor:      strings.TrimSpace(request.RequestedByRole),
		Timestamp:  request.RequestedAt,
	})
	if request.Blocking {
		build.Interaction.WaitingForUser = true
	}
	refreshInteractionAttentionLocked(build)
	return request, true
}

func parseLeadMessagePlan(raw string) leadMessagePlan {
	plan := leadMessagePlan{Reply: strings.TrimSpace(raw)}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return plan
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return plan
	}

	var parsed leadMessagePlan
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &parsed); err != nil {
		return plan
	}

	if strings.TrimSpace(parsed.Reply) == "" {
		parsed.Reply = plan.Reply
	}
	return parsed
}

func buildMessageTargetMatchesAgent(targetMode BuildMessageTargetMode, targetAgentID string, targetAgentRole string, viewer *Agent) bool {
	if viewer == nil {
		return targetMode == BuildMessageTargetLead || targetMode == ""
	}

	switch targetMode {
	case BuildMessageTargetLead:
		return viewer.Role == RoleLead
	case BuildMessageTargetAgent:
		return strings.TrimSpace(targetAgentID) != "" && strings.EqualFold(strings.TrimSpace(targetAgentID), strings.TrimSpace(viewer.ID))
	case BuildMessageTargetRole:
		return strings.TrimSpace(targetAgentRole) != "" && strings.EqualFold(strings.TrimSpace(targetAgentRole), strings.TrimSpace(string(viewer.Role)))
	case BuildMessageTargetAllAgents:
		return true
	default:
		return true
	}
}

func buildConversationMessageVisibleToAgent(msg BuildConversationMessage, viewer *Agent) bool {
	if strings.TrimSpace(string(msg.TargetMode)) == "" {
		return true
	}
	if viewer != nil && viewer.Role == RoleLead {
		return true
	}
	targetMode := normalizeBuildMessageTargetMode(string(msg.TargetMode))
	return buildMessageTargetMatchesAgent(targetMode, msg.TargetAgentID, msg.TargetAgentRole, viewer)
}

func buildConversationSourceLabel(msg BuildConversationMessage) string {
	switch msg.Role {
	case ConversationRoleLead:
		return "lead"
	case ConversationRoleSystem:
		return "system"
	default:
		return "user"
	}
}

func buildConversationTargetLabelLocked(build *Build, msg BuildConversationMessage) string {
	if strings.TrimSpace(string(msg.TargetMode)) == "" {
		return ""
	}
	targetMode := normalizeBuildMessageTargetMode(string(msg.TargetMode))
	switch targetMode {
	case BuildMessageTargetLead:
		return "lead"
	case BuildMessageTargetAgent:
		targetID := strings.TrimSpace(msg.TargetAgentID)
		if targetID == "" {
			return "agent"
		}
		if build != nil {
			if agent := build.Agents[targetID]; agent != nil {
				if agent.Model != "" {
					return fmt.Sprintf("%s (%s)", agent.Role, agent.Model)
				}
				return string(agent.Role)
			}
		}
		return targetID
	case BuildMessageTargetRole:
		if role := strings.TrimSpace(msg.TargetAgentRole); role != "" {
			return role
		}
		return "role"
	case BuildMessageTargetAllAgents:
		return "all_agents"
	default:
		return ""
	}
}

func buildConversationPromptLineLocked(build *Build, msg BuildConversationMessage) string {
	source := buildConversationSourceLabel(msg)
	target := buildConversationTargetLabelLocked(build, msg)
	content := strings.TrimSpace(msg.Content)
	if target != "" {
		return fmt.Sprintf("[%s -> %s] %s", source, target, content)
	}
	return fmt.Sprintf("[%s] %s", source, content)
}

func buildVisibleConversationMessagesLocked(build *Build, viewer *Agent, limit int) []BuildConversationMessage {
	if build == nil || len(build.Interaction.Messages) == 0 {
		return nil
	}

	if limit <= 0 {
		limit = 6
	}

	visible := make([]BuildConversationMessage, 0, limit)
	for idx := len(build.Interaction.Messages) - 1; idx >= 0; idx-- {
		msg := build.Interaction.Messages[idx]
		if !buildConversationMessageVisibleToAgent(msg, viewer) {
			continue
		}
		visible = append(visible, msg)
		if len(visible) == limit {
			break
		}
	}

	for left, right := 0, len(visible)-1; left < right; left, right = left+1, right-1 {
		visible[left], visible[right] = visible[right], visible[left]
	}
	return visible
}

func buildInteractionPromptContext(build *Build, viewer *Agent) string {
	if build == nil {
		return ""
	}

	build.mu.RLock()
	defer build.mu.RUnlock()

	var sections []string

	if len(build.Interaction.SteeringNotes) > 0 {
		var notes strings.Builder
		notes.WriteString("<user_steering>\n")
		for _, note := range build.Interaction.SteeringNotes {
			notes.WriteString("- ")
			notes.WriteString(note)
			notes.WriteString("\n")
		}
		notes.WriteString("</user_steering>")
		sections = append(sections, notes.String())
	}

	if len(build.Interaction.PendingRevisions) > 0 {
		var revisions strings.Builder
		revisions.WriteString("<queued_user_revisions>\n")
		for _, note := range build.Interaction.PendingRevisions {
			revisions.WriteString("- ")
			revisions.WriteString(strings.TrimSpace(note))
			revisions.WriteString("\n")
		}
		revisions.WriteString("</queued_user_revisions>")
		sections = append(sections, revisions.String())
	}

	granted := make([]string, 0, len(build.Interaction.PermissionRules))
	for _, rule := range build.Interaction.PermissionRules {
		if rule.Decision != PermissionDecisionAllow {
			continue
		}
		granted = append(granted, fmt.Sprintf("- %s: %s (%s)", rule.Scope, rule.Target, rule.Mode))
	}
	if len(granted) > 0 {
		sections = append(sections, fmt.Sprintf("<local_permissions>\n%s\n</local_permissions>", strings.Join(granted, "\n")))
	}

	denied := make([]string, 0, len(build.Interaction.PermissionRules))
	for _, rule := range build.Interaction.PermissionRules {
		if rule.Decision != PermissionDecisionDeny {
			continue
		}
		denied = append(denied, fmt.Sprintf("- %s: %s (%s)", rule.Scope, rule.Target, rule.Mode))
	}
	if len(denied) > 0 {
		sections = append(sections, fmt.Sprintf("<restricted_permissions>\n%s\n</restricted_permissions>", strings.Join(denied, "\n")))
	}

	if visibleMessages := buildVisibleConversationMessagesLocked(build, viewer, 8); len(visibleMessages) > 0 {
		var convo strings.Builder
		convo.WriteString("<recent_user_conversation>\n")
		for _, msg := range visibleMessages {
			convo.WriteString(buildConversationPromptLineLocked(build, msg))
			convo.WriteString("\n")
		}
		convo.WriteString("</recent_user_conversation>")
		sections = append(sections, convo.String())
	}

	return strings.Join(sections, "\n\n")
}

func (am *AgentManager) GetBuildInteraction(buildID string) (BuildInteractionState, error) {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return BuildInteractionState{}, err
	}

	build.mu.RLock()
	defer build.mu.RUnlock()
	return copyBuildInteractionStateLocked(build), nil
}

func (am *AgentManager) GetBuildMessages(buildID string) ([]BuildConversationMessage, error) {
	interaction, err := am.GetBuildInteraction(buildID)
	if err != nil {
		return nil, err
	}
	return interaction.Messages, nil
}

func (am *AgentManager) broadcastInteractionUpdate(buildID string, interaction BuildInteractionState) {
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildInteraction,
		BuildID:   buildID,
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"interaction": interaction,
		},
	})
}

func (am *AgentManager) PauseBuild(buildID string, reason string) (BuildInteractionState, error) {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return BuildInteractionState{}, err
	}

	now := time.Now().UTC()
	build.mu.Lock()
	if build.Status == BuildCompleted || build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		return BuildInteractionState{}, fmt.Errorf("build %s already in terminal state: %s", buildID, build.Status)
	}
	if build.Interaction.Paused {
		interaction := copyBuildInteractionStateLocked(build)
		build.mu.Unlock()
		return interaction, nil
	}
	build.Interaction.Paused = true
	build.Interaction.PauseReason = strings.TrimSpace(reason)
	build.UpdatedAt = now
	msg := appendBuildConversationMessageLocked(build, BuildConversationMessage{
		Role:      ConversationRoleSystem,
		Kind:      ConversationKindDirective,
		Content:   "Build paused by user",
		Timestamp: now,
	})
	_ = msg
	interaction := copyBuildInteractionStateLocked(build)
	build.mu.Unlock()

	am.persistBuildSnapshot(build, nil)
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildFSMPaused,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"reason":      interaction.PauseReason,
			"interaction": interaction,
		},
	})
	am.broadcastInteractionUpdate(buildID, interaction)
	return interaction, nil
}

func (am *AgentManager) ResumeBuild(buildID string, reason string) (BuildInteractionState, error) {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return BuildInteractionState{}, err
	}

	now := time.Now().UTC()
	build.mu.Lock()
	if build.Status == BuildCompleted || build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		return BuildInteractionState{}, fmt.Errorf("build %s already in terminal state: %s", buildID, build.Status)
	}
	if !build.Interaction.Paused {
		interaction := copyBuildInteractionStateLocked(build)
		build.mu.Unlock()
		return interaction, nil
	}
	build.Interaction.Paused = false
	build.Interaction.PauseReason = ""
	build.UpdatedAt = now
	appendBuildConversationMessageLocked(build, BuildConversationMessage{
		Role:      ConversationRoleSystem,
		Kind:      ConversationKindDirective,
		Content:   "Build resumed",
		Timestamp: now,
	})
	resolveWaitingStateLocked(build)
	interaction := copyBuildInteractionStateLocked(build)
	build.mu.Unlock()

	am.resumeBuildExecution(build, true)
	am.persistBuildSnapshot(build, nil)
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildFSMResumed,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"reason":      strings.TrimSpace(reason),
			"interaction": interaction,
		},
	})
	am.broadcastInteractionUpdate(buildID, interaction)
	return interaction, nil
}

func (am *AgentManager) SetProviderModelOverride(buildID string, provider ai.AIProvider, model string) error {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	normalizedModel := normalizeProviderModelOverride(provider, model)

	build.mu.Lock()
	if build.Status == BuildCompleted || build.Status == BuildFailed || build.Status == BuildCancelled {
		build.mu.Unlock()
		return fmt.Errorf("build %s already in terminal state: %s", buildID, build.Status)
	}
	if build.ProviderModelOverrides == nil {
		build.ProviderModelOverrides = make(map[string]string)
	}
	if normalizedModel == "" {
		delete(build.ProviderModelOverrides, string(provider))
	} else {
		build.ProviderModelOverrides[string(provider)] = normalizedModel
	}
	updatedModel := selectBuildModelForProviderLocked(build, provider)
	for _, agent := range build.Agents {
		if agent == nil || agent.Provider != provider {
			continue
		}
		agent.mu.Lock()
		agent.Model = updatedModel
		agent.UpdatedAt = now
		agent.mu.Unlock()
	}
	build.UpdatedAt = now
	providerLabel := map[ai.AIProvider]string{
		ai.ProviderClaude: "Claude",
		ai.ProviderGPT4:   "ChatGPT",
		ai.ProviderGemini: "Gemini",
		ai.ProviderGrok:   "Grok",
		ai.ProviderOllama: "Local",
	}[provider]
	message := fmt.Sprintf("%s model control returned to Auto", providerLabel)
	if normalizedModel != "" {
		message = fmt.Sprintf("%s locked to %s", providerLabel, normalizedModel)
	}
	appendBuildConversationMessageLocked(build, BuildConversationMessage{
		Role:      ConversationRoleSystem,
		Kind:      ConversationKindDirective,
		Content:   message,
		Timestamp: now,
	})
	overrides := cloneStringMap(build.ProviderModelOverrides)
	build.mu.Unlock()

	am.persistBuildSnapshot(build, nil)
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"user_update":              true,
			"message":                  message,
			"provider":                 string(provider),
			"model":                    firstNonEmptyString(normalizedModel, "auto"),
			"provider_model_overrides": overrides,
		},
	})
	return nil
}

func (am *AgentManager) SetPermissionRule(buildID string, scope BuildPermissionScope, target string, decision BuildPermissionDecision, mode BuildPermissionMode, reason string) (BuildInteractionState, *BuildPermissionRule, error) {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return BuildInteractionState{}, nil, err
	}

	normalizedTarget := strings.TrimSpace(target)
	if normalizedTarget == "" {
		return BuildInteractionState{}, nil, fmt.Errorf("permission target is required")
	}

	now := time.Now().UTC()
	build.mu.Lock()
	idx := permissionRuleIndexLocked(build, scope, normalizedTarget)
	var rule BuildPermissionRule
	if idx >= 0 {
		rule = build.Interaction.PermissionRules[idx]
		rule.Decision = decision
		rule.Mode = mode
		rule.Reason = strings.TrimSpace(reason)
		rule.UpdatedAt = now
		build.Interaction.PermissionRules[idx] = rule
	} else {
		rule = BuildPermissionRule{
			ID:        uuid.New().String(),
			Scope:     scope,
			Target:    normalizedTarget,
			Decision:  decision,
			Mode:      mode,
			Reason:    strings.TrimSpace(reason),
			CreatedAt: now,
			UpdatedAt: now,
		}
		build.Interaction.PermissionRules = append(build.Interaction.PermissionRules, rule)
	}

	appendBuildConversationMessageLocked(build, BuildConversationMessage{
		Role:      ConversationRoleSystem,
		Kind:      ConversationKindPermissionUpdate,
		Content:   fmt.Sprintf("Permission %s for %s:%s (%s)", decision, scope, normalizedTarget, mode),
		Timestamp: now,
	})
	build.UpdatedAt = now
	resolveWaitingStateLocked(build)
	interaction := copyBuildInteractionStateLocked(build)
	build.mu.Unlock()

	am.persistBuildSnapshot(build, nil)
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildPermissionUpdate,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"rule":        rule,
			"interaction": interaction,
		},
	})
	am.broadcastInteractionUpdate(buildID, interaction)
	return interaction, &rule, nil
}

func (am *AgentManager) ResolvePermissionRequest(buildID string, requestID string, decision BuildPermissionDecision, mode BuildPermissionMode, note string) (BuildInteractionState, *BuildPermissionRequest, error) {
	build, err := am.GetBuild(buildID)
	if err != nil {
		return BuildInteractionState{}, nil, err
	}

	now := time.Now().UTC()
	build.mu.Lock()
	var updated *BuildPermissionRequest
	for idx := range build.Interaction.PermissionRequests {
		req := build.Interaction.PermissionRequests[idx]
		if req.ID != requestID {
			continue
		}
		req.Status = PermissionRequestDenied
		if decision == PermissionDecisionAllow {
			req.Status = PermissionRequestAllowed
		}
		req.Mode = mode
		req.ResolutionNote = strings.TrimSpace(note)
		req.ResolvedAt = &now
		build.Interaction.PermissionRequests[idx] = req
		updated = &build.Interaction.PermissionRequests[idx]

		if mode == PermissionModeBuild {
			existingRuleIdx := permissionRuleIndexLocked(build, req.Scope, req.Target)
			if existingRuleIdx >= 0 {
				build.Interaction.PermissionRules[existingRuleIdx].Decision = decision
				build.Interaction.PermissionRules[existingRuleIdx].Mode = mode
				build.Interaction.PermissionRules[existingRuleIdx].Reason = strings.TrimSpace(req.Reason)
				build.Interaction.PermissionRules[existingRuleIdx].UpdatedAt = now
			} else {
				build.Interaction.PermissionRules = append(build.Interaction.PermissionRules, BuildPermissionRule{
					ID:        uuid.New().String(),
					Scope:     req.Scope,
					Target:    req.Target,
					Decision:  decision,
					Mode:      mode,
					Reason:    strings.TrimSpace(req.Reason),
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
		}

		if decision == PermissionDecisionAllow {
			if mode == PermissionModeOnce {
				appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s is approved once. Do not assume it remains available.", req.Scope, req.Target))
			} else {
				appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s is approved for this build.", req.Scope, req.Target))
			}
		} else {
			if mode == PermissionModeOnce {
				appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s was denied once. Do not depend on it without asking again.", req.Scope, req.Target))
			} else {
				appendSteeringNoteLocked(build, fmt.Sprintf("Local %s access for %s was denied for this build. Do not depend on it.", req.Scope, req.Target))
			}
		}

		appendBuildConversationMessageLocked(build, BuildConversationMessage{
			Role:      ConversationRoleSystem,
			Kind:      ConversationKindPermissionUpdate,
			Content:   fmt.Sprintf("Permission request for %s:%s %s", req.Scope, req.Target, req.Status),
			Timestamp: now,
		})
		eventStatus := ApprovalEventDenied
		if req.Status == PermissionRequestAllowed {
			eventStatus = ApprovalEventSatisfied
		}
		appendBuildApprovalEventLocked(build, BuildApprovalEvent{
			Kind:       fmt.Sprintf("permission_%s", req.Scope),
			Title:      fmt.Sprintf("Permission request for %s", req.Target),
			Status:     eventStatus,
			Summary:    strings.TrimSpace(req.ResolutionNote),
			SourceType: "permission_request",
			SourceID:   req.ID,
			Actor:      "user",
			Timestamp:  now,
		})
		break
	}

	if updated == nil {
		build.mu.Unlock()
		return BuildInteractionState{}, nil, fmt.Errorf("permission request %s not found", requestID)
	}

	build.UpdatedAt = now
	resolveWaitingStateLocked(build)
	interaction := copyBuildInteractionStateLocked(build)
	build.mu.Unlock()

	am.persistBuildSnapshot(build, nil)
	am.broadcast(buildID, &WSMessage{
		Type:      WSBuildPermissionUpdate,
		BuildID:   buildID,
		Timestamp: now,
		Data: map[string]any{
			"request":     updated,
			"interaction": interaction,
		},
	})
	am.broadcastInteractionUpdate(buildID, interaction)
	return interaction, updated, nil
}

type revisionEnqueueOptions struct {
	restartRecovery bool
}

func (am *AgentManager) enqueueUserRevisionTask(build *Build, userRequest string) error {
	return am.enqueueRevisionTask(build, userRequest, revisionEnqueueOptions{})
}

func (am *AgentManager) enqueueRestartRecoveryTask(build *Build, userRequest string) error {
	return am.enqueueRevisionTask(build, userRequest, revisionEnqueueOptions{restartRecovery: true})
}

func (am *AgentManager) enqueueRevisionTask(build *Build, userRequest string, opts revisionEnqueueOptions) error {
	if build == nil {
		return fmt.Errorf("build is required")
	}

	trimmed := strings.TrimSpace(userRequest)
	if trimmed == "" {
		return fmt.Errorf("user request is required")
	}

	if build.Status == BuildCancelled {
		return fmt.Errorf("build %s was cancelled and cannot accept follow-up changes", build.ID)
	}

	now := time.Now().UTC()
	previousStatus := BuildPending
	build.mu.RLock()
	previousStatus = build.Status
	build.mu.RUnlock()

	// Compute semantic diff hint before acquiring write lock — collectGeneratedFiles
	// takes its own read lock internally so this is safe.
	taskInput := map[string]any{
		"action":                   "user_change_request",
		"user_request":             trimmed,
		"app_description":          build.Description,
		"requires_regression_test": true,
	}
	if !opts.restartRecovery {
		allFiles := am.collectGeneratedFiles(build)
		hint := computeSemanticDiffHint(build, allFiles, trimmed)
		if !hint.Uncertainty && len(hint.AffectedFiles) > 0 {
			taskInput["affected_files"] = hint.AffectedFiles
			taskInput["scope_hint"] = "targeted"
		} else {
			taskInput["scope_hint"] = "full"
		}
	}

	task := &Task{
		ID:          uuid.New().String(),
		Type:        TaskFix,
		Description: fmt.Sprintf("Apply user-requested changes: %s", trimmed),
		Priority:    98,
		Status:      TaskPending,
		MaxRetries:  build.MaxRetries,
		Input:       taskInput,
		CreatedAt:   now,
	}

	// Enrich explicit restart tasks with build failure context so the solver
	// knows what broke. Normal queued revisions during an active build must stay
	// as user_change_request tasks; otherwise follow-up edits are misclassified as
	// aggressive restart recoveries.
	if opts.restartRecovery {
		task.Input["action"] = "restart_failed_build"
		task.Priority = 999

		build.mu.RLock()
		buildErr := build.Error
		var failedSummaries []string
		var incompleteTypes []string
		seenTypes := map[string]bool{}
		for _, t := range build.Tasks {
			if t == nil {
				continue
			}
			if t.Status == TaskFailed || t.Status == TaskCancelled {
				errSnippet := ""
				if t.Error != "" {
					msg := t.Error
					if len(msg) > 200 {
						msg = msg[:200] + "..."
					}
					errSnippet = ": " + msg
				}
				failedSummaries = append(failedSummaries, fmt.Sprintf("%s (%s)%s", t.Type, t.Description, errSnippet))
			}
			if t.Status == TaskPending || t.Status == TaskFailed || t.Status == TaskCancelled {
				key := string(t.Type)
				if !seenTypes[key] {
					seenTypes[key] = true
					incompleteTypes = append(incompleteTypes, key)
				}
			}
		}
		build.mu.RUnlock()

		if buildErr != "" {
			task.Input["build_error"] = buildErr
		}
		if len(failedSummaries) > 0 {
			task.Input["failed_task_summaries"] = failedSummaries
		}
		if len(incompleteTypes) > 0 {
			task.Input["incomplete_task_types"] = incompleteTypes
		}
		if task.MaxRetries < 4 {
			task.MaxRetries = 4
		}
	}

	build.mu.Lock()
	build.Status = BuildInProgress
	build.CompletedAt = nil
	build.Error = ""
	if build.Progress > 96 {
		build.Progress = 96
	}
	build.Tasks = append(build.Tasks, task)
	build.UpdatedAt = now
	build.mu.Unlock()

	assignee := am.ensureProblemSolverAgent(build.ID)
	if assignee == nil {
		assignee = am.selectFixAgent(build, []AgentRole{RoleSolver, RoleBackend, RoleFrontend, RoleDatabase, RoleReviewer})
	}
	if assignee == nil {
		return fmt.Errorf("no agent is available to apply the requested changes")
	}

	broadcastMsg := "User requested changes. Launching a follow-up implementation pass."
	phase := "user_feedback"
	qualityGateStage := "revision"
	restartRecovery := false
	if action, _ := task.Input["action"].(string); action == "restart_failed_build" {
		broadcastMsg = "Build restart triggered. Launching aggressive recovery — fixing all failures and resuming pipeline."
		phase = "restart_recovery"
		qualityGateStage = "Recovery"
		restartRecovery = true
	}

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: now,
		Data: map[string]any{
			"phase":                 phase,
			"phase_key":             phase,
			"status":                string(BuildInProgress),
			"message":               broadcastMsg,
			"restart_recovery":      restartRecovery,
			"quality_gate_required": true,
			"quality_gate_active":   true,
			"quality_gate_stage":    qualityGateStage,
		},
	})

	if err := am.AssignTask(assignee.ID, task); err != nil {
		return err
	}

	am.persistBuildSnapshot(build, nil)
	if previousStatus == BuildCompleted || previousStatus == BuildFailed || previousStatus == BuildCancelled {
		am.startBuildMonitors(build.ID)
	}
	return nil
}

func (am *AgentManager) dispatchPendingRevision(build *Build) bool {
	if build == nil {
		return false
	}

	build.mu.Lock()
	if len(build.Interaction.PendingRevisions) == 0 || build.Status == BuildCancelled {
		build.mu.Unlock()
		return false
	}
	queued := append([]string(nil), build.Interaction.PendingRevisions...)
	build.Interaction.PendingRevisions = nil
	build.UpdatedAt = time.Now().UTC()
	interaction := copyBuildInteractionStateLocked(build)
	build.mu.Unlock()

	request := strings.Join(queued, "\n\n")
	if err := am.enqueueUserRevisionTask(build, request); err != nil {
		build.mu.Lock()
		for i := len(queued) - 1; i >= 0; i-- {
			build.Interaction.PendingRevisions = append([]string{queued[i]}, build.Interaction.PendingRevisions...)
		}
		if strings.TrimSpace(build.Error) == "" {
			build.Error = fmt.Sprintf("Unable to schedule queued revision work: %v", err)
		}
		build.Status = BuildFailed
		now := time.Now().UTC()
		build.CompletedAt = &now
		build.UpdatedAt = time.Now().UTC()
		interaction = copyBuildInteractionStateLocked(build)
		build.mu.Unlock()
		am.persistBuildSnapshot(build, nil)
		log.Printf("Build %s: failed to dispatch queued revision: %v", build.ID, err)
		am.broadcast(build.ID, &WSMessage{
			Type:      WSBuildError,
			BuildID:   build.ID,
			Timestamp: time.Now().UTC(),
			Data: map[string]any{
				"status":      string(BuildFailed),
				"error":       "Queued revision could not be scheduled",
				"details":     err.Error(),
				"recoverable": false,
				"interaction": interaction,
			},
		})
		return false
	}

	am.broadcast(build.ID, &WSMessage{
		Type:      WSBuildProgress,
		BuildID:   build.ID,
		Timestamp: time.Now().UTC(),
		Data: map[string]any{
			"phase":                 "user_feedback",
			"status":                string(BuildInProgress),
			"message":               "Applying queued user-requested changes in a follow-up implementation pass.",
			"quality_gate_required": true,
			"quality_gate_active":   true,
			"quality_gate_stage":    "revision",
			"interaction":           interaction,
		},
	})
	return true
}

func (am *AgentManager) waitForBuildInteractionClear(build *Build) error {
	if build == nil {
		return fmt.Errorf("build is required")
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		build.mu.RLock()
		blocked := build.Interaction.Paused || build.Interaction.WaitingForUser
		cancelled := build.Status == BuildCancelled || build.Status == BuildFailed
		build.mu.RUnlock()

		if cancelled {
			return errBuildNotActive
		}
		if !blocked {
			return nil
		}

		select {
		case <-am.ctx.Done():
			return am.ctx.Err()
		case <-ticker.C:
		}
	}
}
