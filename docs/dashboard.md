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
Set `additional_authors` to a list such as `["your-other-account"]` to treat
those accounts' PRs the same way, including the assignment filter above. This
works independently of `include_dependabot`, within your configured owners and
repositories. Repositories containing only these authors' PRs are discovered too.
Overlapping author and review results appear once; your own account's PRs remain
visible regardless of assignee.
Urgent issues are disabled in the example configuration. They use their own
`urgent_query` scope; use `(org:YOUR_ORG OR user:YOUR_NAME)` for both owners.

Two optional alert types appear first in the same continuous list: failed
deployments, then comments. They do not add pages or change the knob controls.

```json
{
  "comments_since": "2026-10-05T00:00:00Z",
  "deployments": [
    {"repository": "your-org/service", "workflow": "deploy.yml", "branch": "main"}
  ]
}
```

`comments_since` is a fixed initial cutoff, not a moving daily window. Set an
RFC3339 timestamp to enable comments; leave it empty to disable them. Local midnight
may require a UTC offset, for example `2026-10-05T00:00:00-04:00`.
`deployments` is an explicit allowlist of workflow filenames or IDs and production
branches within your configured repository/owner scope. No guessing from workflow
names, and no blanket list of failed Actions runs. Archived repositories are skipped.

- **C / Comments:** one row per unread notification thread, opening its latest
  human issue comment, PR conversation comment, inline review comment, or review
  containing text. Comments must be newer than both the initial cutoff and the
  thread's GitHub read timestamp. Bots, your own comments, empty review votes,
  CI notifications, and state changes without a new human comment are excluded.
  Draft PRs do not produce comment alerts. Standalone PR comment alerts require
  earlier participation (you authored, commented on, or reviewed the PR), or an
  unread human comment directly mentioning your account. An existing PR/issue row
  takes precedence over its comment row, so a queued item appears only once.
  Draft state is checked on each refresh even when comment contents are cached.
  Conflicted PRs that are not treated as yours are also excluded from comment alerts.
  Threads can be mentioned, subscribed, authored, assigned, or otherwise present
  in your unread inbox; the notification reason alone does not prove a new comment.
  The title identifies the commenter and issue/PR. `@YOU` identifies a thread
  with GitHub's mention reason; `NEW` identifies other threads. This does not
  prove the latest comment itself mentions you. Unchanged threads reuse cached
  comment data until GitHub reports new activity or a different read timestamp.
  Discussions are not currently included.
  A successful browser-open command clears that comment alert immediately on blip.
  A later human comment gets a new ID and returns. Acknowledgments persist beside
  your config in an ignored `*.alerts.local.json` file, independently of GitHub's
  inbox; clicking does not mark GitHub notifications read or unsubscribe you.
- **D / Deployments:** the latest meaningful completed result per configured
  workflow and branch. Failure, timeout, startup failure, and action-required
  results show a red `FAIL`; a newer success or cancellation clears the alert.
  Cancellation is not treated as successful deployment, and never creates an alert.
  A retry in progress keeps the prior failure visible. Skipped/neutral runs do not
  hide a prior failure or create an alert. Clicking opens the failed Actions run.
  Only push, manual dispatch, repository dispatch, release,
  and schedule events are eligible. PR events and runs linked to PRs are excluded,
  even if they use the production branch. The latest unfiltered workflow history
  is fetched and branch/event/outcome rules are applied locally, avoiding old
  results from filtered Actions searches. Up to 1,000 runs are inspected
  per workflow to find the latest eligible completed result; reaching that limit reports a
  refresh error. This observes workflow outcomes, not actual service health.
  Deployment results are cached for two minutes between polling cycles to limit
  Actions API traffic; browser opening and comment dismissal remain immediate.

The companion uses GitHub's [notification API](https://docs.github.com/en/rest/activity/notifications)
and [workflow-run API](https://docs.github.com/en/rest/actions/workflow-runs).
The CLI login needs access to notifications and Actions in the relevant repositories.

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
  job and other jobs still running. Animated rows are composed in a reusable RGB565
  buffer and sent as one bounded bitmap, with separate title, spinner, and badge
  clips. There is no visible blanking pass between frames; unchanged row images
  and unchanged refreshes do not repaint the screen. CI animation stops when data is stale.
  Highlight an item to see its number, abbreviated repository, and compact status
  together on one footer line. Long repository names shorten before status; the
  list shows ten rows at a time. Long selected titles scroll after a short
  reading pause, replacing only that row's bitmap. Turn past either end of the list
  to clear the highlight and stop title scrolling; turn back to select again.
  Refreshes preserve that unselected state, and clicking it opens nothing. A successful
  LGTM-label workflow does not itself count as the LGTM label.
- **R / Review:** `is:open -is:draft review-requested:@me`, sorted by **least
  recently updated**, with fewer than the configured number of approvals.
  PRs with merge conflicts are hidden until the conflict is resolved. Your own,
  eligible Dependabot, and additional-author PRs keep their conflict status visible
  in Mine.
  PRs with queued/running checks or failed checks (including cancelled, timed-out,
  or action-required runs) are hidden until those checks clear. PRs with no checks
  remain eligible, as do successful, skipped, or neutral checks. This filter applies
  only to Review; your own, Dependabot, and additional-author PRs keep their CI status
  visible in Mine/Merge.
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
Comment IDs add validated numeric GitHub comment anchors; deployment IDs use
`owner/repo/actions/runs/number`. All three kinds open immediately with stale data.
A maximum of 60 rows is sent to the device after locally acknowledged comments
are removed, with truncation shown in the footer.
Large payloads are shortened further to fit the device's 16 KiB input buffer,
with `USB limit` shown in the footer. Higher-priority rows are retained first.
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
