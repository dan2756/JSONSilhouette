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
jsonshape [flags] [file]
```

Reads the named file, or stdin when no file (or `-`) is given.

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
