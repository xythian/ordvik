# RFC Review Checklist

Evaluate the RFC against each criterion. For each, report: **pass**, **concern** (minor issue, non-blocking), or **fail** (must be addressed).

## 1. Completeness

- [ ] All required sections present: Problem, Proposal, Design, Alternatives Considered, Verification Plan
- [ ] Problem section clearly defines *who* is affected and *what* the impact is
- [ ] Proposal section gives a concrete solution (not just "we should improve X")
- [ ] Design section has enough detail that an implementer could start work
- [ ] Alternatives Considered has at least one genuine alternative with real trade-offs
- [ ] Verification Plan describes specific, actionable tests or checks

## 2. Clarity

- [ ] The title accurately describes the RFC's scope
- [ ] The problem can be understood without reading the rest of the RFC
- [ ] The proposal can be summarized in 1-2 sentences
- [ ] Technical terms are defined or linked on first use
- [ ] No ambiguous pronouns or references ("it", "this", "the system" without clear antecedent)

## 3. Feasibility

- [ ] The design is implementable with the project's current technology stack and dependencies
- [ ] External dependencies are identified and their availability/stability assessed
- [ ] Scope is realistic — could be implemented in a reasonable timeframe
- [ ] No unstated assumptions about infrastructure, permissions, or capabilities

## 4. Consistency

- [ ] Aligns with existing architecture documented in `docs/designs/`
- [ ] Does not contradict decisions in other accepted RFCs
- [ ] Terminology is consistent with the rest of the codebase
- [ ] If it supersedes another RFC, changes from the previous design are clear

## 5. Risk Assessment

- [ ] Failure modes are identified and addressed
- [ ] Backwards compatibility impact is documented (if applicable)
- [ ] Security implications are considered (if applicable)
- [ ] Data loss or corruption scenarios are addressed (if applicable)
- [ ] Rollback strategy exists if the implementation fails
