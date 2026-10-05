# WAL entry format

One WAL entry = one file = one batch of operations.
All multi-byte integers are **little-endian**. Floats are IEEE-754 `float32`,
stored via `math.Float32bits`.

## Entry layout

| Field          | Bytes    | Notes                                    |
| -------------- | -------- | ---------------------------------------- |
| Magic          | 4        | fixed constant, identifies a puffer WAL  |
| Version        | 2        | format version, checked before parsing   |
| Sequence       | 8        | uint64, matches the filename             |
| Num operations | 4        | uint32, how many operations follow       |
| Operation 1..N | variable | see below                                |
| Checksum       | 4        | CRC32 over every byte before it          |

Fixed header = 4 + 2 + 8 + 4 = **18 bytes**. Checksum = **4 bytes**.

## Operation layout

### UPSERT (type 0)

| Field         | Bytes    | Notes                        |
| ------------- | -------- | ---------------------------- |
| Type          | 1        | 0 = upsert                   |
| ID length     | 2        | uint16, max 65535            |
| ID            | variable | UTF-8 bytes                  |
| Vector length | 4        | uint32                       |
| Floats        | 4 each   | little-endian float32 bits   |

### DELETE (type 1)

| Field     | Bytes    | Notes       |
| --------- | -------- | ----------- |
| Type      | 1        | 1 = delete  |
| ID length | 2        | uint16      |
| ID        | variable | UTF-8 bytes |

A delete carries no vector.

## Read order

Magic and version are validated **first**, before any length field is trusted.
A wrong magic or an unknown version is rejected without allocating anything.
The checksum is verified before the operations are applied.

## Worked example

Two upserts, 4-number vectors, ids `"a"` and `"bb"`:

```
header                                    18
op1: 1 + 2 + 1  + 4 + (4 x 4)           = 24
op2: 1 + 2 + 2  + 4 + (4 x 4)           = 25
checksum                                   4
                                        ----
                                          71 bytes
```

## Open questions

- Does `Vector length` count **floats** or **bytes**? Decoder and encoder must agree.
- Magic value not yet chosen.
