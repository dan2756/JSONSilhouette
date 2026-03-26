# jsonshape

Prints the structure of JSON from stdin — types instead of values.

Point it at an API response you have never seen before and get the shape of
it on one screen, instead of scrolling through pages of values.

## Install

```sh
go install github.com/dan2756/jsonshape@latest
```

## Usage

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

## License

MIT
