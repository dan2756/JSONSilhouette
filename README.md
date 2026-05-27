# jsonshape

Prints the structure of JSON — types instead of values.

Point it at an API response you have never seen before and get the shape of
it on one screen, instead of scrolling through pages of values.

## Install

```sh
go install github.com/dan2756/jsonshape@latest
```

## Usage

```sh
jsonshape [flags] [file ...]
```

Reads the named files, or stdin when none (or `-`) is given. Several files
are merged as samples of one shape.

```sh
curl -s https://example.com/api/users | jsonshape
```

```
[
  {
    id: int
    name: string
    email: string | null
    tags: [string]
    owner: {
      login: string
      site_admin: bool
    }
    deleted_at?: string
  }
]
```

- The elements of an array are merged into one shape.
- A key that is missing from some of the objects is marked optional with `?`.
- A position that holds different types is shown as a union: `string | null`.
- Ints and floats in the same position are reported as `number`.
- Keys keep the order in which they first appeared.

## Flags

| Flag         | Meaning                                         |
|--------------|-------------------------------------------------|
| `--lines`    | read NDJSON: one value per line, all merged     |
| `--no-color` | never color the output                          |
| `-h, --help` | show usage                                      |
| `--version`  | print the version                               |

### NDJSON

With `--lines` every non-blank line is parsed as its own JSON value, and all
of them are merged, so a log file gives the shape of its records:

```sh
$ jsonshape --lines app.log
{
  ts: string
  level: string
  msg: string
  port?: int
  method?: string
  path?: string
  status?: int
  duration_ms?: number
  error?: {
    code: string
    retryable: bool
  }
}
```

Lines are read one at a time, so large logs are fine.

### Colors

Types, keys and optional markers are colored when stdout is a terminal.
Setting `NO_COLOR` (to anything non-empty) or `TERM=dumb` turns colors off,
as does `--no-color`.

## Errors

Invalid JSON is reported with its line and column:

```
$ jsonshape config.json
jsonshape: config.json:3:15: invalid character '}' looking for beginning of object key string
  3 |   "name": "x",}
    |               ^
```

## Exit status

| Code | Meaning                                      |
|------|----------------------------------------------|
| 0    | success                                      |
| 1    | the input could not be read or is not JSON   |
| 2    | usage error (unknown flag, too many files)   |

## License

MIT
