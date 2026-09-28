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

| Flag            | Meaning                                                 |
|-----------------|---------------------------------------------------------|
| `--depth N`     | show N levels of nesting, collapse deeper ones to `{…}` |
| `--paths`       | flattened output, one path per line                     |
| `--counts`      | note how many samples had each key and each type        |
| `--json-schema` | print a JSON Schema (draft 2020-12) instead             |
| `--lines`       | read NDJSON: one value per line, all merged             |
| `--no-color`    | never color the output                                  |
| `-h, --help`    | show usage                                              |
| `--version`     | print the version                                       |

Flags may come before or after the file names; `--` ends the flags.

### Paths

`--paths` prints every position on its own line with a jq-style path, which
is handy for grep and for writing jq filters:

```sh
$ jsonshape --paths users.json
.                     array
.[]                   object
.[].id                int
.[].name              string
.[].email             string | null
.[].tags              array
.[].tags[]            string
.[].owner             object
.[].owner.login       string
.[].owner.site_admin  bool
.[].deleted_at?       string
```

Optional keys keep their `?`, which jq reads as "ignore errors here", so
the paths can be pasted into a filter as they are. Keys that are not plain
identifiers are written as `.["content-type"]`.

### Depth

`--depth N` keeps the top N levels of objects and arrays and collapses the
rest, for documents too big to take in at once:

```sh
$ jsonshape --depth 2 users.json
[
  {
    id: int
    name: string
    email: string | null
    tags: […]
    owner: {…}
    deleted_at?: string
  }
]
```

Empty objects and arrays still show as `{}` and `[]`. `--depth` applies to
`--paths` too.

### Counts

`--counts` adds a comment to each line: how many of the objects had the
key, and how many values of each type were seen when there was more than
one. Arrays show how many items they held between them.

```sh
$ jsonshape --counts users.json
[
  {                       # 3 items
    id: int               # 3/3
    name: string          # 3/3
    email: string | null  # 3/3 · string 2, null 1
    tags: [string]        # 3/3 · 3 items
    owner: {              # 3/3
      login: string       # 3/3
      site_admin: bool    # 3/3
    }
    score: number         # 3/3 · int 1, float 2
    deleted_at?: string   # 1/3
  }
]
```

It works with `--paths` as well, and is most useful with `--lines`, where
the top line says how many records were read.

### JSON Schema

`--json-schema` turns the merged shape into a [JSON Schema](https://json-schema.org/)
(draft 2020-12) that every sample satisfies: a starting point for
validation or for code generators.

```sh
$ jsonshape --json-schema users.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "id": {
        "type": "integer"
      },
      "email": {
        "type": ["string", "null"]
      },
      ...
    },
    "required": ["id", "name", "email", "tags", "owner", "score"]
  }
}
```

Keys present in every object are `required`; properties keep the order
of the input. Unions become a list of types in one schema, ints mixed with
floats become `"number"`, and arrays that were always empty get no
`items`. `--depth` limits how far the schema goes; `--paths` and
`--counts` do not apply.

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
