<!-- BARON:MANAGED:START -->
# Baron Automatic Agent Contract

You are using Baron through the DeepSeek Reasonix adapter.

- Before any durable Baron or repository write, silently run `baron authority classify "<requested outcome>"`; classify by the outcome, so `review and apply fixes` is a change request even though it contains `review`.
- Silently run `baron work-shape "<task>" --json` before meaningful work. Read-only tasks create no lifecycle state; focused reversible changes use focused proof; risky, coordinated, ambiguous, or multi-session work keeps the full plan, intent, recovery, gate, proof, and trace path.
- When authority is `read_only` or `ambiguous`, inspect only what is needed to answer and do not create or update plan, Harness, proof, trace, review, friction, or learning state. Ambiguous authority stays read-only until the user explicitly requests a change.
- If Baron reports project, Vault, capsule, schema, or identity mismatch, stop durable work and run `baron automation reconcile`; never repair Baron metadata by hand or continue with guessed state. If local reconciliation cannot restore coherent state, tell the user that a human must run public `baron update`. Never run public `baron update`: only a human may authorize a release download or runtime replacement.
- At session start, silently run `baron capability check --adapter reasonix`; capability presence is not execution evidence.
- At session start, silently run `baron runtime check --adapter reasonix`; unsafe, missing, or unverified required backends cannot satisfy proof.
- At session start, silently run `baron context --reasonix` and follow its bounded context.
- For architecture, dependency, impact, ownership, entrypoint, call-flow, refactor, or cross-module work, silently run task context first. If its Optional Code Map section requests it, silently run `baron automation code-map refresh` and then `baron automation code-map query "<task>"`; verify selected source files before edits, proof, traces, or durable memory. Inferred graph results are reading hints only.
- At session start and after interruption, silently inspect `baron continuity status`; resume from its current task, last checkpoint, proof status, trace status, and next action before guessing.
- At session start and after interruption, silently inspect `baron autopilot status`; treat learning candidates as candidates, not facts.
- Respect `.baron/project.toml` platform focus when present; use it to prioritize domain knowledge without creating new workflow ownership.
- Read `docs/baron/platform/PROJECT_PROFILE.md` and `docs/baron/architecture/CURRENT_ARCHITECTURE.md` before structural work; load only the task-relevant profile.
- For cross-module, product, API, data-model, or interface work, silently inspect `docs/baron/harness/DOMAIN_LANGUAGE.md`. Use canonical terms only when their evidence and status support them; record a new term only when it changes cross-module understanding; mark disagreement as `ambiguous` rather than silently choosing a definition. Product Harness owns this document; Superpowers still owns workflow.
- When the user explicitly expands the product to a new platform, silently run `baron init --<platform>` so Baron adds an extension and regenerates non-destructive architecture contracts; never rearrange existing code automatically.
- Silently inspect `baron plan status` and `baron harness status` before meaningful implementation.
- For a new or materially changed task, read repo, Vault, current plan, Harness, continuity, and decisions before asking; ask exactly one missing high-value question at a time and mark unresolved facts unknown.
- Silently inspect `baron harness intent-status`. Before medium/high-risk intake, record the current/target behavior, scope, proof, constraints, decisions, non-goals, and unknowns with `baron harness intent`; do not pass `--confirmed` until the user explicitly confirms the reflected intent.
- Before selecting skills or agents for meaningful work, silently run `baron control-plane route "<task>"` and follow the selected/skipped explanation.
- Use Baron-managed skills and agents as local self-contained assets; if a custom asset looks weak, conflicting, or externally dependent, run `baron asset audit` before trusting it.
- When prior conversation context may matter, rely on `baron context --reasonix --task "<task>"` or `baron session-replay search "<query>"`; do not dump full session histories.
- Start or resume a Baron plan before editing code for a meaningful task.
- Create harness intake for medium/high-risk work.
- Before edits, direction changes, interruptions, and final responses for meaningful work, record `baron continuity checkpoint "<current state and next action>"`.
- If work fails, blocks, or remains interrupted, silently record `baron continuity recover "<root cause>" --outcome <failed|blocked|interrupted> --last-success "<last successful step>" --next-action "<safe next action>"` with available evidence, affected files, and retry conditions; preserve the failed attempt even after a later retry succeeds.
- Before final response after meaningful work, run `baron autopilot review "<task summary, proof state, remaining risks>"`; it may propose learning, but it must not rewrite trusted facts or runtime assets without approval.
- Use Superpowers as the workflow core for planning, TDD, debugging, review, and verification.
- Read the routed skill and agent indexes; do not recursively load every skill or agent.
- For execution-required proof, use Baron-owned `baron proof execute --capability <capability> --provider <provider> -- <executable> <args...>`; record the returned receipt with `baron proof record "<summary>" --receipt <receipt-id>`. A sentence or hand-written receipt is not execution proof.
- After each mandatory quality gate actually runs, record it with `baron control-plane record-gate <agent> "<evidence summary>" --receipt <receipt-id>`; the legacy form remains reported evidence only.
- For concrete reviewer findings, silently run `baron review finding "<summary>" --severity <level> --evidence "<evidence>"`; keep findings open until the fix exists.
- Close a finding only with `baron review close <id> --fix-evidence "<what changed>" --verification "<command/result>"`; fix evidence and verification are both mandatory.
- After actually running a registered provider, attach structured capability evidence with `baron proof record`; then record and run `baron trace score` before claiming completion.
- Never complete high-risk work when proof is missing or trace quality fails.
- Treat Vault Markdown as durable memory and unknown facts as unknown.
<!-- BARON:MANAGED:END -->
