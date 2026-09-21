# blip

**Brian's Little Information Panel.** A tiny USB desk dashboard: one color screen,
one clickable knob, TinyGo firmware, and a Go companion on your Mac.

Turn to highlight a PR or issue. Press to open it in your default browser.
Everything lives in one scrolling dashboard; there are no pages or menus.

## Hardware

| Qty | Part |
| --- | --- |
| 1 | [Adafruit Feather RP2040 #4884](https://www.adafruit.com/product/4884) |
| 1 | [2.4-inch TFT FeatherWing V2 #3315](https://www.adafruit.com/product/3315) |
| 1 | [STEMMA QT encoder breakout #4991](https://www.adafruit.com/product/4991) |
| 1 | [Rotary Encoder + Extras #377](https://www.adafruit.com/product/377) |
| 1 | [100 mm STEMMA QT cable #4210](https://www.adafruit.com/product/4210) |
| 1 | USB-C data cable |
| 1 | [M2.5 nylon hardware kit #3299](https://www.adafruit.com/product/3299) |
| 1 | [Four rubber feet #550](https://www.adafruit.com/product/550) |

Solder the Feather's 12- and 16-pin male headers, long ends away from the component
side. Plug both into the screen's rear sockets, matching pin labels. Solder the
encoder into its breakout; leave the breakout's six-pin header unpopulated.
Connect either encoder QT socket to the screen's QT socket. Connect USB to the
Feather. No battery, SD card, or breadboard is required. Touch is unused.

## Build and first flash

Requires Go 1.26+, TinyGo 0.42+, and GitHub CLI (`gh`). On macOS install TinyGo
from its official `tinygo-org/tools` Homebrew tap. Authenticate with `gh auth login`.

```sh
make all
```

Hold BOOTSEL on the Feather, tap RESET, then release BOOTSEL. A drive named
`RPI-RP2` appears. Copy `build/blip.uf2` to it. It will reboot automatically.
Before the companion connects, the screen shows a hardware test: turn the knob to
change the position counter, press to increment the click counter.

```sh
make probe
```

This logs rotation/click events without replacing the hardware test screen.
The firmware can be reflashed with `make flash`; use BOOTSEL if automatic reset
is unavailable. Flashing replaces the factory firmware.

## GitHub dashboard

```sh
cp blip.example.json blip.local.json
# Edit your repositories, urgent issue query, and approval threshold.
make run
```

`blip.local.json` is ignored by Git. GitHub credentials stay in your existing `gh`
login, on the computer. The device receives titles and opaque item IDs, never tokens.

- **M / Merge:** your non-draft PRs with enough effective approvals, GitHub's
  approved review decision, or **no outstanding review requests**. This is your
  personal merge queue, not a claim that branch protections or checks pass.
  A changes-requested decision keeps the PR in Mine.
- **O / Mine:** your other non-draft PRs, awaiting reviewers or changes.
  Both own-PR groups show `n/n` approvals, `L+`/`L-` for the actual LGTM label,
  and CI status: `+` successful, `!` failed, `~` pending, `?` no checks.
  Highlight a PR to see its number and fuller status in the footer. A successful
  LGTM-label workflow does not itself count as the LGTM label.
- **R / Review:** `is:open -is:draft review-requested:@me`, sorted by **least
  recently updated**, with fewer than the configured number of approvals.
  All review pages are fetched; duplicate approvals from one person count once.
  Bots and the author are excluded. Comments do not revoke an approval; a later
  changes-requested or dismissed review does. GitHub remains authoritative about
  reviewer eligibility and branch protection.
- **! / Urgent:** open issues whose native **Priority** field is **Urgent**.
  `urgent_query` uses advanced issue search, for example
  `org:YOUR_ORG is:issue is:open field.priority:Urgent sort:updated-asc`.
  This works across repositories independently of project membership or labels.
  Returned native Priority values are verified. Leave the query empty to disable
  urgent issues; the urgent count hides when zero.

Clicking an item only opens its GitHub page. It never merges, approves, or edits.

Data refreshes every 30 seconds. Turning the knob scrolls the single list when
needed. Refreshes preserve the highlighted item by ID. The display marks stale
content offline; the companion refuses old-revision or stale click events.
A maximum of 60 rows is sent to the device, with truncation shown in the footer.
GitHub CLI queries are capped at 1,000 candidates per category and repository.
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

## Enclosure

Editable OpenSCAD source and exported STLs live in `enclosure/`.
See its README for fit assumptions before printing. OpenSCAD 2021.01+:

```sh
make enclosure
```

## Scope

The first milestone is GitHub. IAP metrics are planned but not implemented yet;
there are no fake sales figures or private company dependencies in this repo.

## License

MIT. Dependencies retain their respective licenses. Adafruit's hardware designs
are separately licensed and linked as dimensional references, not vendored here.
