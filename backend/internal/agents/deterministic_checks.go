package agents

const (
	verificationReasonDeterministicFailed  = "deterministic_failed"
	verificationReasonDeterministicPassed  = "deterministic_passed"
	verificationReasonProviderCritiqueNeed = "provider_critique_needed"
	verificationReasonProviderCritiqueSkip = "provider_critique_skipped"
)

type deterministicVerificationResult struct {
	Ran                    bool
	Checks                 []string
	Warnings               []string
	Errors                 []string
	DeterministicStatus    string
	ProviderCritiqueStatus string
}

func (am *AgentManager) evaluateDeterministicVerification(build *Build, task *Task, candidate *taskGenerationCandidate) deterministicVerificationResult {
	result := deterministicVerificationResult{
		DeterministicStatus:    verificationReasonDeterministicPassed,
		ProviderCritiqueStatus: verificationReasonProviderCritiqueNeed,
	}
	if am == nil || build == nil || task == nil || candidate == nil {
		return result
	}

	if !candidate.VerifyPassed && len(candidate.VerifyErrors) > 0 {
		result.Ran = true
		result.Checks = append(result.Checks, "deterministic:verify_generated_code")
		result.Errors = append(result.Errors, candidate.VerifyErrors...)
		result.DeterministicStatus = verificationReasonDeterministicFailed
		result.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
		return result
	}

	surface := am.runSurfaceDeterministicChecks(build, task, candidate)
	result.Ran = surface.Ran
	result.Checks = append(result.Checks, surface.Checks...)
	result.Warnings = append(result.Warnings, surface.Warnings...)
	result.Errors = append(result.Errors, surface.Errors...)
	if len(surface.Errors) > 0 {
		result.DeterministicStatus = verificationReasonDeterministicFailed
		result.ProviderCritiqueStatus = verificationReasonProviderCritiqueSkip
		return result
	}
	return result
}
