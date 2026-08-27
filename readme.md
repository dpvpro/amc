# Arch Mirrors Checker (AMC)

Reimplementation of [Reflector](https://wiki.archlinux.org/title/Reflector) on Golang. Fetch the [Arch Linux mirror status](https://archlinux.org/mirrors/status/json/), filter the most up-to-date mirrors, rate them by download speed, and write a pacman mirrorlist.

## Features

- Fetches the official Arch Linux mirror status JSON (with local caching)
- Filters by country, protocol, sync age, delay, completion percent, IPv4/IPv6, ISO support, and URL regexes
- Rates mirror download speed in parallel (`--threads`) over HTTP(S) or rsync
- Sorts by age, rate, country, score or delay and limits results
- Writes a ready-to-use pacman mirrorlist or plain mirror info
- Optional configuration file
- Ships systemd service and timer units for automatic updates

## Installation

Requires Go 1.26+. AMC use the `rsync` binary for rating rsync mirrors.

### Manual

```sh
git clone https://github.com/dpvpro/amc.git
cd amc
go build -o amc .
sudo install -m755 amc /usr/local/bin/amc
```

### AUR

```sh
yay -S amc
```

## Usage

```
amc [options]
```

Without options, AMC prints the full mirrorlist to stdout.

```sh
# The five most recently synchronized HTTPS mirrors, sorted by speed
amc --latest 5 --sort rate --protocol https

# ...and save the result over the system mirrorlist
sudo amc --latest 5 --sort rate --protocol https --save /etc/pacman.d/mirrorlist

# Restrict to specific countries
amc --country Russia,Sweden,Finland --latest 12 --sort rate --protocol https

# List available countries
amc --list-countries

# Show detailed information about the mirrors
amc --info --latest 5
```

## Options

| Option | Default | Descriptioin |
| --- | --- | --- |
| `--age FLOAT` | `0` | Only mirrors synchronized within the last *n* hours |
| `--cache-timeout INT` | `300` | Seconds the mirror status data may be cached |
| `--completion-percent FLOAT` | `100` | Minimum completion percent `[0-100]` |
| `--config FILE` | - | Read options from a config file (see below) |
| `--connection-timeout INT` | `8` | Seconds to wait before a connection times out |
| `--country LIST` | - | Restrict to countries (name or code, comma-separated or repeatable) |
| `--delay FLOAT` | `0` | Only mirrors with a reported sync delay of *n* hours or less |
| `--download-timeout INT` | `8` | Seconds to wait before a download times out |
| `--exclude REGEX` | - | Exclude servers matching this regex (repeatable) |
| `--fastest INT` | `0` | Return the *n* fastest mirrors |
| `--include REGEX` | - | Include only servers matching this regex (repeatable) |
| `--info` | `false` | Print mirror information instead of a mirror list |
| `--ipv4` | `false` | Only mirrors that support IPv4 |
| `--ipv6` | `false` | Only mirrors that support IPv6 |
| `--isos` | `false` | Only mirrors that host ISOs |
| `--latest INT` | `0` | Limit to the *n* most recently synchronized mirrors |
| `--list-countries` | `false` | List countries with a mirror count and exit |
| `--number INT` | `0` | Return at most *n* mirrors |
| `--protocol LIST` | - | Match these protocols, comma-separated or repeatable |
| `--save FILE` | - | Write the mirrorlist to this path instead of stdout |
| `--score INT` | `0` | Limit to the *n* mirrors with the highest score |
| `--sort CRITERION` | - | Sort by `age`, `rate`, `country`, `score` or `delay` |
| `--threads INT` | `8` | Number of parallel rating downloads (`1` = sequential) |
| `--url URL` | - | URL of the mirror status JSON (see below) |
| `--verbose` | `false` | Print extra information to stderr |

> For `--url` option default value is `https://archlinux.org/mirrors/status/json/`.

## How options combine

Options are applied in two stages: filters first, then sorting with limits.

### Stage 1 — filters

`--age`, `--delay`, `--country`, `--protocol`, `--completion-percent`,
`--isos`, `--ipv4`, `--ipv6`, `--include` and `--exclude` run against the full
mirror list and drop mirrors that do not match. A mirror must satisfy all
active filters to survive. Filters never change the order of mirrors.

### Stage 2 — sorting and limits

The remaining mirrors are processed in a fixed order:

```
latest -> score -> fastest -> sort -> number
```

Each of `--latest`, `--score` and `--fastest` means "sort by a criterion, then
keep the first N mirrors". Every option works on the output of the previous
one, so the pool is narrowed step by step.

| Option | What it does |
| --- | --- |
| `--latest N` | Sort by sync age (newest first), keep the N most recent |
| `--score N` | Sort by score (highest first), keep the top N |
| `--fastest N` | Measure download speed, sort by rate, keep the N fastest |
| `--sort X` | Reorder the result by `age`, `rate`, `country`, `score` or `delay` |
| `--number N` | Return at most N mirrors (does not sort) |

### Examples

- `--age 24 --latest 5` — of the mirrors synced within the last 24 hours,
  keep the 5 most recently synchronized.
- `--delay 5 --fastest 3` — of the mirrors with a sync delay of at most 5
  hours, measure speed and keep the 3 fastest.
- `--latest 5 --fastest 3` — the 3 fastest are chosen only from the 5 newest
  mirrors, because `--latest` narrows the pool first.
- `--latest 10 --score 5` — take the 10 newest, then the 5 with the highest
  score (order matters: `--latest` runs before `--score`).
- `--sort age --number 3` — sort by sync age, then keep the first 3.

### Note about `--fastest` and `--sort rate` options

Options `--sort rate` and `--fastest` measure download speed by fetching the `extra/os/x86_64/extra.db` repository database from each mirror.

Both options works similarly, but they differ in what happens to the result:

| Command | Measures speed | Sorts by speed | Limits result |
| --- | --- | --- | --- |
| `--sort rate` | yes | yes | no |
| `--sort rate --number 5` | yes | yes | yes (first 5) |
| `--fastest 5` | yes | yes | yes (first 5) |

- `--sort rate` alone rates every mirror and sorts them by speed but returns
  the **whole list** — with hundreds of mirrors you usually want to pair it
  with `--number`, e.g. `--sort rate --number 5`.
- `--fastest N` is a shorthand for "measure speed, sort by speed, keep the
  N fastest" and is equivalent to `--sort rate --number N` in a single flag.
- `--fastest N --sort X` keeps the **set** of the N fastest mirrors but
  **orders** them by X: `--fastest 5 --sort age` lists the 5 fastest mirrors
  by sync age instead of by speed.


### Note about `--country` option

Option `--sort country` honours the order of `--country`: mirrors from the listed countries come first in that exact order, followed by all other countries alphabetically. For example, `--country se,no,dk,fi --sort country` produces Sweden, Norway, Denmark, Finland, then the rest of the world.

Matching is case-insensitive and accepts both country names and two-letter codes. A `*` in the list places the "everything else" group at that position instead of at the end: `--country se,*,dk --sort country` yields Sweden first, then all unlisted countries, then Denmark.

## Configuration file

AMC reads a config file passed with `--config`. Default location config path is `/etc/xdg/amc/amc.conf`. The file contains valid AMC command-line options, one per line. Empty lines and lines beginning with `#` are ignored, and values may be quoted. Options in the file are merged with the command line. Command line arguments override file values.

```text
--save /etc/pacman.d/mirrorlist
--protocol https
--country Russia,Sweden,Finland
--latest 12
--sort rate
```

## SystemD

AMC ships a oneshot service and a weekly timer for automatic mirrorlist
updates. Install the provided units and the configuration:

```sh
sudo install -m644 configs/amc.conf /etc/xdg/amc/amc.conf
sudo install -m644 configs/amc.service configs/amc.timer /usr/lib/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now amc.timer   # weekly refresh
```

To update the mirrorlist immediately:

```sh
sudo systemctl start amc.service
```

The service runs with `ProtectSystem=strict` and only reads
`/etc/xdg/amc/amc.conf` and writes `/etc/pacman.d/mirrorlist`.

## Caching

The mirror status JSON is cached so repeated runs stay fast. All cache files live in `$XDG_CACHE_HOME/amc/` (or `~/.cache/amc/`). The default URL is cached as `amc/mirrorstatus.json`, and custom `--url` values under the same directory with an encoded file name. A cache is reused within `--cache-timeout` seconds.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Runtime error (fetch, config, save, no mirrors) |
| `2` | Usage error (invalid arguments) |

## License

[MIT](license)
