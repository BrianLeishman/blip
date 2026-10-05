# Security and privacy

Blip is a hobby hardware project. Automated scans and code review help catch
mistakes; they do not constitute a security certification.

## Credentials and network access

The companion uses the GitHub CLI's existing authentication. Blip's configuration
contains repository owners and queries, not a token. Keep credentials in the CLI's
credential storage or a local environment, and grant only the access you need.
The CLI login can have broader permissions than Blip uses: Blip itself performs
read-only GitHub queries and opens pages, with no merge, review, or issue mutations.

The current implementation has no application telemetry, hosted backend, or
network listener. It contacts GitHub through `gh`; clicking an item launches the
host's default browser. The board connects through local USB serial.

## USB and browser trust boundary

Treat the connected USB device as trusted local hardware. The companion accepts
open events from it without checking whether the item exists in the newest
snapshot, so clicks continue to work on stale data and after a restart.

Those events contain item IDs, not arbitrary URLs. The companion validates an
`owner/repo/pull/positive-number`, `owner/repo/issues/positive-number`, or
`owner/repo/actions/runs/positive-number` path. Issue/PR paths can additionally
carry one of the supported comment anchors with a positive numeric comment ID. It
constructs an HTTPS URL on the fixed `github.com` host, and invokes the browser
opener directly without a shell. This prevents the device from supplying a shell
command or another URL scheme/host; it does not authenticate the device or stop a
malicious device from repeatedly opening GitHub pages.

## Private work stays private only if you keep its output private

The USB payload and LCD contain titles, repository names, item numbers, review
counts, and status. Item IDs are human-readable repository paths, not anonymized
identifiers. Data remains visible when a refresh fails. The firmware keeps the
snapshot in RAM; it does not deliberately persist it to flash or an SD card.

`-once` prints live GitHub data. Companion logs can contain repository names, item
IDs and API errors. Do not attach raw logs, snapshots, local configuration, or
photos of live work to public issues. Inspect background monitors in photos too.
The README photo uses fictional LCD content and blank background screens.

`.gitignore` excludes `blip.local.json`, other `*.local.json` files, `.env` files,
logs, build output, and the reserved originals-image folder. Ignore rules do not
protect files that are already tracked, and cannot recognize every secret.
Only copy sanitized assets into `docs/images/`.

Comment acknowledgment files are ignored `*.alerts.local.json` files beside the
configuration. They contain repository paths and comment IDs, without comment
bodies or tokens. Delete the file to reset local acknowledgments. GitHub inbox read
status remains separate. Deployment workflow names in local config and live LCD
output may identify private services or brands; keep those local too.

## Before publishing changes

- Inspect the staged files and Git history, including examples and image metadata.
- Run a secret scanner such as [Gitleaks](https://github.com/gitleaks/gitleaks)
  against both history and the candidate working tree.
- Run `make test`, `make firmware`, and
  [govulncheck](https://go.dev/doc/tutorial/govulncheck) for the host Go packages.
  Standard Go scans do not cover TinyGo's hardware-specific firmware build.
- Review screenshots manually for work data; a text secret scanner cannot do that.

Report suspected vulnerabilities privately through GitHub's private vulnerability
reporting feature if it is enabled. Do not put credentials or private work data in
a public issue; revoke any exposed credential through its issuer.
