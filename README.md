A TUI in the style of K9S for Chainguard

To use, clone the repo and run make in the project root, then ./chaintui

Usage is pretty limited as yet and it relies on chainctl for auth (or `CHAINGUARD_TOKEN`). If you're not logged in, chaintui will offer to run `chainctl auth login` before starting. The home menu offers **groups** (orgs/folders) and **libraries**. Drill into groups then use a `:` command (e.g. `:repos`) for image repos → tags → SBOMs. Libraries: pick Java, Python, or JavaScript → artifacts → versions (`m` toggles remediated-only; `/` searches). Or jump with `:libraries`, `:java`, `:python`, `:javascript` / `:npm`.

You can save a CSV of the SBOM too.

On a libraries artifacts page, `x` exports the whole ecosystem — every package and all of its versions — to `<timestamp>-<ecosystem>.json` in the working directory (e.g. `20260818T143000Z-python.json`; `-remediated` is appended when that toggle is on). This walks the full catalogue rather than the current page, so it takes a while: python is ~18k packages / ~5 minutes, javascript ~32k, java ~85k / tens of minutes. Progress shows in the footer and `x` again cancels; a package whose versions fail to fetch is recorded with an `error` field and counted in `errorCount` instead of failing the export.

Lists use Chainguard API v2beta1 cursor pagination. Use `[` / `]` to move between API pages. `/` filter: advisories and libraries use free-text server query; repos/tags/identities/groups/roles use exact name. `o` sort uses server `order_by` for mapped columns (name, create_time, …); other columns stay local to the current page.
