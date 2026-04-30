A TUI in the style of K9S for Chainguard

To use, clone the repo and run make in the project root, then ./chaintui

Usage is pretty limited as yet and it relies on chainctl for auth.  Once you're in, you should see a list of Orgs you're associated with.  Drill Down on that and you then have to type a ':' command e.g. ':repos' will switch you to the Image Repos. From there you can drill down to tags, and the to SBOMS.

You can save a CSV of the SBOM too.

Haven't added pagination support yet, so Advisories times out trying to download all the CG Advisories.
