#!/usr/bin/env python3
"""Install/uninstall the current user's blip login companion. No sudo needed."""
import os
import pathlib
import plistlib
import shutil
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parent.parent
label = 'com.brianleishman.blip'
agent = pathlib.Path.home() / 'Library/LaunchAgents' / (label + '.plist')
domain = 'gui/' + str(os.getuid())
if sys.platform != 'darwin':
    raise SystemExit('This installer is for macOS.')
if '--uninstall' in sys.argv:
    subprocess.run(['launchctl', 'bootout', domain + '/' + label], check=False)
    agent.unlink(missing_ok=True)
    raise SystemExit('blip login companion removed; your files are unchanged.')
if not (root / 'blip.local.json').exists():
    raise SystemExit('Create blip.local.json first (see blip.example.json).')
if not shutil.which('gh'):
    raise SystemExit('Install gh and run gh auth login first.')
subprocess.run(['go', 'build', '-o', str(root / 'build/blip'), './cmd/blip'], cwd=root, check=True)
logs = pathlib.Path.home() / 'Library/Logs/blip'
logs.mkdir(parents=True, exist_ok=True)
agent.parent.mkdir(parents=True, exist_ok=True)
subprocess.run(['launchctl', 'bootout', domain + '/' + label], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
config = {
    'Label': label,
    'ProgramArguments': [str(root / 'build/blip'), '-config', str(root / 'blip.local.json')],
    'WorkingDirectory': str(root),
    'EnvironmentVariables': {'PATH': os.environ['PATH']},
    'RunAtLoad': True,
    'KeepAlive': True,
    'ThrottleInterval': 10,
    'StandardOutPath': str(logs / 'companion.log'),
    'StandardErrorPath': str(logs / 'companion.log'),
}
with agent.open('wb') as f:
    plistlib.dump(config, f)
subprocess.run(['launchctl', 'bootstrap', domain, str(agent)], check=True)
print('Installed:', agent)
print('Log:', logs / 'companion.log')
