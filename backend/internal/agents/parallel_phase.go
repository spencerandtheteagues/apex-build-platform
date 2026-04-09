package agents

func buildExecutionPhasesParallel(
	archAgents, frontendAgents, dbAgents, backendAgents, testAgents, reviewAgents []agentPriority,
) []executionPhase {
	coreAgents := make([]agentPriority, 0, len(frontendAgents)+len(dbAgents)+len(backendAgents))
	coreAgents = append(coreAgents, frontendAgents...)
	coreAgents = append(coreAgents, dbAgents...)
	coreAgents = append(coreAgents, backendAgents...)

	return []executionPhase{
		{
			name:              "Architecture",
			key:               "architecture",
			status:            BuildInProgress,
			agents:            archAgents,
			startMessage:      "Freezing the scaffold, product flow, and backend contract before implementation begins.",
			completionMessage: "Contract is frozen. Launching frontend, data, and backend implementation in parallel against the same contract.",
		},
		{
			name:              "Parallel Core",
			key:               "parallel_core",
			status:            BuildInProgress,
			agents:            coreAgents,
			startMessage:      "Building the frontend first while data and backend implementation start in parallel behind the frozen contract.",
			completionMessage: "Frontend, data, and backend core work are complete. Starting integration and verification.",
		},
		{
			name:              "Integration",
			key:               "integration",
			status:            BuildTesting,
			qualityStage:      "testing",
			agents:            testAgents,
			startMessage:      "Connecting the UI to backend contracts and checking the main user flow.",
			completionMessage: "Integration checks are complete. Moving into final review.",
		},
		{
			name:              "Review",
			key:               "review",
			status:            BuildReviewing,
			qualityStage:      "review",
			agents:            reviewAgents,
			startMessage:      "Running the final quality review before handoff.",
			completionMessage: "Final review finished. Preparing the build for handoff.",
		},
	}
}
