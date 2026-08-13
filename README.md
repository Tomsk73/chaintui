A TUI in the style of K9S for Chainguard

To use, clone the repo and run make in the project root, then ./chaintui

Usage is pretty limited as yet and it relies on chainctl for auth (or `CHAINGUARD_TOKEN`). If you're not logged in, chaintui will offer to run `chainctl auth login` before starting. The home menu offers **groups** (orgs/folders) and **libraries**. Drill into groups then use a `:` command (e.g. `:repos`) for image repos → tags → SBOMs. Libraries: pick Java or Python → artifacts → versions (`m` toggles remediated-only; `/` searches). Or jump with `:libraries`, `:java`, `:python`.

You can save a CSV of the SBOM too.

Lists use Chainguard API v2beta1 cursor pagination. Use `[` / `]` to move between API pages. `/` filter: advisories and libraries use free-text server query; repos/tags/identities/groups/roles use exact name. `o` sort uses server `order_by` for mapped columns (name, create_time, …); other columns stay local to the current page.
