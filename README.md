A TUI in the style of K9S for Chainguard

To use, clone the repo and run make in the project root, then ./chaintui

Usage is pretty limited as yet and it relies on chainctl for auth (or `CHAINGUARD_TOKEN`). If you're not logged in, chaintui will offer to run `chainctl auth login` before starting.

Everything is scoped to an organisation, so the **org picker is the first screen** — pick one and its menu opens beneath it (`esc` goes back to the picker and clears the org). The org menu covers **repos** → tags → SBOMs, **charts**, **libraries**, **librariespolicy**, **advisories**, plus the IAM lists (folders, identities, roles, rolebindings, idps, invites). `:` commands are scoped to the selected org: `:repos`, `:charts`, `:libraries`, `:libpolicy`, `:blocked`, `:adv`, `:orgs` to go back to the picker.

**Charts** are the Helm charts in the org's chart catalog folders (`charts`, `iamguarded-charts`) — the registry API has no chart list call, so chaintui resolves those folders and lists the repos inside. Enter drills into a chart's tags.

**Libraries** lists Java, Python and JavaScript with the org's policy posture on each row — entitled or not, which sources it may pull (`chainguard` vs `chainguard+upstream`), the entitlement cooldown, and the bound policy and its mode. `d` shows the entitlement and bindings in full. Enter browses that ecosystem's artifacts → versions (`m` toggles remediated-only, `/` searches, `x` exports — see below). The artifact catalogue itself is global, so it still browses with no org selected; the policy columns are then blank.

**librariespolicy** is the org's Libraries policy in four views: `entitlements` (which ecosystems, from where, cooldown, trial/SFDC), `policies` (system and custom gates — cooldown, blocked licences, block/allow counts, with `d` for the PURL lists and a system policy's Rego), `bindings` (which policy is active per ecosystem and whether it enforces or only logs), and `blocked` (packages actually withheld: reason, attempts, and when a cooldown block lifts; `l` switches to log-mode violations, `/` filters by exact package name).

You can save a CSV of the SBOM too.

On a libraries artifacts page, `x` exports the whole ecosystem — every package and all of its versions — to `<timestamp>-<ecosystem>.json` in the working directory (e.g. `20260818T143000Z-python.json`; `-remediated` is appended when that toggle is on). This walks the full catalogue rather than the current page, so it takes a while: python is ~18k packages / ~5 minutes, javascript ~32k, java ~85k / tens of minutes. Progress shows in the footer and `x` again cancels; a package whose versions fail to fetch is recorded with an `error` field and counted in `errorCount` instead of failing the export.

**Roles** shows the org's own custom roles first, then Chainguard's 63 built-in roles — those live outside the group hierarchy, so scoping to the org alone shows nothing (as `chainctl iam roles list` does without `--managed`). `c` hides the built-ins. Capabilities are shown in the canonical `advisories.create` form.

Lists use Chainguard API v2beta1 cursor pagination. Use `[` / `]` to move between API pages. `/` filter: advisories and libraries use free-text server query; repos/tags/identities/groups/roles use exact name. `o` sort uses server `order_by` for mapped columns (name, create_time, …); other columns stay local to the current page.
