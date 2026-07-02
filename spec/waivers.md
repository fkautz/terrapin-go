# Terrapin (Go) Requirement Waivers

Requirements deliberately not covered by an automated test, with an allowed
reason and rationale. A requirement is either tested or waived, never both.

### REQ-SEC-005
- Reason: foundational
- Rationale: Second-preimage resistance is inherited from SHA-256 and the length-binding manifest (covered indirectly by REQ-ID-006 and REQ-SEC-001); there is no in-suite way to demonstrate collision resistance directly.
