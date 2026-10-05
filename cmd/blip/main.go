package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BrianLeishman/blip/dashboard"
	ghdata "github.com/BrianLeishman/blip/internal/github"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

func main() {
	configPath := flag.String("config", "blip.local.json", "GitHub configuration file")
	port := flag.String("port", "", "serial port; auto-detect Feather when omitted")
	probe := flag.Bool("probe", false, "show firmware events without sending dashboard data")
	once := flag.Bool("once", false, "print GitHub snapshot and exit without connecting")
	interval := flag.Duration("interval", 30*time.Second, "GitHub polling interval (minimum 15s)")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var config ghdata.Config
	if !*probe {
		b, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.Unmarshal(b, &config); err != nil {
			log.Fatal(err)
		}
		if err = ghdata.Validate(config); err != nil {
			log.Fatal(err)
		}
	}
	if *once {
		r, err := fetch(ctx, ghdata.NewClient(config))
		if err != nil {
			log.Fatal(err)
		}
		state, err := loadAlertState(ctx, *configPath)
		if err != nil {
			log.Fatal(err)
		}
		b, _ := json.MarshalIndent(state.filter(r.Snapshot), "", "  ")
		fmt.Println(string(b))
		return
	}
	if *interval < 15*time.Second {
		log.Fatal("interval must be at least 15s")
	}
	for ctx.Err() == nil {
		name := *port
		if name == "" {
			var err error
			name, err = detect(ctx)
			if err != nil {
				log.Print(err)
				pause(ctx, 2*time.Second)
				continue
			}
		}
		state, err := loadAlertState(ctx, *configPath)
		if err != nil {
			log.Fatal(err)
		}
		if err := run(ctx, name, config, state, *probe, *interval); err != nil && ctx.Err() == nil {
			log.Print(err)
		}
		pause(ctx, 2*time.Second)
	}
}
func pause(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
func detect(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}
	var matches []string
	for _, p := range ports {
		if p.IsUSB && strings.EqualFold(p.VID, "239A") && strings.HasPrefix(filepath.Base(p.Name), "cu.") {
			matches = append(matches, p.Name)
		}
	}
	if runtime.GOOS != "darwin" {
		for _, p := range ports {
			if p.IsUSB && strings.EqualFold(p.VID, "239A") {
				matches = append(matches, p.Name)
			}
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("found %d Adafruit serial devices; connect blip or use -port", len(matches))
	}
	return matches[0], nil
}
func fetch(ctx context.Context, client *ghdata.Client) (ghdata.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return client.Fetch(ctx)
}

type update struct {
	result ghdata.Result
	err    error
}

func run(ctx context.Context, name string, c ghdata.Config, state *alertState, probe bool, interval time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p, err := serial.Open(name, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return err
	}
	defer p.Close()
	// Setting DTR makes USB CDC output available on the device.
	if err = p.SetDTR(true); err != nil {
		return err
	}
	log.Printf("Connected to %s", name)
	events := make(chan dashboard.Event, 16)
	readErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(p)
		for scanner.Scan() {
			var e dashboard.Event
			if json.Unmarshal(scanner.Bytes(), &e) == nil {
				select {
				case events <- e:
				case <-ctx.Done():
					return
				}
			}
		}
		err := scanner.Err()
		if err == nil {
			err = errors.New("device disconnected")
		}
		readErr <- err
	}()
	updates := make(chan update, 1)
	if !probe {
		go func() {
			client := ghdata.NewClient(c)
			for {
				r, e := fetch(ctx, client)
				select {
				case updates <- update{r, e}:
				case <-ctx.Done():
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval):
				}
			}
		}()
	}
	current := ghdata.Result{URLs: map[string]string{}}
	var wire []byte
	var lastSend, lastFetch time.Time
	send := func() error {
		if len(wire) == 0 {
			return nil
		}
		for off := 0; off < len(wire); {
			end := off + 64
			if end > len(wire) {
				end = len(wire)
			}
			n, e := p.Write(wire[off:end])
			if e != nil {
				return e
			}
			if n == 0 {
				return errors.New("zero byte serial write")
			}
			off += n
			time.Sleep(2 * time.Millisecond)
		}
		lastSend = time.Now()
		return nil
	}
	publish := func() error {
		current.Snapshot = state.filter(current.Snapshot)
		var err error
		current.Snapshot, wire, err = snapshotWire(current.Snapshot)
		if err != nil {
			return err
		}
		return send()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readErr:
			return err
		case u := <-updates:
			if u.err != nil {
				log.Printf("GitHub refresh failed: %v", u.err)
				continue
			}
			current = u.result
			lastFetch = time.Now()
			if e := publish(); e != nil {
				return e
			}
			log.Printf("Dashboard refreshed: %d rows", len(current.Snapshot.Rows))
		case e := <-events:
			switch e.Kind {
			case "hello":
				if probe {
					log.Printf("Firmware heartbeat; encoder error=%q", e.Error)
				}
				if e.Error != "" {
					log.Printf("Device: %s", e.Error)
				}
				if len(wire) > 0 && e.Revision != current.Snapshot.Revision && time.Since(lastFetch) < 60*time.Second && time.Since(lastSend) > 3*time.Second {
					if err = send(); err != nil {
						return err
					}
				}
			case "touch-error":
				log.Printf("Touchscreen: %s", e.Error)
			case "touch-scroll":
				log.Printf("Touch scroll: row %d", e.Position)
			case "ack":
				log.Printf("Device acknowledged snapshot %s", e.Revision)
			case "turn", "click":
				if probe {
					log.Printf("%s position=%d", e.Kind, e.Position)
				}
			case "open":
				link, ok := openTarget(e)
				if !ok {
					log.Printf("Ignored open %s: malformed item ID", e.ID)
					continue
				}
				command := "open"
				if runtime.GOOS != "darwin" {
					command = "xdg-open"
				}
				if err = exec.CommandContext(ctx, command, link).Run(); err != nil {
					log.Printf("Open browser: %v", err)
				} else {
					log.Printf("Opened %s", e.ID)
					if strings.Contains(e.ID, "#") {
						if err := state.acknowledge(ctx, e.ID); err != nil {
							log.Printf("Save comment acknowledgment: %v", err)
							continue
						}
						current.Snapshot.Revision = fmt.Sprintf("%d", time.Now().UnixNano())
						if err := publish(); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}

// The device carries an item ID, not an arbitrary URL. Build the fixed-host
// GitHub URL locally without consulting refresh state or snapshot revisions.
func openTarget(e dashboard.Event) (string, bool) {
	parts := strings.Split(e.ID, "/")
	if len(parts) < 4 || !ghdata.ValidRepository(parts[0]+"/"+parts[1]) {
		return "", false
	}
	positive := func(s string) bool {
		if s == "" || s[0] < '1' || s[0] > '9' {
			return false
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	if parts[2] == "actions" {
		if len(parts) != 5 || parts[3] != "runs" || !positive(parts[4]) {
			return "", false
		}
	} else {
		if len(parts) != 4 || (parts[2] != "pull" && parts[2] != "issues") {
			return "", false
		}
		number, anchor, hasAnchor := strings.Cut(parts[3], "#")
		if !positive(number) {
			return "", false
		}
		if hasAnchor {
			valid := strings.HasPrefix(anchor, "issuecomment-") && positive(strings.TrimPrefix(anchor, "issuecomment-"))
			if parts[2] == "pull" {
				valid = valid || strings.HasPrefix(anchor, "discussion_r") && positive(strings.TrimPrefix(anchor, "discussion_r")) || strings.HasPrefix(anchor, "pullrequestreview-") && positive(strings.TrimPrefix(anchor, "pullrequestreview-"))
			}
			if !valid {
				return "", false
			}
		}
	}
	return "https://github.com/" + e.ID, true
}
