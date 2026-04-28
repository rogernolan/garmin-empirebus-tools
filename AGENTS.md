# Repository Instructions

## Garmin EmpirBus Signal Reference

Keep [docs/garmin-empirbus-signals.md](/Users/rog/Development/empirebus-tests/docs/garmin-empirbus-signals.md) up to date whenever any of these change:

- a new HAR or NDJSON capture reveals a command or state mapping
- a script in this repo starts relying on a new signal or command shape
- a domain grouping changes
- the Code Red module provenance or GitHub source becomes known

When updating that document:

- prefer browser-confirmed facts over inference
- label inferred mappings explicitly
- include the local file or external source that supports each new claim
- add dates or capture filenames when they matter
- keep a section for each domain and list known commands plus their arguments

If the exact Code Red module GitHub URL becomes known, add it to the signal reference document immediately and separate Code Red-derived knowledge from repo-local capture evidence.
