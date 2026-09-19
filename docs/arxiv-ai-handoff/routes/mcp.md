# Route: MCP integration

Path: `/mcp`
Evidence: Documented; not connected
Purpose: Expose paper research tools to an external agent.
Implementation status: investigate_only
Source: https://arxiv.gg/mcp

## Locate and investigate

Documented tools: arxiv_api_overview, arxiv_account, arxiv_search, arxiv_get_paper, arxiv_related_papers. Public tools need no login; account and Deep Search need a session cookie or API key according to the setup text.

No application code paths have been verified. Follow existing repository instructions; preserve unrelated changes.

## Proposed work

- Verify the tool schemas and connection in the receiving agent environment.
- Use public tools first; only use account credentials in the appropriate authenticated context.

## Existing interfaces to investigate

No REST dependency established. Discover the current implementation; do not invent an endpoint.

Documented MCP tools: arxiv_api_overview, arxiv_account, arxiv_search, arxiv_get_paper, arxiv_related_papers.

## Linked implementation tasks

No defect established for this route. Investigate first; add a new stable issue ID only with evidence.

## Agent deliverable

Report current reproduction, cause if established, changed files, checks and results, acceptance criteria met or blocked, and commit/PR when available. Update the existing issue records and revision log. Separate application implementation from report edits and from deployment. Original observations are historical, not proof of current behavior.
