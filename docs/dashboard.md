# Dashboard configuration and behavior


```sh
cp blip.example.json blip.local.json
# Edit your owners, urgent issue query, and approval threshold.
make run
```

`blip.local.json` is ignored by Git. GitHub credentials stay in your existing `gh`
login, on the computer. The device receives titles and repository/item IDs, never tokens.

Configure `owners` with GitHub organization/user names to include all their
repositories accessible to your `gh` login. New repositories are discovered on
each refresh from your open non-draft PRs and review requests. You can also add
explicit `repositories` (`owner/repo`); overlapping repositories are deduplicated.
Archived repositories are excluded from PRs and urgent issues.
Set `include_dependabot` to `true` to treat `dependabot[bot]` PRs as your own,
using the same Mine/Merge, approval, conflict, and CI rules. PRs that also request
your review appear once in Mine/Merge. Dependabot PRs assigned solely to other
people are hidden (including from Review); unassigned PRs and PRs assigned to
you remain visible.
Urgent issues are disabled in the example configuration. They use their own
`urgent_query` scope; use `(org:YOUR_ORG OR user:YOUR_NAME)` for both owners.

- **M / Merge:** your non-draft PRs with enough effective approvals, GitHub's
  approved review decision, or **no outstanding review requests**. This is your
  personal merge queue, not a claim that all branch protections pass.
  A changes-requested decision, failing check, or merge conflict keeps the PR in Mine.
  Conflicts show a red `CONFLICT` badge; approvals and CI remain in the footer.
- **O / Mine:** your other non-draft PRs, awaiting reviewers, changes, or CI fixes.
  Both own-PR groups show `n/n` approvals, `L+`/`L-` for the actual LGTM label,
  and CI status: `+` successful, `~` pending, `?` no checks. Failures take
  priority in the row badge (`n/n FAIL`), with LGTM still in the footer.
  PRs with queued/running CI show a spinner, including those with a failed
  job and other jobs still running. Only the spinner area animates; unchanged
  refreshes do not repaint the screen. Animation stops when data is stale.
  Highlight an item to see its number, abbreviated repository, and compact status
  together on one footer line. Long repository names shorten before status; the
  list shows ten rows at a time. Long selected titles scroll after a short
  reading pause, repainting only the title area. Turn past either end of the list
  to clear the highlight and stop title scrolling; turn back to select again.
  Refreshes preserve that unselected state, and clicking it opens nothing. A successful
  LGTM-label workflow does not itself count as the LGTM label.
- **R / Review:** `is:open -is:draft review-requested:@me`, sorted by **least
  recently updated**, with fewer than the configured number of approvals.
  All review pages are fetched; duplicate approvals from one person count once.
  Bots and the author are excluded. Comments do not revoke an approval; a later
  changes-requested or dismissed review does. GitHub remains authoritative about
  reviewer eligibility and branch protection.
- **! / Urgent:** open issues whose native **Priority** field is **Urgent**.
  Issues with a native or project Status containing the word **Blocked**
  (including department-prefixed statuses), or **Dev: QA**, are excluded.
  `urgent_query` uses advanced issue search, for example
  `org:YOUR_ORG is:issue is:open field.priority:Urgent sort:updated-asc`.
  To show only issues assigned to you or nobody, add
  `(assignee:@me OR no:assignee)` to the query. Assignment filtering comes from
  this query, not a hard-coded organization or account.
  This works across repositories independently of project membership or labels.
  Returned native Priority values are verified. Leave the query empty to disable
  urgent issues; the urgent count hides when zero.

Clicking an item only opens its GitHub page. It never merges, approves, or edits.

Repositories are fetched with up to four concurrent workers, preserving stable
row ordering. A new refresh starts 30 seconds after the previous fetch completes. Swipe up/down with light pressure on the resistive touchscreen to scroll the
list; touch does not open links. The knob still selects and opens items.
Turning the knob scrolls the single list when
needed. Refreshes preserve the highlighted item by ID. The display marks stale
content STALE DATA; clicking a displayed item still opens its GitHub URL
immediately, even before the first refresh after a companion restart. Links are
constructed from validated `owner/repo/pull/number` or `owner/repo/issues/number`
IDs on the fixed `github.com` host, without checking snapshot age or revision.
A maximum of 60 rows is sent to the device, with truncation shown in the footer.
GitHub CLI queries are capped at 1,000 candidates per category and repository.
Owner discovery rejects searches reaching 1,000 candidates rather than silently
omitting repositories. Newly opened PRs may wait for GitHub search indexing.
The small bitmap font currently uses ASCII, replacing unsupported characters.

```sh
./build/blip -once                    # inspect current GitHub data
./build/blip -port /dev/cu.usbmodem…  # choose a port if auto-detection is ambiguous
```

The companion reconnects after USB disconnects. To run it automatically at login
(requires Python 3 for this installer only):

```sh
python3 scripts/macos-service.py
# Remove the background service:
python3 scripts/macos-service.py --uninstall
```

Logs live in `~/Library/Logs/blip/companion.log`. Stop the service before using
`make probe` or flashing, so only one process owns the serial port:
`launchctl bootout gui/$(id -u)/com.brianleishman.blip`.
Re-run the installer afterward to start it again.

macOS is the primary host; Linux has a basic `xdg-open` path but has not been hardware-tested.
