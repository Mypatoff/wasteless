# WasteLess

## Prerequisites

- Go 1.22 or later

## Run

```sh
go run ./cmd/server
```

This prints the URL, the database path, and the backup path, then opens
your browser (unless `NO_BROWSER=1`).

## Build

```sh
./build.sh      # Linux/macOS
build.bat       # Windows
```

Both produce two binaries in `dist/`, with `CGO_ENABLED=0` and
`-ldflags "-s -w"` (no cgo dependency, no debug symbols, smaller size):

- `dist/WasteLess.exe` — windows/amd64
- `dist/wasteless-linux` — linux/amd64

Everything the app needs (HTML templates, CSS, JS) is built in with
`go:embed`, so each file above is the entire app — no other files to
copy alongside it.

## Running the built executable

- Put the `.exe` in its own folder (e.g. `Documents\WasteLess\`), not
  inside `Program Files` — Windows restricts write access there, and
  this app needs to create its database and backups next to itself.
- On first run, `wasteless.db` and a `backups\` folder appear beside the
  executable, in that same folder.
- To get a desktop shortcut, right-click the `.exe` → **Show more
  options** → **Create shortcut**, then drag the shortcut to your
  Desktop.
- The app runs as long as its console window stays open. Closing that
  window stops it.
- To move your data to a new folder or computer: stop the app, then
  copy the `.db` file next to the executable in the new location.
- Set `BACKUP_DIR` to a USB drive or a cloud-synced folder (Dropbox,
  OneDrive, etc.) if you want backups that survive a dead disk — a
  backup kept on the same drive as the database doesn't protect you if
  that drive dies.
- Windows SmartScreen may warn that this is an unrecognized app, since
  the binary isn't code-signed. Click **More info**, then **Run
  anyway**.

## Environment variables

| Var                | Default                                                        |
|--------------------|-----------------------------------------------------------------|
| `PORT`             | `8080`                                                           |
| `DB_PATH`          | `wasteless.db` next to the executable (or `./wasteless.db` under `go run`)     |
| `BACKUP_DIR`       | `backups` next to the database                                   |
| `BACKUP_KEEP_DAYS` | `30`                                                               |
| `NO_BROWSER`       | unset (set to `1` to skip auto-opening a browser)                 |

## Where the data lives

- Database: the resolved `DB_PATH`, printed on every startup.
- Backups: the resolved `BACKUP_DIR`, printed on every startup.
  - `wasteless-YYYY-MM-DD.db` — rewritten every 6 hours and on shutdown.
  - `wasteless-YYYY-MM-DD-start.db` — taken once per day on startup, never overwritten.

## Restore from a backup

1. Stop the app.
2. Copy the backup file over the live database file (overwrite `DB_PATH`).
3. Delete the `-wal` and `-shm` files next to the database, if present
   (e.g. `wasteless.db-wal`, `wasteless.db-shm`).
4. Start the app again.

## Never commit database files

Never commit `.db` files or the `backups/` folder — they contain your
real financial data. Both are already covered by `.gitignore`.

## TODO (my exercise)

- `store.GetEntry(id int64) (Entry, error)` and the matching
  `GET /api/entries/{id}` handler are intentionally not implemented —
  see the `TODO(me)` comment in `internal/store/store.go`. The frontend
  doesn't need them; the edit modal fills itself from the already-loaded
  list.
